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
	for _, taskSet := range job.TaskSets {
		w := NewWorker(ctx, job, taskSet)
		if err := w.Start(); err != nil {
			log.Printf("failed to start the worker: %v", err)
			continue
		}
		taskSet.Start(w)
	}

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
		job.TaskSets = m.partitioner.Partition(affectedTasks, job, 0, numPartitions)
	}
	job.TaskSets = append(job.TaskSets, m.partitioner.Partition(notAffectedTasks, job, len(job.TaskSets), numPartitions)...)
	return nil
}

var elapsedTimeRegexp = regexp.MustCompile(`(?m)^--- (PASS|FAIL|SKIP|BENCH): (.+) \(([0-9.]+s)\)$`)

func (m *JobManager) testResultHandler(job *Job, w io.Writer) {
	for {
		select {
		case <-job.finishedCh:
			break
		case testResult := <-job.testResultCh:
			output := strings.Join(testResult.Output, "")
			w.Write([]byte(output))

			elapsedTime := time.Duration(-1)
			submatch := elapsedTimeRegexp.FindStringSubmatch(output)
			if len(submatch) > 1 {
				if d, err := time.ParseDuration(submatch[1]); err == nil {
					elapsedTime = d
					m.profiler.Add(job.DirPath, testResult.TestName, elapsedTime)
				}
			}

			for _, t := range job.Tasks {
				if t.TestFunction == testResult.TestName {
					t.Finish(testResult.Successful, elapsedTime)
				}
			}
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

	for _, taskSet := range job.TaskSets {
		if taskSet.Worker == nil {
			taskSet.Finish(false)
			continue
		}
		successful, _ := taskSet.Worker.Wait()
		taskSet.Finish(successful)
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()
	job.Finish()
	delete(m.jobs, jobID)

	return nil
}
