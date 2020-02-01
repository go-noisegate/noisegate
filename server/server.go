package server

import (
	"context"
	"encoding/json"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ks888/hornet/common"
	"github.com/ks888/hornet/common/log"
)

var sharedDir string                              // host side
const sharedDirOnContainer = "/opt/hornet/shared" // container side

// SetUpSharedDir initializes the specified directory.
func SetUpSharedDir(dir string) {
	sharedDir = dir
	os.Mkdir(filepath.Join(sharedDir, "bin"), os.ModePerm)
	os.Mkdir(filepath.Join(sharedDir, "lib"), os.ModePerm)
	os.Mkdir(filepath.Join(sharedDir, "log"), os.ModePerm)
	os.Mkdir(filepath.Join(sharedDir, "src"), os.ModePerm)

	log.Debugf("shared dir: %s", sharedDir)
}

// HornetServer serves the APIs for the cli client.
type HornetServer struct {
	*http.Server
	jobManager        *JobManager
	workerManager     *WorkerManager
	repositoryManager *RepositoryManager
	packageManager    *PackageManager
	depthLimit        int
}

// NewHornetServer returns the new hornet server.
// We can use only one server instance in the process even if the address is different.
func NewHornetServer(addr string, workerManager *WorkerManager) HornetServer {
	s := HornetServer{
		jobManager:        NewJobManager(),
		workerManager:     workerManager,
		repositoryManager: NewRepositoryManager(),
		packageManager:    NewPackageManager(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(common.TestPath, s.handleTest)
	mux.HandleFunc(common.SetupPath, s.handleSetup)
	mux.HandleFunc(common.NextTaskSetPath, s.handleNextTaskSet)
	mux.HandleFunc(common.ReportResultPath, s.handleReportResult)
	s.Server = &http.Server{
		Handler: mux,
		Addr:    addr,
	}
	return s
}

// Shutdown shutdowns the http server and workers.
func (s HornetServer) Shutdown(ctx context.Context) error {
	s.workerManager.RemoveWorkers()
	return s.Server.Shutdown(ctx)
}

func (s HornetServer) handleSetup(w http.ResponseWriter, r *http.Request) {
	var input common.SetupRequest
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

	log.Printf("setup %s\n", input.Path)

	if err := s.repositoryManager.Watch(input.Path, true); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("failed to watch %s: %v", input.Path, err)
		return
	}

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
		pkg, _ := s.packageManager.Find(path)
		if err := pkg.Prebuild(); err != nil {
			log.Printf("failed to prebuild: %v", err)
		}
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
	if !fi.IsDir() {
		pathDir = filepath.Dir(input.Path)
	}

	log.Printf("test %s\n", input.Path)

	if err := s.repositoryManager.Watch(pathDir, false); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to watch the repository %s: %v\n", pathDir, err)
		return
	}

	repo, _ := s.repositoryManager.Find(pathDir)
	job, err := NewJob(pathDir, repo, 0)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("failed to generate a new job: %v\n", err)
		fmt.Fprint(w, msg)
		log.Debug(msg)
		return
	}

	if err := s.jobManager.Partition(job, s.workerManager.NumWorkers()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("failed to divide the tests: %v\n", err)
		fmt.Fprint(w, msg)
		log.Debug(msg)
		return
	}
	w.WriteHeader(http.StatusOK)
	s.runAndWaitJob(w, job)
}

func (s HornetServer) runAndWaitJob(w http.ResponseWriter, job *Job) {
	log.Debugf("add the job id %d\n", job.ID)
	s.jobManager.AddJob(job)

	ch := make(chan int)
	for i := range job.TaskSets {
		go func(i int) {
			job.TaskSets[i].WaitFinished()
			ch <- i
		}(i)
	}

	for range job.TaskSets {
		i := <-ch
		s.writeTaskSetLog(w, job, job.TaskSets[i])
	}

	job.WaitFinished()

	result := "FAIL"
	if job.Status == JobStatusSuccessful {
		result = "PASS"
	}
	fmt.Fprintf(w, "%s: Job#%d (%s) (%v)\n", result, job.ID, job.DirPath, job.FinishedAt.Sub(job.CreatedAt))
}

// asyncBuildImportGraph builds the import graph and returns the func to get the built import graph.
// The returned func returns immediately if the import graph is available. Wait otherwise.
func (s HornetServer) asyncBuildImportGraph(path string) (getImportGraph func() *ImportGraph) {
	importGraphCh := make(chan *ImportGraph, 1)
	go func() {
		repoRoot := findRepoRoot(path)
		ctxt := &build.Default
		importGraph := BuildImportGraph(ctxt, repoRoot)
		importGraphCh <- &importGraph
	}()

	start := time.Now()
	var importGraph *ImportGraph
	return func() *ImportGraph {
		if importGraph == nil {
			importGraph = <-importGraphCh
			log.Debugf("time to build the import graph: %v\n", time.Since(start))
		}
		return importGraph
	}
}

func (s HornetServer) writeTaskSetLog(w io.Writer, job *Job, taskSet *TaskSet) {
	result := "FAIL"
	if taskSet.Status == TaskSetStatusSuccessful {
		result = "PASS"
	}
	elapsedTime := taskSet.FinishedAt.Sub(taskSet.StartedAt)
	fmt.Fprintf(w, "%s: Job#%d/TaskSet#%d (%s) (%v)\n", result, job.ID, taskSet.ID, job.DirPath, elapsedTime)
	content, err := ioutil.ReadFile(filepath.Join(sharedDir, taskSet.LogPath))
	if err != nil {
		log.Debugf("failed to read the log file: %v\n", err)
		fmt.Fprintf(w, "(no test log)\n")
	} else {
		fmt.Fprintf(w, "%s\n", string(content))
	}
}

// handleNextTaskSet handles the next task set request.
func (s HornetServer) handleNextTaskSet(w http.ResponseWriter, r *http.Request) {
	var req common.NextTaskSetRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	job, taskSet, err := s.jobManager.NextTaskSet(req.WorkerGroupName, req.WorkerID)
	if err != nil {
		if err == errNoTaskSet {
			w.WriteHeader(http.StatusNotFound)
		} else {
			log.Printf("failed to get the next task set: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	worker := s.workerManager.Workers[req.WorkerID]
	log.Debugf("%s handles the task set %d\n", worker.Name, taskSet.ID)

	resp := &common.NextTaskSetResponse{
		JobID:             job.ID,
		TaskSetID:         taskSet.ID,
		LogPath:           filepath.Join(sharedDirOnContainer, taskSet.LogPath),
		TestBinaryPath:    filepath.Join(sharedDirOnContainer, job.TestBinaryPath),
		RepoPath:          filepath.Join(sharedDirOnContainer, job.Repository.destPathFromSharedDir),
		RepoToPackagePath: job.RepoToPackagePath,
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
func (s HornetServer) handleReportResult(w http.ResponseWriter, r *http.Request) {
	var req common.ReportResultRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.jobManager.ReportResult(req.JobID, req.TaskSetID, req.Successful); err != nil {
		log.Printf("failed to report the result: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
