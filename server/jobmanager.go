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
	profiler    *SimpleProfiler
	partitioner LPTPartitioner
	jobs        map[int64]*Job
	mtx         sync.Mutex // protect `jobs`
}

// NewJobManager returns the new job manager.
func NewJobManager() *JobManager {
	profiler := NewSimpleProfiler()
	return &JobManager{
		profiler:    profiler,
		partitioner: NewLPTPartitioner(profiler),
		jobs:        make(map[int64]*Job),
	}
}

// StartJob starts the job.
func (m *JobManager) StartJob(ctx context.Context, job *Job, numPartitions int, testResultWriter io.Writer) error {
	if err := m.partition(job, numPartitions); err != nil {
		return err
	}
	go m.testResultHandler(job, testResultWriter)

	log.Debugf("starts %d task set(s)\n", len(job.TaskSets))
	job.Start(ctx)

	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func (m *JobManager) partition(job *Job, numPartitions int) error {
	if numPartitions == 0 && len(job.Tasks) != 0 {
		return errors.New("the number of partitions is 0")
	}

	// TODO: partitioner should handle this.
	var affectedTasks, notAffectedTasks []*Task
	for _, t := range job.Tasks {
		if t.Affected {
			affectedTasks = append(affectedTasks, t)
		} else {
			notAffectedTasks = append(notAffectedTasks, t)
		}
	}

	if len(affectedTasks) > 0 {
		job.TaskSets = m.partitioner.Partition(affectedTasks, job, numPartitions)
	}
	job.TaskSets = append(job.TaskSets, m.partitioner.Partition(notAffectedTasks, job, numPartitions)...)
	return nil
}

var elapsedTimeRegexp = regexp.MustCompile(`(?m)^--- (PASS|FAIL|SKIP|BENCH): (.+) \(([0-9.]+s)\)$`)

func (m *JobManager) testResultHandler(job *Job, w io.Writer) {
	tasks := make(map[string]*Task)
	affected := make(map[string]struct{})
	var affectedTestnames []string
	for _, t := range job.Tasks {
		tasks[t.TestFunction] = t
		if t.Affected {
			affected[t.TestFunction] = struct{}{}
			affectedTestnames = append(affectedTestnames, t.TestFunction)
		}
	}
	if len(affectedTestnames) != 0 {
		w.Write([]byte("### Important tests: " + strings.Join(affectedTestnames, "|") + "\n"))
	} else {
		w.Write([]byte("### No important tests\n"))
	}

	handle := func(task *Task, testResult TestResult) {
		output := strings.Join(testResult.Output, "")
		w.Write([]byte(output))

		elapsedTime := time.Duration(-1)
		submatch := elapsedTimeRegexp.FindStringSubmatch(output)
		if len(submatch) > 1 {
			if d, err := time.ParseDuration(submatch[3]); err == nil {
				elapsedTime = d
				m.profiler.Add(job.DirPath, testResult.TestName, elapsedTime)
			}
		}

		task.Finish(testResult.Successful, elapsedTime)
	}

	var resultBuffer []TestResult
	for {
		select {
		case <-job.finishedCh:
			return
		case testResult := <-job.testResultCh:
			task, ok := tasks[testResult.TestName]
			if !ok {
				break
			}

			if len(affected) == 0 {
				handle(task, testResult)
				break
			}

			if !task.Affected {
				// buffer not-affected test function until all the affected tests are done.
				resultBuffer = append(resultBuffer, testResult)
				break
			}

			handle(task, testResult)
			delete(affected, task.TestFunction)

			if len(affected) == 0 {
				w.Write([]byte("### Important tests are done. Now running other tests\n"))

				// Now all the affected test functions are done. Release the buffer.
				for _, r := range resultBuffer {
					handle(tasks[r.TestName], r)
				}
			}
		}
	}
	fmt.Printf("done\n")
}

func (m *JobManager) handleOneResult(dirPath string, w io.Writer) {
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

	m.mtx.Lock()
	defer m.mtx.Unlock()
	delete(m.jobs, jobID)

	return nil
}
