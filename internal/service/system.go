package service

import "hvc/internal/model"

// SystemService 表示系统服务。
type SystemService struct {
	name string
	mode string
}

// NewSystemService 创建系统服务。
func NewSystemService(name string, mode string) *SystemService {
	return &SystemService{name: name, mode: mode}
}

// Health 返回健康状态。
func (s *SystemService) Health() model.HealthData {
	return model.HealthData{
		Name:   s.name,
		Status: "ok",
		Mode:   s.mode,
	}
}
