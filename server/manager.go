package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

	"github.com/ks888/hornet/common/log"
)

// Manager manages the workers.
type Manager struct {
	scheduler   taskSetScheduler
	profiler    *SimpleProfiler
	partitioner LPTPartitioner
	jobs        map[int64]*Job
}

type Worker struct{}

// NewManager returns the new manager.
func NewManager() *Manager {
	profiler := NewSimpleProfiler()
	partitioner := NewLPTPartitioner(profiler)
	return &Manager{
		profiler:    profiler,
		partitioner: partitioner,
		jobs:        make(map[int64]*Job),
	}
}

// NextTaskSet returns the runnable task set.
func (m *Manager) NextTaskSet(workerID int64) (job *Job, taskSet *TaskSet, err error) {
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

	taskSet.Start(workerID)
	return
}

// AddJob partitions the job into the task sets and adds them to the scheduler.
func (m *Manager) AddJob(job *Job) {
	job.TaskSets = m.partitioner.Partition(job.Tasks, 1)
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
func (m *Manager) ReportResult(jobID int64, taskSetID int, successful bool) error {
	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("failed to find the job %d", jobID)
	}

	taskSet := job.TaskSets[taskSetID]
	rawProfiles := m.parseGoTestLog(taskSet.LogPath)
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

func (m *Manager) parseGoTestLog(logPath string) map[string]rawProfile {
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

const (
	// the internal APIs for the workers and no need to be RESTful so far.
	nextTaskSetPath  = "/next"
	reportResultPath = "/report"
)

type nextTaskSetRequest struct {
	WorkerID int64 `json:"worker_id"`
}

type nextTaskSetResponse struct {
	JobID         int64    `json:"job_id"`
	TaskSetID     int      `json:"task_set_id"`
	TestFunctions []string `json:"test_functions"`
	// The abs path in the manager fs.
	DirPath string `json:"dir_path"`
	// The path from the NFS root
	TestBinaryPath string `json:"test_binary_path"`
	// The path from the NFS root
	RepoArchivePath string `json:"repo_archive_path"`
}

type reportResultRequest struct {
	JobID      int64 `json:"job_id"`
	TaskSetID  int   `json:"task_set_id"`
	Successful bool  `json:"successful"`
}

// ManagerServer serves some of the manager's function as the APIs so that the workers can use them.
type ManagerServer struct {
	*http.Server
	manager *Manager
}

// NewManagerServer returns the new manager server.
func NewManagerServer(addr string, manager *Manager) ManagerServer {
	s := ManagerServer{manager: manager}

	mux := http.NewServeMux()
	mux.HandleFunc(nextTaskSetPath, s.handleNextTaskSet)
	s.Server = &http.Server{
		Handler: mux,
		Addr:    addr,
	}
	return s
}

// handleNextTaskSet handles the next task set request.
func (s ManagerServer) handleNextTaskSet(w http.ResponseWriter, r *http.Request) {
	var req nextTaskSetRequest
	rawBody, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal(rawBody, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	job, taskSet, err := s.manager.NextTaskSet(req.WorkerID)
	if err != nil {
		if err == errNoTaskSet {
			w.WriteHeader(http.StatusNotFound)
		} else {
			log.Printf("failed to get the next task set: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	resp := &nextTaskSetResponse{
		JobID:           job.ID,
		TaskSetID:       taskSet.ID,
		DirPath:         job.DirPath,
		TestBinaryPath:  job.TestBinaryPath,
		RepoArchivePath: job.RepoArchivePath,
	}
	for _, t := range taskSet.Tasks {
		resp.TestFunctions = append(resp.TestFunctions, t.TestFunction)
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(&resp); err != nil {
		log.Printf("failed to encode the response: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// handleReportResult handles the report result request.
func (s ManagerServer) handleReportResult(w http.ResponseWriter, r *http.Request) {
	var req reportResultRequest
	rawBody, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal(rawBody, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.manager.ReportResult(req.JobID, req.TaskSetID, req.Successful); err != nil {
		log.Printf("failed to report the result: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
