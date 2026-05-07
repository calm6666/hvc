package gpu

import (
	"bufio"
	"bytes"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	ffmpeg "hvc/internal/infra/ffmpeg/process"
	"hvc/internal/model"
)

// ProbeResult 表示一次本机 GPU 能力探测结果。
//
// 这个结果是 internal/infra/gpu 包内部的统一中间结构，目的是把平台差异先收敛在探测层，
// 然后再由 ToGPUCapabilities 转成调度层和指标上报层都能消费的 model.GPUCapability。
//
// Current project guidance:
// - 这是运行期能力快照，不属于启动配置，也不属于动态业务配置；
// - 这里只表达“当前机器能做什么”，不表达“当前策略允不允许这样做”；
// - 策略阈值和是否启用仍由 DynamicRuntimeConfig 决定。
type ProbeResult struct {
	GPUDeviceID      uint64
	GPUUUID          string
	GPUIndex         int
	EncodeCodecs     []string
	DecodeCodecs     []string
	ExecutionHWTypes []string
	MaxSessions      int
	SupportsFilter   bool
}

// Probe 返回当前进程所在机器可观测到的 GPU 能力集合。
//
// 这个入口会优先走平台实现；如果平台专用探测拿不到结果，则回退到基于 ffmpeg 能力输出的通用探测。
// 这样可以保证：
// 1. macOS / Linux / Windows 至少都有统一入口；
// 2. 即使用户机器上没有额外管理工具，也能从 ffmpeg 的 hwaccels / encoders / decoders 中提取最小能力集合；
// 3. 不把平台分支扩散到调度、Worker、HTTP 接口等上层模块。
func Probe() []ProbeResult {
	results := probePlatform()
	if len(results) > 0 {
		return normalize(results)
	}
	return normalize(probeFallback())
}

// ToGPUCapabilities 把探测层结果转换成调度与指标上报统一使用的模型对象。
func ToGPUCapabilities(results []ProbeResult) []model.GPUCapability {
	items := make([]model.GPUCapability, 0, len(results))
	for _, result := range normalize(results) {
		items = append(items, model.GPUCapability{
			GPUDeviceID:      result.GPUDeviceID,
			GPUUUID:          result.GPUUUID,
			GPUIndex:         result.GPUIndex,
			EncodeCodecs:     append([]string(nil), result.EncodeCodecs...),
			DecodeCodecs:     append([]string(nil), result.DecodeCodecs...),
			ExecutionHWTypes: append([]string(nil), result.ExecutionHWTypes...),
			MaxSessions:      result.MaxSessions,
			SupportsFilter:   result.SupportsFilter,
		})
	}
	return items
}

// normalize 负责对探测结果做稳定化整理。
//
// 这里统一做三件事：
// 1. 去重 codec / hw 类型；
// 2. 按字典序排序，保证测试断言稳定；
// 3. 为缺失 UUID 的设备生成稳定占位值，避免上层逻辑不得不判断空字符串。
func normalize(results []ProbeResult) []ProbeResult {
	items := make([]ProbeResult, 0, len(results))
	for _, result := range results {
		result.EncodeCodecs = uniqueSorted(result.EncodeCodecs)
		result.DecodeCodecs = uniqueSorted(result.DecodeCodecs)
		result.ExecutionHWTypes = uniqueSorted(result.ExecutionHWTypes)
		if strings.TrimSpace(result.GPUUUID) == "" {
			result.GPUUUID = fallbackGPUUUID(result.GPUIndex, result.ExecutionHWTypes)
		}
		items = append(items, result)
	}
	return items
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}

func fallbackGPUUUID(index int, hwTypes []string) string {
	if len(hwTypes) == 0 {
		return "software-0"
	}
	return hwTypes[0] + "-" + strconv.Itoa(index)
}

// commandOutput 返回命令输出；命令不可用或执行失败时返回空字节切片。
//
// GPU 探测必须尽量“尽力而为”，不能因为某台机器上缺一个辅助命令就让整个 Worker 启动失败。
func commandOutput(name string, args ...string) []byte {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil
	}
	cmd := exec.Command(path, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	return output
}

// ffmpegCapabilityOutput 读取 ffmpeg 的能力输出。
func ffmpegCapabilityOutput(args ...string) []byte {
	path := ffmpeg.FindFFmpeg()
	cmd := exec.Command(path, args...)
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return nil
	}
	return output
}

func scanLines(output []byte) []string {
	if len(output) == 0 {
		return nil
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	items := make([]string, 0, 16)
	for scanner.Scan() {
		items = append(items, strings.TrimSpace(scanner.Text()))
	}
	return items
}

func parseIndex(value string) int {
	index, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return index
}
