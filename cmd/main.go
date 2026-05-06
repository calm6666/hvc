package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"hvc/internal/bootstrap"
	"hvc/internal/config"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

func main() {
	cfg, err := config.LoadRuntimeConfig()
	if err != nil {
		panic(err)
	}
	startTime, err := time.Parse(time.RFC3339, cfg.ID.StartTime)
	if err != nil {
		panic(err)
	}
	idgen.Configure(startTime, cfg.Server.NodeID, cfg.ID.NodeBits, cfg.ID.SequenceBits)
	logx.Init(cfg.Server.ServiceName)
	defer logx.Sync()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	app, err := bootstrap.NewApplication(cfg)
	if err != nil {
		panic(err)
	}
	if err := app.Run(ctx); err != nil {
		panic(err)
	}
}
