package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/ks888/hornet/common/log"
)

const (
	// the internal APIs for the workers and no need to be RESTful so far.
	nextTaskSetPath = "/next"
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

// Manager manages the workers.
type Manager struct {
	serverForWorkers *http.Server
	scheduler        taskSetScheduler
	jobs             map[int64]*Job
}

type Worker struct{}

// NewManager returns the new manager.
func NewManager(addr string) Manager {
	manager := Manager{jobs: make(map[int64]*Job)}

	mux := http.NewServeMux()
	mux.HandleFunc(nextTaskSetPath, manager.handleNextTaskSet)

	manager.serverForWorkers = &http.Server{
		Handler: mux,
		Addr:    addr,
	}
	return manager
}

// NextTaskSet returns the runnable task set.
func (m Manager) NextTaskSet(workerID int64) (job *Job, taskSet *TaskSet, err error) {
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

func (m Manager) handleNextTaskSet(w http.ResponseWriter, r *http.Request) {
	var req nextTaskSetRequest
	rawBody, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal(rawBody, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	job, taskSet, err := m.NextTaskSet(req.WorkerID)
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
