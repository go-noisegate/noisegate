package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ks888/noisegate/common"
	"github.com/ks888/noisegate/common/log"
)

// Server serves the APIs for the cli client.
type Server struct {
	*http.Server
	changeManager changeManager
}

// NewServer returns a new server.
// We can use only one server instance in the process even if the address is different.
func NewServer(addr string) Server {
	s := Server{
		changeManager: newChangeManager(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(common.TestPath, s.handleTest)
	mux.HandleFunc(common.HintPath, s.handleHint)
	s.Server = &http.Server{
		Handler: mux,
		Addr:    addr,
	}
	return s
}

// Shutdown shutdowns the server.
func (s Server) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
}

func (s Server) handleHint(w http.ResponseWriter, r *http.Request) {
	var input common.HintRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&input); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !filepath.IsAbs(input.Path) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("the path must be abs\n"))
		return
	}

	fi, err := os.Stat(input.Path)
	if os.IsNotExist(err) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("the specified path not found\n"))
		return
	}

	if log.DebugLogEnabled() {
		log.Debugf("hint %s:%s\n", input.Path, common.RangesToQuery(input.Ranges))
	} else {
		log.Printf("hint %s\n", input.Path)
	}

	if fi.IsDir() {
		pathDir := input.Path
		baseName := ""
		s.changeManager.Add(pathDir, Change{baseName, 0, 0})
	} else {
		pathDir := filepath.Dir(input.Path)
		baseName := filepath.Base(input.Path)
		for _, r := range input.Ranges {
			s.changeManager.Add(pathDir, Change{baseName, r.Begin, r.End})
		}
	}

	w.Write([]byte("accepted\n"))
}

func (s Server) handleTest(w http.ResponseWriter, r *http.Request) {
	var input common.TestRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&input); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request body\n"))
		return
	}

	if !filepath.IsAbs(input.Path) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("the path must be abs\n"))
		return
	}

	fi, err := os.Stat(input.Path)
	if os.IsNotExist(err) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("the specified path not found\n"))
		return
	}
	var pathDir string
	if fi.IsDir() {
		pathDir = input.Path
		baseName := ""
		s.changeManager.Add(pathDir, Change{baseName, 0, 0})
	} else {
		pathDir = filepath.Dir(input.Path)
		baseName := filepath.Base(input.Path)
		for _, r := range input.Ranges {
			s.changeManager.Add(pathDir, Change{baseName, r.Begin, r.End})
		}
	}

	if log.DebugLogEnabled() {
		log.Debugf("test %s:%s\n", input.Path, common.RangesToQuery(input.Ranges))
	} else {
		log.Printf("test %s\n", input.Path)
	}

	respWriter := newFlushWriter(w)
	job, err := NewJob(pathDir, s.changeManager.Find(pathDir), input.GoTestOptions, respWriter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("failed to generate a new job: %v\n", err)
		fmt.Fprint(w, msg)
		log.Debug(msg)
		return
	}

	log.Debugf("start job %d\n", job.ID)
	respWriter.Write([]byte(fmt.Sprintf("Changed: [%s]\n", strings.Join(job.changedIdentityNames(), ", "))))

	job.Run(context.Background())

	result := "FAIL"
	if job.Status == JobStatusSuccessful {
		result = "PASS"
		s.changeManager.Delete(pathDir)
	}

	fmt.Fprintf(job.writer, "%s (%v)\n", result, job.ElapsedTestTime)
	log.Debugf("time to execute all the tests: %v\n", job.ElapsedTestTime)
	log.Debugf("total time: %v\n", job.FinishedAt.Sub(job.CreatedAt))
}

type flushWriter struct {
	flusher http.Flusher
	writer  io.Writer
}

func newFlushWriter(w http.ResponseWriter) flushWriter {
	return flushWriter{
		flusher: w.(http.Flusher),
		writer:  w,
	}
}

func (w flushWriter) Write(p []byte) (int, error) {
	defer w.flusher.Flush()
	return w.writer.Write(p)
}
