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

// LoadDynamicRuntimeConfig 加载动态运行配置。
func LoadDynamicRuntimeConfig(base RuntimeConfig) (DynamicRuntimeConfig, error) {
	if base.ConfigCenter.Enabled {
		return FetchConfigCenterRuntimeConfig(base.ConfigCenter, base.Redis)
	}
	return DynamicRuntimeConfig{}, fmt.Errorf("dynamic runtime config must be loaded from database or redis cache")
}

// FetchConfigCenterRuntimeConfig 读取配置中心配置。
func FetchConfigCenterRuntimeConfig(configCenter ConfigCenterConfig, redisConfig RedisConfig) (DynamicRuntimeConfig, error) {
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
		return DynamicRuntimeConfig{}, fmt.Errorf("config center payload is empty")
	}
	if err != nil {
		return DynamicRuntimeConfig{}, err
	}
	var cfg DynamicRuntimeConfig
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		return DynamicRuntimeConfig{}, err
	}
	return cfg, nil
}

func fetchFromConfigCenterHTTP(configCenter ConfigCenterConfig) (DynamicRuntimeConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configCenter.Endpoint, nil)
	if err != nil {
		return DynamicRuntimeConfig{}, err
	}
	if configCenter.Token != "" {
		req.Header.Set("Authorization", "Bearer "+configCenter.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DynamicRuntimeConfig{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return DynamicRuntimeConfig{}, fmt.Errorf("config center http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DynamicRuntimeConfig{}, err
	}
	var cfg DynamicRuntimeConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return DynamicRuntimeConfig{}, err
	}
	return cfg, nil
}
