//go:build !linux && !windows && !darwin

package hoststats

type noopCPUSampler struct{}

func newCPUSampler() cpuSampler {
	return noopCPUSampler{}
}

func (noopCPUSampler) Percent() int {
	return 0
}
