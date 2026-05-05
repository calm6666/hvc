package admin

import (
	"net/http"

	"hvc/pkg/logx"
)

// Ping 处理后台接口存活检查。
func Ping(w http.ResponseWriter, r *http.Request) {
	logx.WriteJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok"})
}
