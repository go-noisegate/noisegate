package server

import (
	"encoding/json"
	"net/http"

	"github.com/ks888/hornet/common"
	"github.com/ks888/hornet/common/log"
)

var sharedDir string

// HornetServer serves the APIs for the cli client.
type HornetServer struct {
	*http.Server
	manager *Manager
}

// NewHornetServer returns the new hornet server.
// We can use only one server instance in the process even if the address is different.
func NewHornetServer(addr, dir string, manager *Manager) HornetServer {
	sharedDir = dir

	s := HornetServer{manager: manager}

	mux := http.NewServeMux()
	mux.HandleFunc(common.TestPath, s.handleTest)
	s.Server = &http.Server{
		Handler: mux,
		Addr:    addr,
	}
	return s
}

func (s HornetServer) handleTest(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("http.Flusher is not implemented")
	}

	var input common.TestRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&input); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	job, err := NewJob("", input.Path, 0)
	if err != nil {
		log.Printf("failed to generate a new job: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	s.manager.AddJob(job)

	flusher = flusher
	// Note that the data is not flushed if \n is not appended.
	// fmt.Fprintf(w, "test %s...\n", input.Path)
	// flusher.Flush()

	job.WaitFinished()

	if job.Status == JobStatusSuccessful {
		w.Write([]byte("successful\n"))
	} else {
		w.Write([]byte("failed\n"))
	}
}
