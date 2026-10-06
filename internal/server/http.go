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
	wsmonitor "hvc/internal/interfaces/ws/monitor"
	"hvc/pkg/logx"
)

// HTTPServer 表示 HTTP 服务。
type HTTPServer struct {
	listenAddress        string
	nodeMode             string
	internalAuthToken    string
	systemHandler        *handler.SystemHandler
	transcodeHandler     *publichttp.TranscodeHandler
	clusterHandler       *publichttp.ClusterHandler
	liveHandler          *publichttp.LiveHandler
	manifestHandler      *publichttp.ManifestHandler
	authHandler          *adminhttp.AuthHandler
	configHandler        *adminhttp.ConfigHandler
	callbackHandler      *adminhttp.CallbackHandler
	registryEtcdHandler  *adminhttp.RegistryEtcdHandler
	rbacHandler          *adminhttp.RBACHandler
	writeRBACHandler     *adminhttp.WriteRBAC
	configCenterHandle   *adminhttp.ConfigCenterHandler
	adminClusterHandle   *adminhttp.ClusterHandler
	adminTranscodeHandle *adminhttp.TranscodeHandler
	adminLiveHandle      *adminhttp.LiveHandler
	namingTemplateHandle *adminhttp.NamingTemplateHandler
	monitorHandle        *wsmonitor.SnapshotHandler
}

// NewHTTPServer 创建 HTTP 服务。
func NewHTTPServer(cfg config.ServerConfig, internalAuthToken string, systemHandler *handler.SystemHandler, transcodeHandler *publichttp.TranscodeHandler, clusterHandler *publichttp.ClusterHandler, liveHandler *publichttp.LiveHandler, manifestHandler *publichttp.ManifestHandler, authHandler *adminhttp.AuthHandler, configHandler *adminhttp.ConfigHandler, callbackHandler *adminhttp.CallbackHandler, registryEtcdHandler *adminhttp.RegistryEtcdHandler, rbacHandler *adminhttp.RBACHandler, writeRBACHandler *adminhttp.WriteRBAC, configCenterHandler *adminhttp.ConfigCenterHandler, adminClusterHandler *adminhttp.ClusterHandler, adminTranscodeHandler *adminhttp.TranscodeHandler, adminLiveHandler *adminhttp.LiveHandler, namingTemplateHandler *adminhttp.NamingTemplateHandler, monitorHandler *wsmonitor.SnapshotHandler) *HTTPServer {
	return &HTTPServer{
		listenAddress:        cfg.ListenAddress,
		nodeMode:             cfg.NodeMode,
		internalAuthToken:    internalAuthToken,
		systemHandler:        systemHandler,
		transcodeHandler:     transcodeHandler,
		clusterHandler:       clusterHandler,
		liveHandler:          liveHandler,
		manifestHandler:      manifestHandler,
		authHandler:          authHandler,
		configHandler:        configHandler,
		callbackHandler:      callbackHandler,
		registryEtcdHandler:  registryEtcdHandler,
		rbacHandler:          rbacHandler,
		writeRBACHandler:     writeRBACHandler,
		configCenterHandle:   configCenterHandler,
		adminClusterHandle:   adminClusterHandler,
		adminTranscodeHandle: adminTranscodeHandler,
		adminLiveHandle:      adminLiveHandler,
		namingTemplateHandle: namingTemplateHandler,
		monitorHandle:        monitorHandler,
	}
}

