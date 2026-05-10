package middleware

import (
	"net"
	"net/http"
	"strings"

	"hvc/internal/model"
	"hvc/pkg/logx"
)

const internalTokenHeader = "X-HVC-Internal-Token"

// RequireInternalAccess 保护集群内部 HTTP 控制面接口。
//
// 约束如下：
// 1. 配置了 shared_token 时，必须显式携带内部令牌；
// 2. 未配置 shared_token 时，只允许本机回环地址访问；
// 3. 这样既兼容单机本地联调，也避免未加认证的内部接口直接裸露到公网。
func RequireInternalAccess(sharedToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowInternalRequest(r, sharedToken) {
			next.ServeHTTP(w, r)
			return
		}
		logx.Error("http.internal.unauthorized", nil, logx.Fields{
			"path":      r.URL.Path,
			"remote_ip": r.RemoteAddr,
		})
		logx.WriteJSON(w, http.StatusUnauthorized, model.Response{Code: 401, Message: "unauthorized"})
	})
}

func allowInternalRequest(r *http.Request, sharedToken string) bool {
	if strings.TrimSpace(sharedToken) != "" {
		return secureCompare(strings.TrimSpace(extractInternalToken(r)), strings.TrimSpace(sharedToken))
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func extractInternalToken(r *http.Request) string {
	token := strings.TrimSpace(r.Header.Get(internalTokenHeader))
	if token != "" {
		return token
	}
	token = strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return strings.TrimSpace(token[7:])
	}
	return token
}

func secureCompare(left string, right string) bool {
	if left == "" || right == "" {
		return false
	}
	if len(left) != len(right) {
		return false
	}
	result := byte(0)
	for i := 0; i < len(left); i++ {
		result |= left[i] ^ right[i]
	}
	return result == 0
}
