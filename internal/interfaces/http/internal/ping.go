package internal

import (
	"net/http"

	"hvc/pkg/logx"
)

// Ping 处理内部接口存活检查。
func Ping(w http.ResponseWriter, r *http.Request) {
	logx.WriteJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok"})
}
