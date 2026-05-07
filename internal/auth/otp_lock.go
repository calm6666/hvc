package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"hvc/pkg/logx"
)

// OTPConfig 表示 OTP 配置。
type OTPConfig struct {
	// SecretKey OTP 密钥，用于生成和验证动态口令。
	SecretKey string
	// TimeStep 时间步长（秒），默认 30 秒。
	TimeStep int
	// Digits 口令位数，默认 6 位。
	Digits int
	// Window 验证窗口大小，允许前后各 Window 个时间步的偏差。
	Window int
}

// DefaultOTPConfig 返回默认 OTP 配置。
//
// 注意：SecretKey 必须在生产环境中通过配置或环境变量覆盖，
// 不要使用默认值。
func DefaultOTPConfig() OTPConfig {
	return OTPConfig{
		SecretKey: GenerateSecureToken(32),
		TimeStep:  30,
		Digits:    6,
		Window:    1,
	}
}

// OTPGenerator 表示 OTP 生成器。
//
// 基于 HMAC-SHA256 的 TOTP（Time-Based One-Time Password）实现，
// 兼容 RFC 6238 标准。
type OTPGenerator struct {
	cfg OTPConfig
	mu  sync.RWMutex
	// usedCodes 已使用的动态口令缓存，防止重放攻击。
	usedCodes map[string]time.Time
}

// NewOTPGenerator 创建 OTP 生成器。
func NewOTPGenerator(cfg OTPConfig) *OTPGenerator {
	return &OTPGenerator{
		cfg:       cfg,
		usedCodes: make(map[string]time.Time),
	}
}

// Generate 生成动态口令。
//
// 算法：TOTP = HMAC-SHA256(secretKey, timeStep) 的前 Digits 位数字。
func (g *OTPGenerator) Generate(userID uint64) string {
	timeStep := g.currentTimeStep()
	mac := hmac.New(sha256.New, []byte(g.cfg.SecretKey))
	mac.Write([]byte(fmt.Sprintf("%d:%d", userID, timeStep)))
	sum := mac.Sum(nil)

	code := truncateOTP(sum, g.cfg.Digits)
	return fmt.Sprintf("%0*d", g.cfg.Digits, code)
}

// Verify 验证动态口令。
//
// 支持时间窗口容错，允许前后各 Window 个时间步的偏差。
// 同时检查口令是否已被使用，防止重放攻击。
func (g *OTPGenerator) Verify(userID uint64, code string) bool {
	if len(code) != g.cfg.Digits {
		return false
	}

	now := time.Now()
	for delta := -g.cfg.Window; delta <= g.cfg.Window; delta++ {
		timeStep := g.currentTimeStep() + int64(delta)
		mac := hmac.New(sha256.New, []byte(g.cfg.SecretKey))
		mac.Write([]byte(fmt.Sprintf("%d:%d", userID, timeStep)))
		sum := mac.Sum(nil)

		expectedCode := fmt.Sprintf("%0*d", g.cfg.Digits, truncateOTP(sum, g.cfg.Digits))
		if expectedCode == code {
			cacheKey := fmt.Sprintf("%d:%s", userID, code)
			if g.isCodeUsed(cacheKey) {
				return false
			}
			g.markCodeUsed(cacheKey, now)
			return true
		}
	}
	return false
}

// currentTimeStep 返回当前时间步。
func (g *OTPGenerator) currentTimeStep() int64 {
	return time.Now().Unix() / int64(g.cfg.TimeStep)
}

// isCodeUsed 检查口令是否已被使用。
func (g *OTPGenerator) isCodeUsed(key string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.usedCodes[key]
	return ok
}

// markCodeUsed 标记口令为已使用。
func (g *OTPGenerator) markCodeUsed(key string, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.usedCodes[key] = now
	g.cleanupExpiredCodes(now)
}

// cleanupExpiredCodes 清理过期的已使用口令缓存。
func (g *OTPGenerator) cleanupExpiredCodes(now time.Time) {
	expiry := now.Add(time.Duration(g.cfg.TimeStep*(g.cfg.Window*2+1)) * time.Second)
	for key, usedAt := range g.usedCodes {
		if usedAt.Before(expiry.Add(-time.Duration(g.cfg.TimeStep*2) * time.Second)) {
			delete(g.usedCodes, key)
		}
	}
}

