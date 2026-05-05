package planner

import "hvc/internal/model"

// BuildPlan 构造执行计划。
func BuildPlan(job model.TranscodeJob) Pipeline {
	return Pipeline{
		HardwareEncode: true,
		HardwareDecode: true,
	}
}
