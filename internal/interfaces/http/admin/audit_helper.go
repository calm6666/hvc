package admin

import (
	"context"
	"net/http"
	"sync"

	"hvc/internal/audit"
)

var (
	auditRepo *audit.Repository
	auditMu   sync.RWMutex
)

// ConfigureAdminAudit 注入后台审计仓储。
func ConfigureAdminAudit(repo *audit.Repository) {
	auditMu.Lock()
	defer auditMu.Unlock()
	auditRepo = repo
}

func writeAdminAudit(ctx context.Context, r *http.Request, action, targetType, targetID string, resultCode int, resultMessage string) {
	auditMu.RLock()
	repo := auditRepo
	auditMu.RUnlock()
	if repo == nil {
		return
	}
	_ = repo.Save(ctx, audit.Record{
		Action:        action,
		TargetType:    targetType,
		TargetID:      targetID,
		RequestIP:     r.RemoteAddr,
		UserAgent:     r.UserAgent(),
		ResultCode:    resultCode,
		ResultMessage: resultMessage,
	})
}
