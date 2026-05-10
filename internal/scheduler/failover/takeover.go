package failover

import (
	"context"
	"sync"
	"time"

	"hvc/internal/cluster"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// ShouldTakeOver 判断是否应触发任务接管。
//
// 当租约已过期且超过心跳超时阈值时，认为原执行节点已失联，
// 需要将任务重新调度到其他可用节点。
func ShouldTakeOver(lease cluster.LeaseState, now time.Time) bool {
	return !lease.ExpireAt.IsZero() && !lease.ExpireAt.After(now)
}

// ShieldConfig 表示故障屏蔽配置。
type ShieldConfig struct {
	// MaxConsecutiveFailures 节点连续失败次数上限，超过后自动隔离。
	MaxConsecutiveFailures int
	// ShieldDuration 隔离持续时间，到期后自动恢复。
	ShieldDuration time.Duration
	// RecoveryProbeInterval 恢复探测间隔，隔离期间定期检查节点是否恢复。
	RecoveryProbeInterval time.Duration
}

// DefaultShieldConfig 返回默认故障屏蔽配置。
func DefaultShieldConfig() ShieldConfig {
	return ShieldConfig{
		MaxConsecutiveFailures: 3,
		ShieldDuration:         10 * time.Minute,
		RecoveryProbeInterval:  2 * time.Minute,
	}
}

// NodeShield 表示节点故障屏蔽状态。
type NodeShield struct {
	NodeID             uint64
	ConsecutiveFailures int
	ShieldedAt         time.Time
	ShieldDuration     time.Duration
	Reason             string
}

// ShieldTracker 跟踪节点故障屏蔽状态。
type ShieldTracker struct {
	mu      sync.RWMutex
	cfg     ShieldConfig
	shields map[uint64]*NodeShield
}

// NewShieldTracker 创建故障屏蔽跟踪器。
func NewShieldTracker(cfg ShieldConfig) *ShieldTracker {
	return &ShieldTracker{
		cfg:     cfg,
		shields: make(map[uint64]*NodeShield),
	}
}

// RecordFailure 记录节点失败事件。
//
// 当连续失败次数达到阈值时，自动将节点加入隔离列表。
func (t *ShieldTracker) RecordFailure(nodeID uint64, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	shield, ok := t.shields[nodeID]
	if !ok {
		shield = &NodeShield{
			NodeID:             nodeID,
			ConsecutiveFailures: 0,
		}
		t.shields[nodeID] = shield
	}
	shield.ConsecutiveFailures++
	shield.Reason = reason
	if shield.ConsecutiveFailures >= t.cfg.MaxConsecutiveFailures && shield.ShieldedAt.IsZero() {
		shield.ShieldedAt = time.Now()
		shield.ShieldDuration = t.cfg.ShieldDuration
		logx.Info("failover.shield.node_shielded", logx.Fields{
			"node_id":               nodeID,
			"consecutive_failures":  shield.ConsecutiveFailures,
			"shield_duration":       t.cfg.ShieldDuration.String(),
			"reason":                reason,
		})
	}
}

// RecordSuccess 记录节点成功事件，重置连续失败计数。
func (t *ShieldTracker) RecordSuccess(nodeID uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.shields, nodeID)
}

// IsShielded 判断节点是否处于隔离状态。
func (t *ShieldTracker) IsShielded(nodeID uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.isShieldedLocked(nodeID, time.Now())
}

func (t *ShieldTracker) isShieldedLocked(nodeID uint64, now time.Time) bool {
	shield, ok := t.shields[nodeID]
	if !ok {
		return false
	}
	if shield.ShieldedAt.IsZero() {
		return false
	}
	if now.Sub(shield.ShieldedAt) > shield.ShieldDuration {
		delete(t.shields, nodeID)
		logx.Info("failover.shield.node_recovered", logx.Fields{
			"node_id": nodeID,
		})
		return false
	}
	return true
}

// RecoverShieldedNodes 检查所有隔离节点，将已到期的节点恢复。
//
// 返回本次恢复的节点 ID 列表。
func (t *ShieldTracker) RecoverShieldedNodes() []uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	recovered := make([]uint64, 0)
	now := time.Now()
	for nodeID, shield := range t.shields {
		if shield.ShieldedAt.IsZero() {
			continue
		}
		if now.Sub(shield.ShieldedAt) > shield.ShieldDuration {
			delete(t.shields, nodeID)
			recovered = append(recovered, nodeID)
			logx.Info("failover.shield.node_auto_recovered", logx.Fields{
				"node_id":              nodeID,
				"shield_duration":      shield.ShieldDuration.String(),
				"consecutive_failures": shield.ConsecutiveFailures,
			})
		}
	}
	return recovered
}

// Snapshot 返回当前节点屏蔽状态快照。
//
// 该方法主要供后台洞察接口读取，
// 返回值已经做了深拷贝，不会把内部 map 暴露给调用方。
func (t *ShieldTracker) Snapshot() map[uint64]NodeShield {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	result := make(map[uint64]NodeShield, len(t.shields))
	for nodeID, shield := range t.shields {
		if shield == nil {
			continue
		}
		if t.isShieldedLocked(nodeID, now) {
			result[nodeID] = *shield
		}
	}
	return result
}

// TakeoverExpiredLeases 扫描所有已过期的租约任务，将其重置为排队状态以便重新调度。
//
// 同时对失联节点记录失败事件，可能触发故障屏蔽。
func TakeoverExpiredLeases(ctx context.Context, jobRepo *mysql.JobRepository, leaseCache *cluster.LeaseCache, shieldTracker *ShieldTracker, heartbeatTimeout time.Duration) int {
	if jobRepo == nil {
		return 0
	}
	jobs := jobRepo.ListAssigned(ctx, 0, "")
	now := time.Now()
	takeoverCount := 0
	for _, job := range jobs {
		if job.Status != model.JobStatusAssigned && job.Status != model.JobStatusRunning {
			continue
		}
		lease, ok := leaseCache.Get(ctx, job.JobID)
		if !ok {
			continue
		}
		if !ShouldTakeOver(lease, now.Add(-heartbeatTimeout)) {
			continue
		}
		if err := jobRepo.ResetToQueued(ctx, job.JobID); err != nil {
			logx.Error("failover.takeover.reset_failed", err, logx.Fields{
				"job_id":  job.JobID,
				"node_id": job.AssignedNodeID,
			})
			continue
		}
		takeoverCount++
		if shieldTracker != nil && job.AssignedNodeID != 0 {
			shieldTracker.RecordFailure(job.AssignedNodeID, "lease_expired")
		}
		logx.Info("failover.takeover.job_reset", logx.Fields{
			"job_id":           job.JobID,
			"previous_node_id": job.AssignedNodeID,
			"lease_generation": job.LeaseGeneration,
		})
	}
	return takeoverCount
}
