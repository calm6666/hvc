package hoststats

// cpuSampler 抽象跨平台 CPU 使用率采样器。
type cpuSampler interface {
	Percent() int
}
