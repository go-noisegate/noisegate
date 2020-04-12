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

// Server serves the APIs for the cli client.
type Server struct {
	*http.Server
	changeManager ChangeManager
	depthLimit    int
}

// NewServer returns a new server.
// We can use only one server instance in the process even if the address is different.
func NewServer(addr string) Server {
	s := Server{
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
	job, err := NewJob(pathDir, s.changeManager.Pop(pathDir), input.BuildTags, input.Bypass, respWriter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("failed to generate a new job: %v\n", err)
		fmt.Fprint(w, msg)
		log.Debug(msg)
		return
	}

	log.Debugf("start job %d\n", job.ID)
	respWriter.Write([]byte(fmt.Sprintf("Changed: [%s]\n", strings.Join(job.ChangedIdentityNames(), ", "))))

	ctx := context.Background()
	job.Start(ctx)

	s.WaitJob(job)
}

func (s Server) WaitJob(job *Job) {
	job.Wait()

	result := "FAIL"
	if job.Status == JobStatusSuccessful {
		result = "PASS"
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
