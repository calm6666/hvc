package monitor

import "net/http"

// Snapshot 处理监控快照请求。
func Snapshot(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
