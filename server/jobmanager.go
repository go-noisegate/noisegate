package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ks888/hornet/common/log"
)

// JobManager manages the jobs.
type JobManager struct {
	profiler    *TaskProfiler
	partitioner LPTPartitioner
	jobs        map[int64]*Job
	mtx         sync.Mutex // protect `jobs`
}

// NewJobManager returns the new job manager.
func NewJobManager() *JobManager {
	profiler := NewTaskProfiler()
	return &JobManager{
		profiler:    profiler,
		partitioner: NewLPTPartitioner(profiler),
		jobs:        make(map[int64]*Job),
	}
}

// StartJob starts the job.
func (m *JobManager) StartJob(ctx context.Context, job *Job, numWorkers int, testResultWriter io.Writer) error {
	if err := m.partition(job, numWorkers); err != nil {
		return err
	}
	go m.testEventHandler(job, testResultWriter)

	if log.DebugLogEnabled() {
		mode := "sequential"
		if job.EnableParallel {
			mode = "parallel"
		}
		log.Debugf("starts %d task set(s) [%s]\n", len(job.TaskSets), mode)
		for _, taskSet := range job.TaskSets {
			var ts []string
			for _, t := range taskSet.Tasks {
				ts = append(ts, t.TestFunction)
			}
			log.Debugf("task set %d: [%v]\n", taskSet.ID, ts)
		}
	}
	job.Start(ctx)

	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func (m *JobManager) partition(job *Job, numWorkers int) error {
	if numWorkers == 0 && len(job.Tasks) != 0 {
		return errors.New("the number of partitions is 0")
	} else if !job.EnableParallel {
		job.TaskSets = m.partitioner.Partition(job.Tasks, job, 1)
		return nil
	}

	// TODO: partitioner should handle this.
	var importantTasks, notImportantTasks []*Task
	for _, t := range job.Tasks {
		if t.Important {
			importantTasks = append(importantTasks, t)
		} else {
			notImportantTasks = append(notImportantTasks, t)
		}
	}

	if len(importantTasks) > 0 {
		job.TaskSets = m.partitioner.Partition(importantTasks, job, numWorkers)
	}
	job.TaskSets = append(job.TaskSets, m.partitioner.Partition(notImportantTasks, job, numWorkers)...)
	return nil
}

func (m *JobManager) testEventHandler(job *Job, w io.Writer) {
	handler := newEventHandler(job, w)
	for {
		select {
		case <-job.jobFinishedCh:
			return
		case ev := <-job.testEventCh:
			handler.handle(ev)
		}
	}
}

// Find finds the specified job.
func (m *JobManager) Find(jobID int64) (*Job, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("failed to find the job %d", jobID)
	}
	return job, nil
}

// WaitJob waits the job finished.
func (m *JobManager) WaitJob(jobID int64) error {
	job, err := m.Find(jobID)
	if err != nil {
		return err
	}
	job.Wait()

	// TODO: wait the test result handler finished. The task status may not be updated yet.
	for _, t := range job.Tasks {
		if t.Status != TaskStatusSuccessful && t.Status != TaskStatusFailed {
			log.Debugf("the task status is not 'done' status: %s\n", t.TestFunction)
			continue
		}
		m.profiler.Add(job.DirPath, t.TestFunction, t.ElapsedTime)
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()
	delete(m.jobs, jobID)

	return nil
}

var elapsedTimeRegexp = regexp.MustCompile(`(?m)^--- (PASS|FAIL|SKIP|BENCH): (.+) \(([0-9.]+s)\)$`)

type eventHandler struct {
	job            *Job
	tasks          map[string]*Task
	importantTests map[string]struct{}
	runningTests   map[string][]string
	buffer         []TestResult
	w              io.Writer
}

func newEventHandler(job *Job, w io.Writer) *eventHandler {
	tasks := make(map[string]*Task)
	importantTests := make(map[string]struct{})
	for _, t := range job.Tasks {
		tasks[t.TestFunction] = t
		if t.Important {
			importantTests[t.TestFunction] = struct{}{}
		}
	}
	runningTests := make(map[string][]string)
	return &eventHandler{job: job, tasks: tasks, importantTests: importantTests, runningTests: runningTests, w: w}
}

func (h *eventHandler) handleResult(result TestResult) {
	task, ok := h.tasks[result.TestName]
	if !ok {
		return
	}

	output := strings.Join(result.Output, "")
	h.w.Write([]byte(output))

	elapsedTime := time.Duration(-1)
	submatch := elapsedTimeRegexp.FindStringSubmatch(output)
	if len(submatch) > 1 {
		if d, err := time.ParseDuration(submatch[3]); err == nil {
			elapsedTime = d
		}
	}

	task.Finish(result.Successful, elapsedTime)
}

func (h *eventHandler) handleResultWithBuffer(result TestResult) {
	task, ok := h.tasks[result.TestName]
	if !ok {
		if result.TestName == "" && !h.job.EnableParallel {
			h.job.ElapsedTestTime = result.ElapsedTime
		}
		return
	}

	if len(h.importantTests) == 0 {
		h.handleResult(result)
		return
	}

	if !task.Important {
		// buffer not-important test function until all the important tests are done.
		h.buffer = append(h.buffer, result)
		return
	}

	// it's important test function. Handle here.
	h.handleResult(result)
	delete(h.importantTests, task.TestFunction)

	if len(h.importantTests) == 0 {
		h.w.Write([]byte("\nRun other tests:\n"))
		if h.job.EnableParallel {
			log.Debugf("time to execute important tests: %v\n", time.Now().Sub(h.job.StartedAt))
		} else {
			// On sequential exec, the time which the build has finished is unknown
			log.Debugf("time to execute important tests: (unknown)\n")
		}

		// Now all the important test functions are done. Release the buffer.
		for _, r := range h.buffer {
			h.handleResult(r)
		}
	}
}

func (h *eventHandler) handle(ev TestEvent) {
	if ev.Action == "unknown" {
		h.w.Write([]byte(ev.Output))
		return
	}

	chunks := strings.SplitN(ev.Test, "/", 2)
	if len(chunks) == 2 {
		// merge the output to the parent test
		if ev.Action == "output" {
			parentTest := chunks[0]
			h.runningTests[parentTest] = append(h.runningTests[parentTest], ev.Output)
		}
		return
	}

	switch ev.Action {
	case "run":
		h.runningTests[ev.Test] = []string{}
	case "pause", "cont":
		// do nothing
	case "output":
		h.runningTests[ev.Test] = append(h.runningTests[ev.Test], ev.Output)
	case "pass", "fail", "skip", "bench":
		elapsedTime := time.Duration(ev.Elapsed * 1000 * 1000 * 1000) // Elapsed is float64 in second
		res := TestResult{TestName: ev.Test, Successful: ev.Action != "fail", ElapsedTime: elapsedTime, Output: h.runningTests[ev.Test]}
		h.handleResultWithBuffer(res)
		delete(h.runningTests, ev.Test)
	}
}
