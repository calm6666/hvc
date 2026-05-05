package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const configCenterCacheKey = "hvc:runtime:config:center"

// MergeConfigCenterRuntimeConfig 从配置中心读取并合并运行配置。
func MergeConfigCenterRuntimeConfig(cfg RuntimeConfig) (RuntimeConfig, error) {
	if !cfg.ConfigCenter.Enabled {
		return cfg, nil
	}
	fetched, err := FetchConfigCenterRuntimeConfig(cfg.ConfigCenter, cfg.Redis)
	if err != nil {
		return RuntimeConfig{}, err
	}
	if fetched.Server.ListenAddress != "" {
		cfg.Server.ListenAddress = fetched.Server.ListenAddress
	}
	if fetched.Server.ServiceName != "" {
		cfg.Server.ServiceName = fetched.Server.ServiceName
	}
	if fetched.Server.NodeID != 0 {
		cfg.Server.NodeID = fetched.Server.NodeID
	}
	if fetched.Server.WorkerID != "" {
		cfg.Server.WorkerID = fetched.Server.WorkerID
	}
	if len(fetched.Redis.Addrs) > 0 {
		cfg.Redis.Addrs = fetched.Redis.Addrs
	}
	if fetched.Redis.Password != "" {
		cfg.Redis.Password = fetched.Redis.Password
	}
	if fetched.Redis.DB != 0 {
		cfg.Redis.DB = fetched.Redis.DB
	}
	if fetched.Scheduler.LoopInterval != 0 {
		cfg.Scheduler.LoopInterval = fetched.Scheduler.LoopInterval
	}
	if fetched.Scheduler.JobLeaseTTL != 0 {
		cfg.Scheduler.JobLeaseTTL = fetched.Scheduler.JobLeaseTTL
	}
	if fetched.Scheduler.WorkerHeartbeatTimeout != 0 {
		cfg.Scheduler.WorkerHeartbeatTimeout = fetched.Scheduler.WorkerHeartbeatTimeout
	}
	if fetched.Worker.LoopInterval != 0 {
		cfg.Worker.LoopInterval = fetched.Worker.LoopInterval
	}
	return cfg, nil
}

// FetchConfigCenterRuntimeConfig 读取配置中心配置。
func FetchConfigCenterRuntimeConfig(configCenter ConfigCenterConfig, redisConfig RedisConfig) (RuntimeConfig, error) {
	if strings.TrimSpace(configCenter.Endpoint) != "" {
		return fetchFromConfigCenterHTTP(configCenter)
	}
	engine := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        redisConfig.Addrs,
		Password:     redisConfig.Password,
		DB:           redisConfig.DB,
		DialTimeout:  redisConfig.DialTimeout,
		ReadTimeout:  redisConfig.ReadTimeout,
		WriteTimeout: redisConfig.WriteTimeout,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	payload, err := engine.Get(ctx, configCenterCacheKey).Result()
	if err == redis.Nil {
		return RuntimeConfig{}, fmt.Errorf("config center payload is empty")
	}
	if err != nil {
		return RuntimeConfig{}, err
	}
	var cfg RuntimeConfig
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		return RuntimeConfig{}, err
	}
	return cfg, nil
}

func fetchFromConfigCenterHTTP(configCenter ConfigCenterConfig) (RuntimeConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configCenter.Endpoint, nil)
	if err != nil {
		return RuntimeConfig{}, err
	}
	if configCenter.Token != "" {
		req.Header.Set("Authorization", "Bearer "+configCenter.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return RuntimeConfig{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return RuntimeConfig{}, fmt.Errorf("config center http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RuntimeConfig{}, err
	}
	var cfg RuntimeConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return RuntimeConfig{}, err
	}
	return cfg, nil
}
