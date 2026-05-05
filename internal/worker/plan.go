package worker

import "hvc/internal/config"

// Plan 表示执行计划。
type Plan struct {
	UseHardwareDecode    bool
	UseHardwareEncode    bool
	UseHardwareWatermark bool
	RequiresSoftDecode   bool
	Reason               string
}

// BuildPlan 生成执行计划。
func BuildPlan(cfg config.RuntimeConfig, sourceSupportsHardwareDecode bool, watermarkEnabled bool, cpuPercent int) (Plan, bool) {
	if cfg.Scheduler.RequireHardwareEncode == false {
		return Plan{}, false
	}
	if sourceSupportsHardwareDecode {
		return Plan{
			UseHardwareDecode:    true,
			UseHardwareEncode:    true,
			UseHardwareWatermark: watermarkEnabled,
			RequiresSoftDecode:   false,
			Reason:               "硬解硬编执行",
		}, true
	}
	if !cfg.Scheduler.AllowSoftwareDecodeFallback {
		return Plan{}, false
	}
	if cpuPercent >= cfg.Scheduler.SoftDecodeCPULimitPercent {
		return Plan{}, false
	}
	return Plan{
		UseHardwareDecode:    false,
		UseHardwareEncode:    true,
		UseHardwareWatermark: watermarkEnabled,
		RequiresSoftDecode:   true,
		Reason:               "软解硬编执行",
	}, true
}
