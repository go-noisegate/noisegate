package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-noisegate/noisegate/common"
	"github.com/go-noisegate/noisegate/common/log"
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
	input.Path = filepath.Clean(input.Path)

	if err := s.updateChanges(input.Path, input.Ranges); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	if log.DebugLogEnabled() {
		log.Debugf("hint %s:%s\n", input.Path, common.RangesToQuery(input.Ranges))
	} else {
		log.Printf("hint %s\n", input.Path)
	}

	w.Write([]byte("accepted\n"))
}

func (s Server) updateChanges(inputPath string, ranges []common.Range) error {
	if !filepath.IsAbs(inputPath) {
		return errors.New("the path must be abs")
	}

	fi, err := os.Stat(inputPath)
	if os.IsNotExist(err) {
		return errors.New("the path not exist")
	}

	if fi.IsDir() {
		return errors.New("the path must be file")
	}

	if len(ranges) == 0 {
		return errors.New("the range is not specified")
	}

	pathDir := filepath.Dir(inputPath)
	baseName := filepath.Base(inputPath)
	for _, r := range ranges {
		s.changeManager.Add(pathDir, Change{baseName, r.Begin, r.End})
	}
	return nil
}

func (s Server) handleTest(w http.ResponseWriter, r *http.Request) {
	var input common.TestRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&input); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request body\n"))
		return
	}
	input.Path = filepath.Clean(input.Path)

	if err := s.validateTestPath(input.Path); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	log.Printf("test %s\n", input.Path)

	respWriter := newFlushWriter(w)
	job, err := NewJob(input.Path, input.Bypass, s.changeManager.Find(input.Path), input.GoTestOptions, respWriter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("failed to generate a new job: %v\n", err)
		fmt.Fprint(w, msg)
		log.Debug(msg)
		return
	}

	log.Debugf("start job #%d\n", job.ID)
	job.Run(context.Background())

	if job.Status == JobStatusSuccessful {
		s.changeManager.Delete(input.Path)
	}
	log.Debugf("finish job #%d\n", job.ID)
	log.Debugf("build + test time: %v\n", job.FinishedAt.Sub(job.CreatedAt))
}

func (s Server) validateTestPath(inputPath string) error {
	if !filepath.IsAbs(inputPath) {
		return errors.New("the path must be abs")
	}

	fi, err := os.Stat(inputPath)
	if os.IsNotExist(err) {
		return errors.New("the path not exist")
	}

	if !fi.IsDir() {
		return errors.New("the path must be directory")
	}
	return nil
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
