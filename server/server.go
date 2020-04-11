package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ks888/hornet/common"
	"github.com/ks888/hornet/common/log"
)

var sharedDir string

// SetUpSharedDir initializes the specified directory.
func SetUpSharedDir(dir string) {
	sharedDir = dir
	os.Mkdir(filepath.Join(sharedDir, "bin"), os.ModePerm)
	os.Mkdir(filepath.Join(sharedDir, "lib"), os.ModePerm)
	os.Mkdir(filepath.Join(sharedDir, "log"), os.ModePerm)
	os.Mkdir(filepath.Join(sharedDir, "log", "job"), os.ModePerm)
	os.Mkdir(filepath.Join(sharedDir, "src"), os.ModePerm)

	log.Debugf("shared dir: %s", sharedDir)
}

// HornetServer serves the APIs for the cli client.
type HornetServer struct {
	*http.Server
	jobManager    *JobManager
	changeManager ChangeManager
	depthLimit    int
}

// NewHornetServer returns the new hornet server.
// We can use only one server instance in the process even if the address is different.
func NewHornetServer(addr string) HornetServer {
	s := HornetServer{
		jobManager:    NewJobManager(),
		changeManager: NewChangeManager(),
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

// Shutdown shutdowns the http server.
func (s HornetServer) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
}

func (s HornetServer) handleHint(w http.ResponseWriter, r *http.Request) {
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

	var ranges string
	if log.DebugLogEnabled() {
		ranges = common.RangesToQuery(input.Ranges)
		log.Debugf("hint %s:%s\n", input.Path, ranges)
	} else {
		log.Printf("hint %s\n", input.Path)
	}

	if !fi.IsDir() {
		for _, r := range input.Ranges {
			s.changeManager.Add(filepath.Dir(input.Path), change{input.Path, r.Begin, r.End})
		}
	}
	w.Write([]byte("accepted\n"))
}

func (s HornetServer) handleTest(w http.ResponseWriter, r *http.Request) {
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
	pathDir := input.Path
	if !fi.IsDir() {
		pathDir = filepath.Dir(input.Path)
		for _, r := range input.Ranges {
			s.changeManager.Add(filepath.Dir(input.Path), change{input.Path, r.Begin, r.End})
		}
	}

	var ranges string
	if log.DebugLogEnabled() {
		ranges = common.RangesToQuery(input.Ranges)
		log.Debugf("test %s:%s\n", input.Path, ranges)
	} else {
		log.Printf("test %s\n", input.Path)
	}

	respWriter := newFlushWriter(w)
	job, err := NewJob(pathDir, s.changeManager.Pop(pathDir), input.BuildTags, respWriter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("failed to generate a new job: %v\n", err)
		fmt.Fprint(w, msg)
		log.Debug(msg)
		return
	}

	log.Debugf("start job %d\n", job.ID)
	if err := s.jobManager.StartJob(context.Background(), job); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("set up failed: %v\n", err)
		fmt.Fprint(w, msg)
		log.Debug(msg)
		return
	}

	s.WaitJob(w, job)
}

func (s HornetServer) WaitJob(w http.ResponseWriter, job *Job) {
	if err := s.jobManager.WaitJob(job.ID); err != nil {
		fmt.Fprintf(w, "set up failed: %v", err)
	}

	result := "FAIL"
	if job.Status == JobStatusSuccessful {
		result = "PASS"
	}

	fmt.Fprintf(w, "%s (%v)\n", result, job.ElapsedTestTime)
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
