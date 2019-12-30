package server

import "time"

// SimpleProfiler is the profiler which uses only the last elapsed time.
type SimpleProfiler struct {
	profiles map[string]time.Duration
}

// NewSimpleProfiler returns the new simple profiler.
func NewSimpleProfiler() *SimpleProfiler {
	return &SimpleProfiler{profiles: make(map[string]time.Duration)}
}

// Add adds the given data to the profiler.
func (p *SimpleProfiler) Add(filePath, function string, elapsedTime time.Duration) {
	p.profiles[p.key(filePath, function)] = elapsedTime
}

func (p *SimpleProfiler) key(filePath, function string) string {
	return filePath + "#" + function
}

// ExpectExecTime expects the exec time of the specified function.
func (p *SimpleProfiler) ExpectExecTime(filePath, function string) time.Duration {
	if d, ok := p.profiles[p.key(filePath, function)]; ok {
		return d
	}
	return 0
}
