package live

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"hvc/internal/config"
	"hvc/pkg/logx"
)

// AuthConfig 表示直播鉴权配置。
type AuthConfig struct {
	PushKey        string
	PlayKey        string
	PushExpireSec  int
	PlayExpireSec  int
	EnablePushAuth bool
	EnablePlayAuth bool
}

// AuthResult 表示鉴权结果。
type AuthResult struct {
	Allowed   bool
	Reason    string
	ChannelKey string
	ExpireAt  int64
}

// NewAuthConfig 从动态配置创建鉴权配置。
func NewAuthConfig(cfg config.DynamicRuntimeConfig) AuthConfig {
	return AuthConfig{
		PushKey:        "hvc-push-secret-key",
		PlayKey:        "hvc-play-secret-key",
		PushExpireSec:  3600,
		PlayExpireSec:  1800,
		EnablePushAuth: true,
		EnablePlayAuth: true,
	}
}

// VerifyPushAuth 验证推流鉴权。
//
// 推流鉴权采用 URL 鉴权方式，支持两种模式：
//  1. HMAC-SHA256 签名模式：推流 URL 携带 sign 和 expire 参数，
//     sign = HMAC-SHA256(pushKey, channelKey+expire)
//  2. 简单 Token 模式：推流 URL 携带 token 参数，
//     token = SHA256(pushKey+channelKey+expire)
//
// 推流 URL 格式示例：
//
//	rtmp://push.example.com/live/{channel_key}?expire=1700000000&sign=xxxx
func VerifyPushAuth(cfg AuthConfig, channelKey string, query url.Values) AuthResult {
	if !cfg.EnablePushAuth {
		return AuthResult{
			Allowed:    true,
			Reason:     "push_auth_disabled",
			ChannelKey: channelKey,
		}
	}

	if channelKey == "" {
		return AuthResult{
			Allowed: false,
			Reason:  "channel_key_empty",
		}
	}

	expireStr := query.Get("expire")
	if expireStr == "" {
		expireStr = query.Get("txTime")
	}
	sign := query.Get("sign")
	if sign == "" {
		sign = query.Get("txSecret")
	}
	token := query.Get("token")

	if sign != "" && expireStr != "" {
		return verifyHMACSignature(cfg.PushKey, channelKey, expireStr, sign, cfg.PushExpireSec)
	}

	if token != "" && expireStr != "" {
		return verifySimpleToken(cfg.PushKey, channelKey, expireStr, token, cfg.PushExpireSec)
	}

	return AuthResult{
		Allowed: false,
		Reason:  "missing_auth_params",
	}
}

// VerifyPlayAuth 验证播放鉴权。
//
// 播放鉴权同样采用 URL 鉴权方式，与推流鉴权使用不同的密钥。
// 播放 URL 格式示例：
//
//	http://play.example.com/live/{channel_key}.m3u8?expire=1700000000&sign=xxxx
func VerifyPlayAuth(cfg AuthConfig, channelKey string, query url.Values) AuthResult {
	if !cfg.EnablePlayAuth {
		return AuthResult{
			Allowed:    true,
			Reason:     "play_auth_disabled",
			ChannelKey: channelKey,
		}
	}

	if channelKey == "" {
		return AuthResult{
			Allowed: false,
			Reason:  "channel_key_empty",
		}
	}

	expireStr := query.Get("expire")
	sign := query.Get("sign")
	token := query.Get("token")

	if sign != "" && expireStr != "" {
		return verifyHMACSignature(cfg.PlayKey, channelKey, expireStr, sign, cfg.PlayExpireSec)
	}

	if token != "" && expireStr != "" {
		return verifySimpleToken(cfg.PlayKey, channelKey, expireStr, token, cfg.PlayExpireSec)
	}

	return AuthResult{
		Allowed: false,
		Reason:  "missing_auth_params",
	}
}

