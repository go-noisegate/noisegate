package server

import (
	"encoding/json"
	"net/http"
)

const (
	// the internal APIs for the workers and no need to be RESTful so far.
	nextTaskSetPath = "/next"
)

type nextTaskSetResponse struct {
}

// Manager manages the workers.
type Manager struct {
	serverForWorkers *http.Server
	scheduler        taskSetScheduler
}

type Worker struct{}

// NewManager returns the new manager.
func NewManager(addr string) Manager {
	manager := Manager{}

	mux := http.NewServeMux()
	mux.HandleFunc(nextTaskSetPath, manager.handleNextTaskSet)

	manager.serverForWorkers = &http.Server{
		Handler: mux,
		Addr:    addr,
	}
	return manager
}

func (m Manager) handleNextTaskSet(w http.ResponseWriter, r *http.Request) {
	taskSet, _ := m.scheduler.Next()

	enc := json.NewEncoder(w)
	if err := enc.Encode(&taskSet); err != nil {
		// TODO
	}
}