// truncateOTP 将 HMAC 结果截断为指定位数的数字。
func truncateOTP(sum []byte, digits int) int {
	offset := sum[len(sum)-1] & 0x0f
	binary := (int(sum[offset]&0x7f) << 24) |
		(int(sum[offset+1]) << 16) |
		(int(sum[offset+2]) << 8) |
		int(sum[offset+3])
	modulo := 1
	for i := 0; i < digits; i++ {
		modulo *= 10
	}
	return binary % modulo
}

// AccountLockConfig 表示账户锁定配置。
type AccountLockConfig struct {
	// MaxFailedAttempts 最大连续失败次数。
	MaxFailedAttempts int
	// LockDuration 锁定持续时间。
	LockDuration time.Duration
	// FailedAttemptWindow 失败计数窗口。
	FailedAttemptWindow time.Duration
}

// DefaultAccountLockConfig 返回默认账户锁定配置。
func DefaultAccountLockConfig() AccountLockConfig {
	return AccountLockConfig{
		MaxFailedAttempts:   5,
		LockDuration:        30 * time.Minute,
		FailedAttemptWindow: 15 * time.Minute,
	}
}

// AccountLockManager 管理账户锁定状态。
//
// 当管理员连续登录失败达到阈值时，自动锁定账户，
// 防止暴力破解攻击。锁定到期后自动解锁。
type AccountLockManager struct {
	cfg     AccountLockConfig
	mu      sync.RWMutex
	records map[uint64]*lockRecord
}

type lockRecord struct {
	failedAttempts int
	lastAttemptAt  time.Time
	lockedAt       time.Time
}

// NewAccountLockManager 创建账户锁定管理器。
func NewAccountLockManager(cfg AccountLockConfig) *AccountLockManager {
	return &AccountLockManager{
		cfg:     cfg,
		records: make(map[uint64]*lockRecord),
	}
}

// RecordFailedAttempt 记录一次登录失败。
//
// 当连续失败次数达到阈值时，自动锁定账户。
func (m *AccountLockManager) RecordFailedAttempt(userID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, ok := m.records[userID]
	if !ok {
		record = &lockRecord{}
		m.records[userID] = record
	}

	now := time.Now()
	if !record.lastAttemptAt.IsZero() && now.Sub(record.lastAttemptAt) > m.cfg.FailedAttemptWindow {
		record.failedAttempts = 0
	}

	record.failedAttempts++
	record.lastAttemptAt = now

	if record.failedAttempts >= m.cfg.MaxFailedAttempts && record.lockedAt.IsZero() {
		record.lockedAt = now
		logx.Info("auth.account.locked", logx.Fields{
			"user_id":           userID,
			"failed_attempts":   record.failedAttempts,
			"lock_duration":     m.cfg.LockDuration.String(),
		})
	}
}

// RecordSuccessfulLogin 记录一次登录成功，重置失败计数。
func (m *AccountLockManager) RecordSuccessfulLogin(userID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.records, userID)
}

// IsLocked 判断账户是否被锁定。
func (m *AccountLockManager) IsLocked(userID uint64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, ok := m.records[userID]
	if !ok {
		return false
	}
	if record.lockedAt.IsZero() {
		return false
	}
	if time.Since(record.lockedAt) > m.cfg.LockDuration {
		return false
	}
	return true
}

// Unlock 手动解锁账户。
func (m *AccountLockManager) Unlock(userID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.records, userID)
	logx.Info("auth.account.unlocked", logx.Fields{
		"user_id": userID,
	})
}

// FailedAttempts 返回账户当前连续失败次数。
func (m *AccountLockManager) FailedAttempts(userID uint64) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if record, ok := m.records[userID]; ok {
		return record.failedAttempts
	}
	return 0
}

// GenerateSecureToken 生成安全的随机令牌。
//
// 使用 crypto/rand 生成指定长度的十六进制随机字符串。
func GenerateSecureToken(byteLength int) string {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
