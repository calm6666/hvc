package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"hvc/internal/config"
	"hvc/internal/handler"
	adminhttp "hvc/internal/interfaces/http/admin"
	httpmiddleware "hvc/internal/interfaces/http/middleware"
	publichttp "hvc/internal/interfaces/http/public"
	"hvc/pkg/logx"
)

// HTTPServer 表示 HTTP 服务。
type HTTPServer struct {
	listenAddress        string
	systemHandler        *handler.SystemHandler
	transcodeHandler     *publichttp.TranscodeHandler
	clusterHandler       *publichttp.ClusterHandler
	liveHandler          *publichttp.LiveHandler
	manifestHandler      *publichttp.ManifestHandler
	authHandler          *adminhttp.AuthHandler
	configHandler        *adminhttp.ConfigHandler
	callbackHandler      *adminhttp.CallbackHandler
	rbacHandler          *adminhttp.RBACHandler
	writeRBACHandler     *adminhttp.WriteRBAC
	configCenterHandle   *adminhttp.ConfigCenterHandler
	adminClusterHandle   *adminhttp.ClusterHandler
	adminTranscodeHandle *adminhttp.TranscodeHandler
	adminLiveHandle      *adminhttp.LiveHandler
}

// NewHTTPServer 创建 HTTP 服务。
func NewHTTPServer(cfg config.ServerConfig, systemHandler *handler.SystemHandler, transcodeHandler *publichttp.TranscodeHandler, clusterHandler *publichttp.ClusterHandler, liveHandler *publichttp.LiveHandler, manifestHandler *publichttp.ManifestHandler, authHandler *adminhttp.AuthHandler, configHandler *adminhttp.ConfigHandler, callbackHandler *adminhttp.CallbackHandler, rbacHandler *adminhttp.RBACHandler, writeRBACHandler *adminhttp.WriteRBAC, configCenterHandler *adminhttp.ConfigCenterHandler, adminClusterHandler *adminhttp.ClusterHandler, adminTranscodeHandler *adminhttp.TranscodeHandler, adminLiveHandler *adminhttp.LiveHandler) *HTTPServer {
	return &HTTPServer{
		listenAddress:        cfg.ListenAddress,
		systemHandler:        systemHandler,
		transcodeHandler:     transcodeHandler,
		clusterHandler:       clusterHandler,
		liveHandler:          liveHandler,
		manifestHandler:      manifestHandler,
		authHandler:          authHandler,
		configHandler:        configHandler,
		callbackHandler:      callbackHandler,
		rbacHandler:          rbacHandler,
		writeRBACHandler:     writeRBACHandler,
		configCenterHandle:   configCenterHandler,
		adminClusterHandle:   adminClusterHandler,
		adminTranscodeHandle: adminTranscodeHandler,
		adminLiveHandle:      adminLiveHandler,
	}
}

