package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"hvc/internal/config"
	"hvc/internal/handler"
	publichttp "hvc/internal/interfaces/http/public"
	httpmiddleware "hvc/internal/interfaces/http/middleware"
	"hvc/pkg/logx"
)

// HTTPServer 表示 HTTP 服务。
type HTTPServer struct {
	listenAddress    string
	systemHandler    *handler.SystemHandler
	transcodeHandler *publichttp.TranscodeHandler
	clusterHandler   *publichttp.ClusterHandler
	liveHandler      *publichttp.LiveHandler
}

// NewHTTPServer 创建 HTTP 服务。
func NewHTTPServer(cfg config.ServerConfig, systemHandler *handler.SystemHandler, transcodeHandler *publichttp.TranscodeHandler, clusterHandler *publichttp.ClusterHandler, liveHandler *publichttp.LiveHandler) *HTTPServer {
	return &HTTPServer{
		listenAddress:    cfg.ListenAddress,
		systemHandler:    systemHandler,
		transcodeHandler: transcodeHandler,
		clusterHandler:   clusterHandler,
		liveHandler:      liveHandler,
	}
}

// Start 启动 HTTP 服务。
func (s *HTTPServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.systemHandler.Health)
	mux.HandleFunc("/v1/transcode/job/create", s.transcodeHandler.CreateJob)
	mux.HandleFunc("/v1/transcode/job/progress", s.transcodeHandler.QueryProgress)
	mux.HandleFunc("/v1/live/channel/playback", s.liveHandler.GetPlaybackInfo)
	mux.HandleFunc("/v1/internal/worker/heartbeat", s.clusterHandler.ReportHeartbeat)
	mux.HandleFunc("/v1/internal/worker/metrics", s.clusterHandler.ReportMetrics)
	mux.HandleFunc("/v1/internal/jobs/lease/renew", s.clusterHandler.RenewLease)
	mux.HandleFunc("/v1/internal/segments/upload-failed", s.clusterHandler.ReportUploadFailed)
	mux.HandleFunc("/v1/internal/segments/upload-succeeded", s.clusterHandler.ReportUploadSucceeded)
	mux.Handle("/v1/admin/ping", httpmiddleware.RequirePermission("system.user.read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logx.WriteJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "ok"})
	})))
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