// verifyHMACSignature 验证 HMAC-SHA256 签名。
func verifyHMACSignature(secretKey, channelKey, expireStr, signature string, maxExpireSec int) AuthResult {
	expireAt, err := strconv.ParseInt(expireStr, 10, 64)
	if err != nil {
		return AuthResult{Allowed: false, Reason: "invalid_expire_format"}
	}

	now := time.Now().Unix()
	if expireAt < now {
		return AuthResult{Allowed: false, Reason: "auth_expired"}
	}

	if maxExpireSec > 0 && expireAt > now+int64(maxExpireSec) {
		return AuthResult{Allowed: false, Reason: "expire_too_far"}
	}

	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(channelKey + expireStr))
	expectedSign := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(strings.ToLower(expectedSign))) {
		return AuthResult{Allowed: false, Reason: "signature_mismatch"}
	}

	return AuthResult{
		Allowed:    true,
		Reason:     "ok",
		ChannelKey: channelKey,
		ExpireAt:   expireAt,
	}
}

// verifySimpleToken 验证简单 Token。
func verifySimpleToken(secretKey, channelKey, expireStr, token string, maxExpireSec int) AuthResult {
	expireAt, err := strconv.ParseInt(expireStr, 10, 64)
	if err != nil {
		return AuthResult{Allowed: false, Reason: "invalid_expire_format"}
	}

	now := time.Now().Unix()
	if expireAt < now {
		return AuthResult{Allowed: false, Reason: "auth_expired"}
	}

	if maxExpireSec > 0 && expireAt > now+int64(maxExpireSec) {
		return AuthResult{Allowed: false, Reason: "expire_too_far"}
	}

	h := sha256.New()
	h.Write([]byte(secretKey + channelKey + expireStr))
	expectedToken := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(strings.ToLower(token)), []byte(strings.ToLower(expectedToken))) {
		return AuthResult{Allowed: false, Reason: "token_mismatch"}
	}

	return AuthResult{
		Allowed:    true,
		Reason:     "ok",
		ChannelKey: channelKey,
		ExpireAt:   expireAt,
	}
}

// GeneratePushAuthURL 生成带鉴权参数的推流 URL。
func GeneratePushAuthURL(cfg AuthConfig, baseURL string, channelKey string) string {
	if !cfg.EnablePushAuth {
		return fmt.Sprintf("%s/%s", baseURL, channelKey)
	}
	expireAt := time.Now().Add(time.Duration(cfg.PushExpireSec) * time.Second).Unix()
	expireStr := strconv.FormatInt(expireAt, 10)

	mac := hmac.New(sha256.New, []byte(cfg.PushKey))
	mac.Write([]byte(channelKey + expireStr))
	sign := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s/%s?expire=%s&sign=%s", baseURL, channelKey, expireStr, sign)
}

// GeneratePlayAuthURL 生成带鉴权参数的播放 URL。
func GeneratePlayAuthURL(cfg AuthConfig, baseURL string, channelKey string, format string) string {
	if !cfg.EnablePlayAuth {
		return fmt.Sprintf("%s/%s.%s", baseURL, channelKey, format)
	}
	expireAt := time.Now().Add(time.Duration(cfg.PlayExpireSec) * time.Second).Unix()
	expireStr := strconv.FormatInt(expireAt, 10)

	mac := hmac.New(sha256.New, []byte(cfg.PlayKey))
	mac.Write([]byte(channelKey + expireStr))
	sign := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s/%s.%s?expire=%s&sign=%s", baseURL, channelKey, format, expireStr, sign)
}

// OnPushAuth 推流鉴权回调处理。
//
// 当推流请求到达时，先验证鉴权参数，再检查频道状态。
// 鉴权通过且频道处于可推流状态时，允许推流。
func OnPushAuth(cfg AuthConfig, channelKey string, query url.Values) AuthResult {
	result := VerifyPushAuth(cfg, channelKey, query)
	if !result.Allowed {
		logx.Info("live.auth.push_rejected", logx.Fields{
			"channel_key": channelKey,
			"reason":      result.Reason,
		})
		return result
	}
	logx.Info("live.auth.push_allowed", logx.Fields{
		"channel_key": channelKey,
		"expire_at":   result.ExpireAt,
	})
	return result
}

// OnPlayAuth 播放鉴权回调处理。
func OnPlayAuth(cfg AuthConfig, channelKey string, query url.Values) AuthResult {
	result := VerifyPlayAuth(cfg, channelKey, query)
	if !result.Allowed {
		logx.Info("live.auth.play_rejected", logx.Fields{
			"channel_key": channelKey,
			"reason":      result.Reason,
		})
		return result
	}
	logx.Info("live.auth.play_allowed", logx.Fields{
		"channel_key": channelKey,
		"expire_at":   result.ExpireAt,
	})
	return result
}
