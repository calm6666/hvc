package admin

import (
	"net/http"

	"hvc/internal/model"
	"hvc/pkg/logx"
)

// Ping 处理后台接口存活检查。
func Ping(w http.ResponseWriter, r *http.Request) {
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}
