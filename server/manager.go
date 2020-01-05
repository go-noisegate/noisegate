package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

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

	taskSet.Started(workerID)
	return
}

// AddJob partitions the job and adds them to the scheduler.
func (m *Manager) AddJob(job *Job, depth int) {
	job.TaskSets = m.partitioner.Partition(job.Tasks, 1)
	for _, taskSet := range job.TaskSets {
		// TODO: finish the empty task set immediately
		if err := m.scheduler.Add(taskSet, depth); err != nil {
			log.Printf("failed to add the new task set %v: %v", taskSet, err)
		}
	}
	m.jobs[job.ID] = job
}

// AddJob partitions the job and adds them to the scheduler.
func (m *Manager) ReportResult(jobID int64, taskSetID int, successful bool, log []byte) error {
	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("failed to find the job %d", jobID)
	}

	taskSet := job.TaskSets[taskSetID]
	taskSet.Finish(successful, log)

	// update tasks

	// update profiler

	if job.CanFinish() {
		job.Finish()
		delete(m.jobs, jobID)
	}

	return nil
}

const (
	// the internal APIs for the workers and no need to be RESTful so far.
	nextTaskSetPath = "/next"
	reportPath      = "/report"
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
}

type reportRequest struct {
	JobID      int64  `json:"job_id"`
	TaskSetID  int    `json:"task_set_id"`
	Successful bool   `json:"successful"`
	Log        []byte `json:"log"`
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
		JobID:          job.ID,
		TaskSetID:      taskSet.ID,
		DirPath:        job.DirPath,
		TestBinaryPath: job.TestBinaryPath,
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
