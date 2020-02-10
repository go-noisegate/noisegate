package server

import (
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
	scheduler   taskSetScheduler
	profiler    *SimpleProfiler
	partitioner LPTPartitioner
	jobs        map[int64]*Job
	mtx         sync.Mutex // protect `jobs`
}

// NewJobManager returns the new job manager.
func NewJobManager() *JobManager {
	profiler := NewSimpleProfiler()
	// TODO: LPT is generally good, but maybe tests associated with the changed file or previously failed tests should be executed first.
	partitioner := NewLPTPartitioner(profiler)
	return &JobManager{
		profiler:    profiler,
		partitioner: partitioner,
		jobs:        make(map[int64]*Job),
	}
}

// NextTaskSet returns the runnable task set.
func (m *JobManager) NextTaskSet(groupName string, workerID int) (job *Job, taskSet *TaskSet, err error) {
	taskSet, err = m.scheduler.Next()
	if err != nil {
		return
	}
	// assumes this task set has at least one task
	job = taskSet.Tasks[0].Job

	taskSet.Start(groupName, workerID)
	return
}

// Partition partitions the job into the task sets.
func (m *JobManager) Partition(job *Job, numPartitions int) error {
	if numPartitions == 0 && len(job.Tasks) != 0 {
		return errors.New("the number of partitions is 0")
	}

	var affectedTasks, notAffectedTasks []*Task
	for _, t := range job.Tasks {
		if t.Affected {
			affectedTasks = append(affectedTasks, t)
		} else {
			notAffectedTasks = append(notAffectedTasks, t)
		}
	}

	if len(affectedTasks) > 0 {
		job.TaskSets = m.partitioner.Partition(affectedTasks, job.ID, numPartitions)
	}
	job.TaskSets = append(job.TaskSets, m.partitioner.Partition(notAffectedTasks, job.ID, numPartitions)...)
	return nil
}

// AddJob partitions the job into the task sets and adds them to the scheduler.
func (m *JobManager) AddJob(job *Job) {
	log.Debugf("add the %d task set(s)\n", len(job.TaskSets))
	for _, taskSet := range job.TaskSets {
		if len(taskSet.Tasks) == 0 {
			taskSet.Start("", 0)
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

	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.jobs[job.ID] = job
}

// ReportResult reports the result and updates the statuses.
func (m *JobManager) ReportResult(jobID int64, taskSetID int, successful bool) error {
	job, err := m.Find(jobID)
	if err != nil {
		return err
	}

	taskSet := job.TaskSets[taskSetID]
	rawProfiles := m.parseGoTestLog(taskSet.LogPath)
	for _, t := range taskSet.Tasks {
		p, ok := rawProfiles[t.TestFunction]
		if ok {
			m.profiler.Add(job.DirPath, t.TestFunction, p.elapsedTime)
			t.Finish(p.successful, p.elapsedTime)
		} else {
			log.Printf("failed to detect the result of %s. Assume it's same as the result of the task set: %v\n", t.TestFunction, successful)
			t.Finish(successful, 0)
		}
	}

	taskSet.Finish(successful)

	m.mtx.Lock()
	defer m.mtx.Unlock()
	if job.CanFinish() {
		job.Finish()
		delete(m.jobs, jobID)
	}
	return nil
}

// ReportResult reports the result and updates the statuses.
func (m *JobManager) Find(jobID int64) (*Job, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("failed to find the job %d", jobID)
	}
	return job, nil
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