// Start 启动 HTTP 服务。
func (s *HTTPServer) Start(ctx context.Context) error {
	logx.Info("http.server.listening", logx.Fields{
		"address": s.listenAddress,
	})
	mux := http.NewServeMux()
	internalOnly := func(next http.HandlerFunc) http.Handler {
		return httpmiddleware.RequireInternalAccess(s.internalAuthToken, next)
	}
	mux.HandleFunc("/healthz", s.systemHandler.Health)
	mux.Handle("/v1/internal/live/publish-connected", internalOnly(s.liveHandler.PublishConnected))
	mux.Handle("/v1/internal/live/publish-disconnected", internalOnly(s.liveHandler.PublishDisconnected))
	mux.Handle("/v1/internal/live/publish-interrupted", internalOnly(s.liveHandler.PublishInterrupted))
	mux.Handle("/v1/internal/live/channel/start", internalOnly(s.liveHandler.StartRequested))
	mux.Handle("/v1/internal/live/channel/stop", internalOnly(s.liveHandler.StopRequested))
	mux.Handle("/v1/internal/worker/heartbeat", internalOnly(s.clusterHandler.ReportHeartbeat))
	mux.Handle("/v1/internal/worker/metrics", internalOnly(s.clusterHandler.ReportMetrics))
	mux.Handle("/v1/internal/jobs/lease/renew", internalOnly(s.clusterHandler.RenewLease))
	mux.Handle("/v1/internal/segments/upload-failed", internalOnly(s.clusterHandler.ReportUploadFailed))
	mux.Handle("/v1/internal/segments/upload-succeeded", internalOnly(s.clusterHandler.ReportUploadSucceeded))

	if s.nodeMode != config.NodeModeClusterWorker {
		mux.HandleFunc("/v1/transcode/job/create", s.transcodeHandler.CreateJob)
		mux.HandleFunc("/v1/transcode/job/progress", s.transcodeHandler.QueryProgress)
		mux.HandleFunc("/v1/live/channel/playback", s.liveHandler.GetPlaybackInfo)
		mux.HandleFunc("/v1/live/channel/push-auth", s.liveHandler.PushAuth)
		mux.HandleFunc("/v1/live/channel/play-auth", s.liveHandler.PlayAuth)
		mux.HandleFunc("/v1/manifest/dash/{job_id}", s.manifestHandler.ServeMPD)
		mux.HandleFunc("/v1/manifest/hls/{job_id}", s.manifestHandler.ServeMasterM3U8)
		mux.HandleFunc("/v1/manifest/hls/{job_id}/{rendition}", s.manifestHandler.ServeVariantM3U8)

		mux.HandleFunc("/v1/admin/auth/login", s.authHandler.Login)
		mux.Handle("/v1/admin/auth/logout", httpmiddleware.RequirePermission("auth.session.write", http.HandlerFunc(s.authHandler.Logout)))
		mux.Handle("/v1/admin/auth/profile", httpmiddleware.RequirePermission("auth.session.read", http.HandlerFunc(s.authHandler.WhoAmI)))
		mux.Handle("/v1/admin/auth/current-user", httpmiddleware.RequirePermission("auth.session.read", http.HandlerFunc(s.authHandler.WhoAmI)))
		mux.Handle("/v1/admin/auth/session", httpmiddleware.RequirePermission("auth.session.read", http.HandlerFunc(s.authHandler.WhoAmI)))
		mux.Handle("/v1/admin/auth/me", httpmiddleware.RequirePermission("auth.session.read", http.HandlerFunc(s.authHandler.WhoAmI)))
		mux.Handle("/v1/admin/ping", httpmiddleware.RequirePermission("system.user.read", http.HandlerFunc(adminhttp.Ping)))
		mux.Handle("/v1/admin/config/runtime/versions", httpmiddleware.RequirePermission("config.version.read", http.HandlerFunc(s.configHandler.ListRuntimeVersions)))
		mux.Handle("/v1/admin/config/publish", httpmiddleware.RequirePermission("config.version.publish", http.HandlerFunc(s.configHandler.PublishRuntime)))
		mux.Handle("/v1/admin/config/callback/list", httpmiddleware.RequirePermission("config.callback.read", http.HandlerFunc(s.callbackHandler.List)))
		mux.Handle("/v1/admin/config/callback/upsert", httpmiddleware.RequirePermission("config.callback.update", http.HandlerFunc(s.callbackHandler.Upsert)))
		mux.Handle("/v1/admin/config/callback/enabled", httpmiddleware.RequirePermission("config.callback.update", http.HandlerFunc(s.callbackHandler.SetEnabled)))
		mux.Handle("/v1/admin/config/runtime/server/update", httpmiddleware.RequirePermission("config.runtime.update", http.HandlerFunc(s.configHandler.UpdateRuntimeServer)))
		mux.Handle("/v1/admin/config/runtime/scheduler/update", httpmiddleware.RequirePermission("config.runtime.update", http.HandlerFunc(s.configHandler.UpdateRuntimeScheduler)))
		mux.Handle("/v1/admin/config/runtime/worker/update", httpmiddleware.RequirePermission("config.runtime.update", http.HandlerFunc(s.configHandler.UpdateRuntimeWorker)))
		mux.Handle("/v1/admin/config/runtime/storage/update", httpmiddleware.RequirePermission("config.runtime.update", http.HandlerFunc(s.configHandler.UpdateRuntimeStorage)))
		mux.Handle("/v1/admin/config/runtime/callback/update", httpmiddleware.RequirePermission("config.runtime.update", http.HandlerFunc(s.configHandler.UpdateRuntimeCallback)))
		mux.Handle("/v1/admin/config/runtime/mq/update", httpmiddleware.RequirePermission("config.runtime.update", http.HandlerFunc(s.configHandler.UpdateRuntimeMQ)))
		mux.Handle("/v1/admin/config/runtime/grpc/update", httpmiddleware.RequirePermission("config.runtime.update", http.HandlerFunc(s.configHandler.UpdateRuntimeGRPC)))
		mux.Handle("/v1/admin/config/naming-template/list", httpmiddleware.RequirePermission("config.naming_template.read", http.HandlerFunc(s.namingTemplateHandle.ListTemplates)))
		mux.Handle("/v1/admin/config/naming-template/configure", httpmiddleware.RequirePermission("config.naming_template.update", http.HandlerFunc(s.namingTemplateHandle.ConfigureTemplate)))
		mux.Handle("/v1/admin/config/naming-template/activate", httpmiddleware.RequirePermission("config.naming_template.update", http.HandlerFunc(s.namingTemplateHandle.ActivateTemplate)))
		mux.Handle("/v1/admin/config-center/list", httpmiddleware.RequirePermission("config.version.read", http.HandlerFunc(s.configCenterHandle.List)))
		mux.Handle("/v1/admin/config-center/upsert", httpmiddleware.RequirePermission("config.version.publish", http.HandlerFunc(s.configCenterHandle.Upsert)))
		mux.Handle("/v1/admin/config-center/enabled", httpmiddleware.RequirePermission("config.version.publish", http.HandlerFunc(s.configCenterHandle.SetEnabled)))
		mux.Handle("/v1/admin/registry/etcd/list", httpmiddleware.RequirePermission("config.version.read", http.HandlerFunc(s.registryEtcdHandler.List)))
		mux.Handle("/v1/admin/registry/etcd/upsert", httpmiddleware.RequirePermission("config.runtime.update", http.HandlerFunc(s.registryEtcdHandler.Upsert)))
		mux.Handle("/v1/admin/registry/etcd/enabled", httpmiddleware.RequirePermission("config.runtime.update", http.HandlerFunc(s.registryEtcdHandler.SetEnabled)))
		mux.Handle("/v1/admin/system/user/list", httpmiddleware.RequirePermission("system.user.read", http.HandlerFunc(s.rbacHandler.ListUsers)))
		mux.Handle("/v1/admin/system/role/list", httpmiddleware.RequirePermission("system.role.read", http.HandlerFunc(s.rbacHandler.ListRoles)))
		mux.Handle("/v1/admin/system/role/all", httpmiddleware.RequirePermission("system.role.read", http.HandlerFunc(s.rbacHandler.ListAllRoles)))
		mux.Handle("/v1/admin/system/permission/list", httpmiddleware.RequirePermission("system.permission.read", http.HandlerFunc(s.rbacHandler.ListPermissions)))
		mux.Handle("/v1/admin/system/permission/tree", httpmiddleware.RequirePermission("system.permission.read", http.HandlerFunc(s.rbacHandler.PermissionTree)))
		mux.Handle("/v1/admin/system/menu/tree", httpmiddleware.RequirePermission("system.menu.read", http.HandlerFunc(s.rbacHandler.ListMenuTree)))
		mux.Handle("/v1/admin/system/menu/current-tree", httpmiddleware.RequirePermission("auth.session.read", http.HandlerFunc(s.rbacHandler.CurrentMenuTree)))
		mux.Handle("/v1/admin/system/role/menu/tree", httpmiddleware.RequirePermission("system.menu.read", http.HandlerFunc(s.rbacHandler.RoleMenuTree)))
		mux.Handle("/v1/admin/system/role/upsert", httpmiddleware.RequirePermission("system.role.update", http.HandlerFunc(s.writeRBACHandler.UpsertRole)))
		mux.Handle("/v1/admin/system/permission/upsert", httpmiddleware.RequirePermission("system.role.permission_bind", http.HandlerFunc(s.writeRBACHandler.UpsertPermission)))
		mux.Handle("/v1/admin/system/user/upsert", httpmiddleware.RequirePermission("system.user.create", http.HandlerFunc(s.writeRBACHandler.UpsertUser)))
		mux.Handle("/v1/admin/system/user/status", httpmiddleware.RequirePermission("system.user.update", http.HandlerFunc(s.writeRBACHandler.SetUserStatus)))
		mux.Handle("/v1/admin/system/menu/upsert", httpmiddleware.RequirePermission("system.menu.update", http.HandlerFunc(s.writeRBACHandler.UpsertMenu)))
		mux.Handle("/v1/admin/system/menu/delete", httpmiddleware.RequirePermission("system.menu.delete", http.HandlerFunc(s.writeRBACHandler.DeleteMenu)))
		mux.Handle("/v1/admin/system/user-role/bind", httpmiddleware.RequirePermission("system.user.role_bind", http.HandlerFunc(s.writeRBACHandler.BindUserRole)))
		mux.Handle("/v1/admin/system/role-permission/bind", httpmiddleware.RequirePermission("system.role.permission_bind", http.HandlerFunc(s.writeRBACHandler.BindRolePermission)))
		mux.Handle("/v1/admin/system/role-menu/assign", httpmiddleware.RequirePermission("system.role.menu_bind", http.HandlerFunc(s.writeRBACHandler.AssignRoleMenus)))
		mux.Handle("/v1/admin/audit/list", httpmiddleware.RequirePermission("audit.read", http.HandlerFunc(s.rbacHandler.ListAuditLogs)))
		mux.Handle("/v1/admin/system/log/list", httpmiddleware.RequirePermission("system.log.read", http.HandlerFunc(s.rbacHandler.ListRuntimeLogs)))
		mux.Handle("/v1/admin/cluster/node/list", httpmiddleware.RequirePermission("cluster.node.read", http.HandlerFunc(s.adminClusterHandle.ListNodes)))
		mux.Handle("/v1/admin/cluster/node/detail", httpmiddleware.RequirePermission("cluster.node.read", http.HandlerFunc(s.adminClusterHandle.GetNodeDetail)))
		mux.Handle("/v1/admin/cluster/node/metrics", httpmiddleware.RequirePermission("cluster.node.metrics.read", http.HandlerFunc(s.adminClusterHandle.ListNodeMetrics)))
		mux.Handle("/v1/admin/cluster/overview", httpmiddleware.RequirePermission("cluster.read", http.HandlerFunc(s.adminClusterHandle.Overview)))
		mux.Handle("/v1/admin/cluster/realtime", httpmiddleware.RequirePermission("cluster.read", http.HandlerFunc(s.adminClusterHandle.Realtime)))
		mux.Handle("/v1/admin/cluster/topology", httpmiddleware.RequirePermission("cluster.read", http.HandlerFunc(s.adminClusterHandle.Topology)))
		mux.Handle("/v1/admin/cluster/resource/distribution", httpmiddleware.RequirePermission("cluster.read", http.HandlerFunc(s.adminClusterHandle.ResourceDistribution)))
		mux.Handle("/v1/admin/cluster/member/list", httpmiddleware.RequirePermission("cluster.read", http.HandlerFunc(s.adminClusterHandle.ListMembers)))
		mux.Handle("/v1/admin/cluster/worker/list", httpmiddleware.RequirePermission("cluster.read", http.HandlerFunc(s.adminClusterHandle.ListWorkers)))
		mux.Handle("/v1/admin/cluster/scheduler/insight", httpmiddleware.RequirePermission("cluster.read", http.HandlerFunc(s.adminClusterHandle.SchedulerInsight)))
		mux.Handle("/v1/admin/cluster/worker/offline", httpmiddleware.RequirePermission("cluster.worker.offline", http.HandlerFunc(s.adminClusterHandle.SetWorkerOffline)))
		mux.Handle("/v1/admin/cluster/worker/exit", httpmiddleware.RequirePermission("cluster.worker.exit", http.HandlerFunc(s.adminClusterHandle.SetWorkerExited)))
		mux.Handle("/v1/admin/cluster/job/takeover", httpmiddleware.RequirePermission("cluster.job.takeover", http.HandlerFunc(s.adminClusterHandle.ForceTakeoverJobs)))
		mux.Handle("/v1/admin/cluster/node/enabled", httpmiddleware.RequirePermission("cluster.node.enable", http.HandlerFunc(s.adminClusterHandle.SetNodeEnabled)))
		mux.Handle("/v1/admin/cluster/node/quarantined", httpmiddleware.RequirePermission("cluster.node.quarantine", http.HandlerFunc(s.adminClusterHandle.SetNodeQuarantined)))
		mux.Handle("/v1/admin/cluster/node/draining", httpmiddleware.RequirePermission("cluster.node.drain", http.HandlerFunc(s.adminClusterHandle.SetNodeDraining)))

		mux.Handle("/v1/admin/transcode/job/list", httpmiddleware.RequirePermission("transcode.job.read", http.HandlerFunc(s.adminTranscodeHandle.ListJobs)))
		mux.Handle("/v1/admin/transcode/job/detail", httpmiddleware.RequirePermission("transcode.job.detail.read", http.HandlerFunc(s.adminTranscodeHandle.JobDetail)))
		mux.Handle("/v1/admin/transcode/job/retry", httpmiddleware.RequirePermission("transcode.job.retry", http.HandlerFunc(s.adminTranscodeHandle.RetryJob)))
		mux.Handle("/v1/admin/transcode/job/cancel", httpmiddleware.RequirePermission("transcode.job.cancel", http.HandlerFunc(s.adminTranscodeHandle.CancelJob)))
		mux.Handle("/v1/admin/transcode/job/progress", httpmiddleware.RequirePermission("transcode.job.read", http.HandlerFunc(s.adminTranscodeHandle.JobProgress)))
		mux.Handle("/v1/admin/transcode/monitor/snapshot", httpmiddleware.RequirePermission("transcode.job.read", http.HandlerFunc(s.monitorHandle.Snapshot)))
		mux.Handle("/v1/admin/transcode/monitor/ws", httpmiddleware.RequirePermission("transcode.job.read", http.HandlerFunc(s.monitorHandle.HandleWS)))

		mux.Handle("/v1/admin/live/channel/create", httpmiddleware.RequirePermission("live.channel.create", http.HandlerFunc(s.adminLiveHandle.CreateChannel)))
		mux.Handle("/v1/admin/live/channel/list", httpmiddleware.RequirePermission("live.channel.read", http.HandlerFunc(s.adminLiveHandle.ListChannels)))
		mux.Handle("/v1/admin/live/channel/detail", httpmiddleware.RequirePermission("live.channel.read", http.HandlerFunc(s.adminLiveHandle.ChannelDetail)))
		mux.Handle("/v1/admin/live/channel/update", httpmiddleware.RequirePermission("live.channel.update", http.HandlerFunc(s.adminLiveHandle.UpdateChannel)))
		mux.Handle("/v1/admin/live/channel/start", httpmiddleware.RequirePermission("live.channel.start", http.HandlerFunc(s.adminLiveHandle.StartChannel)))
		mux.Handle("/v1/admin/live/channel/stop", httpmiddleware.RequirePermission("live.channel.stop", http.HandlerFunc(s.adminLiveHandle.StopChannel)))
		mux.Handle("/v1/admin/live/channel/delete", httpmiddleware.RequirePermission("live.channel.delete", http.HandlerFunc(s.adminLiveHandle.DeleteChannel)))
		mux.Handle("/v1/admin/live/session/list", httpmiddleware.RequirePermission("live.session.read", http.HandlerFunc(s.adminLiveHandle.ListSessions)))
	}

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
