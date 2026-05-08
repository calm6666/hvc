package bootstrap

import (
	"context"
	"time"

	"hvc/internal/configcenter"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/pkg/logx"
)

const runtimeConfigSyncInterval = time.Second

// runtimeConfigSyncer 负责让当前节点自动追平最新已发布的 runtime config。
//
// 设计目标：
// 1. 先看 Redis 里的版本号，尽量不把数据库打成热点；
// 2. Redis 丢缓存时自动回源数据库并回填；
// 3. 其它节点无需重启，也能在发布后自动收敛到新版本。
type runtimeConfigSyncer struct {
	cache     *rediscache.RuntimeConfigCache
	db        *mysql.DB
	effective *configcenter.EffectiveConfig
}

func newRuntimeConfigSyncer(
	cache *rediscache.RuntimeConfigCache,
	db *mysql.DB,
	effective *configcenter.EffectiveConfig,
) *runtimeConfigSyncer {
	return &runtimeConfigSyncer{
		cache:     cache,
		db:        db,
		effective: effective,
	}
}

func (s *runtimeConfigSyncer) Start(ctx context.Context) error {
	ticker := time.NewTicker(runtimeConfigSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.syncOnce(ctx)
		}
	}
}

func (s *runtimeConfigSyncer) syncOnce(ctx context.Context) {
	if s == nil || s.db == nil || s.effective == nil {
		return
	}

	currentVersion := s.effective.CurrentVersion()
	if s.cache != nil {
		cacheVersion, exists, err := s.cache.GetPublishedVersion(ctx)
		if err == nil && exists {
			if cacheVersion == currentVersion {
				return
			}
			if snapshot, ok, loadErr := s.cache.LoadPublished(ctx); loadErr == nil && ok && snapshot.ConfigVersion == cacheVersion {
				s.effective.ReplaceWithVersion(snapshot.Config, snapshot.ConfigVersion)
				return
			}
		}
	}

	snapshot, ok := loadPublishedDynamicRuntimeConfig(ctx, s.cache, s.db)
	if !ok {
		return
	}
	if snapshot.ConfigVersion == currentVersion {
		return
	}

	logx.Info("runtime.config.synced", logx.Fields{
		"from_version": currentVersion,
		"to_version":   snapshot.ConfigVersion,
	})
	s.effective.ReplaceWithVersion(snapshot.Config, snapshot.ConfigVersion)
}