// Start 启动 HTTP 服务。
func (s *HTTPServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.systemHandler.Health)
	mux.HandleFunc("/v1/transcode/job/create", s.transcodeHandler.CreateJob)
	mux.HandleFunc("/v1/transcode/job/progress", s.transcodeHandler.QueryProgress)
	mux.HandleFunc("/v1/live/channel/playback", s.liveHandler.GetPlaybackInfo)
	mux.HandleFunc("/v1/manifest/dash/{job_id}.mpd", s.manifestHandler.ServeMPD)
	mux.HandleFunc("/v1/manifest/hls/{job_id}.m3u8", s.manifestHandler.ServeMasterM3U8)
	mux.HandleFunc("/v1/manifest/hls/{job_id}/{rendition}.m3u8", s.manifestHandler.ServeVariantM3U8)
	mux.HandleFunc("/v1/internal/worker/heartbeat", s.clusterHandler.ReportHeartbeat)
	mux.HandleFunc("/v1/internal/worker/metrics", s.clusterHandler.ReportMetrics)
	mux.HandleFunc("/v1/internal/jobs/lease/renew", s.clusterHandler.RenewLease)
	mux.HandleFunc("/v1/internal/segments/upload-failed", s.clusterHandler.ReportUploadFailed)
	mux.HandleFunc("/v1/internal/segments/upload-succeeded", s.clusterHandler.ReportUploadSucceeded)

	mux.HandleFunc("/v1/admin/auth/login", s.authHandler.Login)
	mux.Handle("/v1/admin/auth/logout", httpmiddleware.RequirePermission("auth.session.write", http.HandlerFunc(s.authHandler.Logout)))
	mux.Handle("/v1/admin/auth/me", httpmiddleware.RequirePermission("auth.session.read", http.HandlerFunc(s.authHandler.WhoAmI)))
	mux.Handle("/v1/admin/ping", httpmiddleware.RequirePermission("system.user.read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logx.WriteJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok"})
	})))
	mux.Handle("/v1/admin/config/runtime/versions", httpmiddleware.RequirePermission("config.version.read", http.HandlerFunc(s.configHandler.ListRuntimeVersions)))
	mux.Handle("/v1/admin/config/runtime/update", httpmiddleware.RequirePermission("config.runtime.update", http.HandlerFunc(s.configHandler.UpdateRuntime)))
	mux.Handle("/v1/admin/config/publish", httpmiddleware.RequirePermission("config.version.publish", http.HandlerFunc(s.configHandler.PublishRuntime)))
	mux.Handle("/v1/admin/config/callback/list", httpmiddleware.RequirePermission("config.callback.read", http.HandlerFunc(s.callbackHandler.List)))
	mux.Handle("/v1/admin/config/callback/upsert", httpmiddleware.RequirePermission("config.callback.update", http.HandlerFunc(s.callbackHandler.Upsert)))
	mux.Handle("/v1/admin/config/callback/enabled", httpmiddleware.RequirePermission("config.callback.update", http.HandlerFunc(s.callbackHandler.SetEnabled)))
	mux.Handle("/v1/admin/config-center/list", httpmiddleware.RequirePermission("config.version.read", http.HandlerFunc(s.configCenterHandle.List)))
	mux.Handle("/v1/admin/config-center/upsert", httpmiddleware.RequirePermission("config.version.publish", http.HandlerFunc(s.configCenterHandle.Upsert)))
	mux.Handle("/v1/admin/config-center/enabled", httpmiddleware.RequirePermission("config.version.publish", http.HandlerFunc(s.configCenterHandle.SetEnabled)))
	mux.Handle("/v1/admin/system/user/list", httpmiddleware.RequirePermission("system.user.read", http.HandlerFunc(s.rbacHandler.ListUsers)))
	mux.Handle("/v1/admin/system/role/list", httpmiddleware.RequirePermission("system.role.read", http.HandlerFunc(s.rbacHandler.ListRoles)))
	mux.Handle("/v1/admin/system/permission/list", httpmiddleware.RequirePermission("system.permission.read", http.HandlerFunc(s.rbacHandler.ListPermissions)))
	mux.Handle("/v1/admin/system/role/upsert", httpmiddleware.RequirePermission("system.role.update", http.HandlerFunc(s.writeRBACHandler.UpsertRole)))
	mux.Handle("/v1/admin/system/permission/upsert", httpmiddleware.RequirePermission("system.role.permission_bind", http.HandlerFunc(s.writeRBACHandler.UpsertPermission)))
	mux.Handle("/v1/admin/system/user/upsert", httpmiddleware.RequirePermission("system.user.create", http.HandlerFunc(s.writeRBACHandler.UpsertUser)))
	mux.Handle("/v1/admin/system/user/status", httpmiddleware.RequirePermission("system.user.update", http.HandlerFunc(s.writeRBACHandler.SetUserStatus)))
	mux.Handle("/v1/admin/system/user-role/bind", httpmiddleware.RequirePermission("system.user.role_bind", http.HandlerFunc(s.writeRBACHandler.BindUserRole)))
	mux.Handle("/v1/admin/system/role-permission/bind", httpmiddleware.RequirePermission("system.role.permission_bind", http.HandlerFunc(s.writeRBACHandler.BindRolePermission)))
	mux.Handle("/v1/admin/audit/list", httpmiddleware.RequirePermission("audit.read", http.HandlerFunc(s.rbacHandler.ListAuditLogs)))
	mux.Handle("/v1/admin/cluster/node/list", httpmiddleware.RequirePermission("cluster.node.read", http.HandlerFunc(s.adminClusterHandle.ListNodes)))
	mux.Handle("/v1/admin/cluster/node/detail", httpmiddleware.RequirePermission("cluster.node.read", http.HandlerFunc(s.adminClusterHandle.GetNodeDetail)))
	mux.Handle("/v1/admin/cluster/node/metrics", httpmiddleware.RequirePermission("cluster.node.metrics.read", http.HandlerFunc(s.adminClusterHandle.ListNodeMetrics)))
	mux.Handle("/v1/admin/cluster/member/list", httpmiddleware.RequirePermission("cluster.read", http.HandlerFunc(s.adminClusterHandle.ListMembers)))
	mux.Handle("/v1/admin/cluster/node/enabled", httpmiddleware.RequirePermission("cluster.node.enable", http.HandlerFunc(s.adminClusterHandle.SetNodeEnabled)))
	mux.Handle("/v1/admin/cluster/node/quarantined", httpmiddleware.RequirePermission("cluster.node.quarantine", http.HandlerFunc(s.adminClusterHandle.SetNodeQuarantined)))

	mux.Handle("/v1/admin/transcode/job/list", httpmiddleware.RequirePermission("transcode.job.read", http.HandlerFunc(s.adminTranscodeHandle.ListJobs)))
	mux.Handle("/v1/admin/transcode/job/detail", httpmiddleware.RequirePermission("transcode.job.detail.read", http.HandlerFunc(s.adminTranscodeHandle.JobDetail)))
	mux.Handle("/v1/admin/transcode/job/retry", httpmiddleware.RequirePermission("transcode.job.retry", http.HandlerFunc(s.adminTranscodeHandle.RetryJob)))
	mux.Handle("/v1/admin/transcode/job/cancel", httpmiddleware.RequirePermission("transcode.job.cancel", http.HandlerFunc(s.adminTranscodeHandle.CancelJob)))
	mux.Handle("/v1/admin/transcode/job/progress", httpmiddleware.RequirePermission("transcode.job.read", http.HandlerFunc(s.adminTranscodeHandle.JobProgress)))

	mux.Handle("/v1/admin/live/channel/create", httpmiddleware.RequirePermission("live.channel.create", http.HandlerFunc(s.adminLiveHandle.CreateChannel)))
	mux.Handle("/v1/admin/live/channel/detail", httpmiddleware.RequirePermission("live.channel.read", http.HandlerFunc(s.adminLiveHandle.ChannelDetail)))
	mux.Handle("/v1/admin/live/channel/update", httpmiddleware.RequirePermission("live.channel.update", http.HandlerFunc(s.adminLiveHandle.UpdateChannel)))
	mux.Handle("/v1/admin/live/channel/start", httpmiddleware.RequirePermission("live.channel.start", http.HandlerFunc(s.adminLiveHandle.StartChannel)))
	mux.Handle("/v1/admin/live/channel/stop", httpmiddleware.RequirePermission("live.channel.stop", http.HandlerFunc(s.adminLiveHandle.StopChannel)))

	server := &http.Server{
		Addr:    s.listenAddress,
		Handler: logx.Middleware(mux),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
