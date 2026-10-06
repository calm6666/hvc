package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadRuntimeConfigFromConfigCenter 从外部 bootstrap 配置源加载启动配置。
//
// 当前实现支持两类最小可用 provider：
// 1. `file://` 或普通文件路径：读取本地 YAML/JSON 文件；
// 2. `http://` / `https://`：拉取远端 YAML/JSON 文本。
//
// 这里故意只覆盖 bootstrap 配置，不允许它承载 runtime 业务动态配置。
func LoadRuntimeConfigFromConfigCenter(base RuntimeConfig, localConfigPath string) (RuntimeConfig, bool, error) {
	endpoint := strings.TrimSpace(base.ConfigCenter.Endpoint)
	if !base.ConfigCenter.Enabled || endpoint == "" {
		return base, false, nil
	}

	content, err := loadBootstrapConfigCenterPayload(base.ConfigCenter, endpoint, localConfigPath)
	if err != nil {
		return RuntimeConfig{}, false, err
	}
	if len(content) == 0 {
		return base, false, nil
	}

	var cfg RuntimeConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return RuntimeConfig{}, false, err
	}

	// 外部 bootstrap 配置可以不重复声明 config_center 段；
	// 当前进程实际使用的 bootstrap source 元信息仍保留本地入口里的这一份。
	if cfg.ConfigCenter == (ConfigCenterConfig{}) {
		cfg.ConfigCenter = base.ConfigCenter
	}
	return cfg, true, nil
}

func loadBootstrapConfigCenterPayload(cfg ConfigCenterConfig, endpoint string, localConfigPath string) ([]byte, error) {
	switch {
	case strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://"):
		return loadBootstrapConfigCenterHTTP(cfg, endpoint)
	case strings.HasPrefix(endpoint, "file://"):
		return os.ReadFile(strings.TrimSpace(strings.TrimPrefix(endpoint, "file://")))
	default:
		return os.ReadFile(resolveBootstrapConfigCenterFilePath(endpoint, localConfigPath))
	}
}

func loadBootstrapConfigCenterHTTP(cfg ConfigCenterConfig, endpoint string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if token := strings.TrimSpace(cfg.Token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if namespace := strings.TrimSpace(cfg.Namespace); namespace != "" {
		req.Header.Set("X-HVC-Config-Namespace", namespace)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("config_center bootstrap http status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func resolveBootstrapConfigCenterFilePath(endpoint string, localConfigPath string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return endpoint
	}
	if filepath.IsAbs(endpoint) {
		return endpoint
	}
	baseDir := filepath.Dir(localConfigPath)
	if strings.TrimSpace(baseDir) == "" || baseDir == "." {
		return endpoint
	}
	return filepath.Join(baseDir, endpoint)
}
