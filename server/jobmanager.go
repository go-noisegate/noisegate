package server

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
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
func (m *JobManager) StartJob(ctx context.Context, job *Job, numPartitions int) error {
	if err := m.partition(job, numPartitions); err != nil {
		return err
	}

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

		rawProfiles := m.parseGoTestLog(taskSet.LogPath)
		for _, t := range taskSet.Tasks {
			p, ok := rawProfiles[t.TestFunction]
			if ok {
				m.profiler.Add(job.DirPath, t.TestFunction, p.elapsedTime)
				t.Finish(p.successful, p.elapsedTime)
			} else {
				successful := taskSet.Status == TaskSetStatusSuccessful
				log.Printf("failed to detect the result of %s. Assume it's same as the result of the task set: %v\n", t.TestFunction, successful)
				t.Finish(successful, 0)
			}
		}
	}

	m.mtx.Lock()
	defer m.mtx.Unlock()
	job.Finish()
	delete(m.jobs, jobID)

	return nil
}

type rawProfile struct {
	testFuncName string
	successful   bool
	elapsedTime  time.Duration
}

var goTestLogRegexp = regexp.MustCompile(`(?m)^--- (PASS|FAIL): (.+) \(([0-9.]+s)\)$`)

func (m *JobManager) parseGoTestLog(logPath string) map[string]rawProfile {
	profiles := make(map[string]rawProfile)

	goTestLog, err := ioutil.ReadFile(logPath)
	if err != nil {
		log.Debugf("failed to read the log file: %v", err)
		return profiles
	}

	submatches := goTestLogRegexp.FindAllStringSubmatch(string(goTestLog), -1)
	for _, submatch := range submatches {
		successful := true
		if submatch[1] == "FAIL" {
			successful = false
		}
		funcName := submatch[2]
		d, err := time.ParseDuration(submatch[3])
		if err != nil {
			log.Printf("failed to parse go test log: %v", err)
			continue
		}

		profiles[funcName] = rawProfile{funcName, successful, d}
	}
	return profiles
}
