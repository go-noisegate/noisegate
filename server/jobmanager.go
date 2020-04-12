package server

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ks888/noisegate/common/log"
)

// JobManager manages the jobs.
type JobManager struct {
	jobs map[int64]*Job
	mtx  sync.Mutex // protect `jobs`
}

// NewJobManager returns the new job manager.
func NewJobManager() *JobManager {
	return &JobManager{
		jobs: make(map[int64]*Job),
	}
}

// StartJob starts the job.
func (m *JobManager) StartJob(ctx context.Context, job *Job) error {
	ts := NewTaskSet(0, job)
	for _, t := range job.Tasks {
		if t.Important {
			ts.Tasks = append(ts.Tasks, t)
		}
	}
	job.TaskSets = []*TaskSet{ts}

	if log.DebugLogEnabled() {
		log.Debugf("starts %d task set(s)\n", len(job.TaskSets))
		for _, taskSet := range job.TaskSets {
			var ts []string
			for _, t := range taskSet.Tasks {
				ts = append(ts, t.TestFunction)
			}
			log.Debugf("task set %d: [%v]\n", taskSet.ID, ts)
		}
	}

	job.writer.Write([]byte(fmt.Sprintf("Changed: [%s]\n", strings.Join(job.ChangedIdentityNames(), ", "))))
	job.Start(ctx)

	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.jobs[job.ID] = job
	return nil
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
