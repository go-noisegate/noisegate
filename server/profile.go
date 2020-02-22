package server

import (
	"sync"
	"time"
)

// JobProfiler is the profiler which maintains the last elapsed time.
type JobProfiler struct {
	profiles map[string]time.Duration
	mtx      sync.Mutex
}

// NewJobProfiler returns the new task profiler.
func NewJobProfiler() *JobProfiler {
	return &JobProfiler{profiles: make(map[string]time.Duration)}
}

// Add adds the given data to the profiler.
func (p *JobProfiler) Add(filePath string, elapsedTime time.Duration) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	p.profiles[filePath] = elapsedTime
}

// LastElapsedTime returns the elapsed time of the last job.
func (p *JobProfiler) LastElapsedTime(filePath string) (time.Duration, bool) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if d, ok := p.profiles[filePath]; ok {
		return d, true
	}
	return 0, false
}

// TaskProfiler is the simple task profiler which expects the exec time based on the last elapsed time.
type TaskProfiler struct {
	profiles map[string]time.Duration
	mtx      sync.Mutex
}

// NewTaskProfiler returns the new task profiler.
func NewTaskProfiler() *TaskProfiler {
	return &TaskProfiler{profiles: make(map[string]time.Duration)}
}

// Add adds the given data to the profiler.
func (p *TaskProfiler) Add(filePath, function string, elapsedTime time.Duration) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	p.profiles[p.key(filePath, function)] = elapsedTime
}

func (p *TaskProfiler) key(filePath, function string) string {
	return filePath + "#" + function
}

// ExpectExecTime expects the exec time of the specified function.
func (p *TaskProfiler) ExpectExecTime(filePath, function string) time.Duration {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if d, ok := p.profiles[p.key(filePath, function)]; ok {
		return d
	}
	return 0
}
