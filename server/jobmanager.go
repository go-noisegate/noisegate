package server

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"github.com/ks888/hornet/common/log"
)

// JobManager manages the jobs.
type JobManager struct {
	scheduler   taskSetScheduler
	profiler    *SimpleProfiler
	partitioner LPTPartitioner
	jobs        map[int64]*Job
}

// NewJobManager returns the new job manager.
func NewJobManager() *JobManager {
	profiler := NewSimpleProfiler()
	partitioner := NewLPTPartitioner(profiler)
	return &JobManager{
		profiler:    profiler,
		partitioner: partitioner,
		jobs:        make(map[int64]*Job),
	}
}

// NextTaskSet returns the runnable task set.
func (m *JobManager) NextTaskSet(groupName string, workerID int) (job *Job, taskSet *TaskSet, err error) {
	for {
		taskSet, err = m.scheduler.Next()
		if err != nil {
			return
		}

		if len(taskSet.Tasks) != 0 {
			job = taskSet.Tasks[0].Job
			break
		}
		log.Printf("the task set %d has no tasks", taskSet.ID)
	}

	taskSet.Start(groupName, workerID)
	return
}

// AddJob partitions the job into the task sets and adds them to the scheduler.
func (m *JobManager) AddJob(job *Job) {
	job.TaskSets = m.partitioner.Partition(job, 1)
	for _, taskSet := range job.TaskSets {
		if len(taskSet.Tasks) == 0 {
			taskSet.Finish(true)
			continue
		}

		if err := m.scheduler.Add(taskSet, job.DependencyDepth); err != nil {
			log.Printf("failed to add the new task set %v: %v", taskSet, err)
		}
	}

	if job.CanFinish() {
		// if all task sets have no tasks, we can finish the job here.
		job.Finish()
		return
	}
	m.jobs[job.ID] = job
}

// ReportResult reports the result and updates the statuses.
func (m *JobManager) ReportResult(jobID int64, taskSetID int, successful bool) error {
	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("failed to find the job %d", jobID)
	}

	taskSet := job.TaskSets[taskSetID]
	rawProfiles := m.parseGoTestLog(filepath.Join(sharedDir, taskSet.LogPath))
	for _, t := range taskSet.Tasks {
		p, ok := rawProfiles[t.TestFunction]
		if ok {
			m.profiler.Add(job.DirPath, t.TestFunction, p.elapsedTime)
			t.Finish(p.successful, p.elapsedTime)
		} else {
			log.Printf("failed to detect the result of %s. Consider it's same as the result of the task set (%v)\n", t.TestFunction, successful)
			t.Finish(successful, 0)
		}
	}

	taskSet.Finish(successful)

	if job.CanFinish() {
		job.Finish()
		delete(m.jobs, jobID)
	}
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
		log.Debugf("failed to read the log file %s: %v", logPath, err)
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