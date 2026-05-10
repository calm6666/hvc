package internal

import (
	"net/http"

	"hvc/internal/model"
	"hvc/pkg/logx"
)

// Ping 处理内部接口存活检查。
func Ping(w http.ResponseWriter, r *http.Request) {
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}
