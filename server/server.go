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
	"time"

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
	jobManager        *JobManager
	packageManager    *PackageManager
	jobProfiler       *JobProfiler
	defaultNumWorkers int
	depthLimit        int
}

// NewHornetServer returns the new hornet server.
// We can use only one server instance in the process even if the address is different.
func NewHornetServer(addr string, defaultNumWorkers int) HornetServer {
	s := HornetServer{
		jobManager:        NewJobManager(),
		packageManager:    NewPackageManager(),
		defaultNumWorkers: defaultNumWorkers,
		jobProfiler:       NewJobProfiler(),
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

	if _, err := os.Stat(input.Path); os.IsNotExist(err) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("the specified path not found\n"))
		return
	}

	log.Printf("hint %s:#%d\n", input.Path, input.Offset)

	if err := s.prebuild(input.Path); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
		return
	}
	w.Write([]byte("accepted\n"))
}

func (s HornetServer) prebuild(path string) error {
	if err := s.packageManager.Watch(path); err != nil {
		return fmt.Errorf("failed to watch %s: %v", path, err)
	}

	go func() {
		start := time.Now()

		pkg, _ := s.packageManager.Find(path)
		if err := pkg.Prebuild(); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Debugf("failed to prebuild: %v", err)
			}
			return
		}
		log.Debugf("prebuild time: %v\n", time.Since(start))
	}()
	return nil
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
	changedFilename := ""
	if !fi.IsDir() {
		pathDir = filepath.Dir(input.Path)
		changedFilename = input.Path
	}

	log.Printf("test %s:#%d\n", input.Path, input.Offset)

	if err := s.packageManager.Watch(pathDir); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to watch the package %s: %v\n", pathDir, err)
		return
	}
	pkg, _ := s.packageManager.Find(pathDir)

	job, err := NewJob(pkg, changedFilename, input.Offset, s.parallel(pkg.path, input.Parallel), input.BuildTags)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("failed to generate a new job: %v\n", err)
		fmt.Fprint(w, msg)
		log.Debug(msg)
		return
	}

	respWriter := newFlushWriter(w)
	if len(job.influence.to) == 0 {
		respWriter.Write([]byte("No important tests. Run all the tests:\n"))
	} else {
		respWriter.Write([]byte("Found important tests. Run them first:\n"))
	}

	log.Debugf("start the job id %d\n", job.ID)
	if err := s.jobManager.StartJob(context.Background(), job, s.defaultNumWorkers, respWriter); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("set up failed: %v\n", err)
		fmt.Fprint(w, msg)
		log.Debug(msg)
		return
	}

	s.WaitJob(w, job)
}

func (s HornetServer) parallel(path, clientArg string) bool {
	switch clientArg {
	case common.ParallelOn:
		return true
	case common.ParallelOff:
		return false
	default:
		if lastElapsedTime, ok := s.jobProfiler.LastElapsedTime(path); ok && lastElapsedTime < time.Second {
			return false
		}
		return true
	}
}

func (s HornetServer) WaitJob(w http.ResponseWriter, job *Job) {
	if err := s.jobManager.WaitJob(job.ID); err != nil {
		fmt.Fprintf(w, "set up failed: %v", err)
	}

	result := "FAIL"
	if job.Status == JobStatusSuccessful {
		result = "PASS"
	}

	s.jobProfiler.Add(job.DirPath, job.ElapsedTestTime)

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
