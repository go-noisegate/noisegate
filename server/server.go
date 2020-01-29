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
	"sync"
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
	depthLimit        int
}

// NewHornetServer returns the new hornet server.
// We can use only one server instance in the process even if the address is different.
func NewHornetServer(addr string, jobManager *JobManager, workerManager *WorkerManager, repositoryManager *RepositoryManager) HornetServer {
	s := HornetServer{jobManager: jobManager, workerManager: workerManager, repositoryManager: repositoryManager}

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
	err := s.Server.Shutdown(ctx)
	s.workerManager.RemoveWorkers() // not return error
	return err
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
	w.Write([]byte("accepted\n"))
}

func (s HornetServer) handleTest(w http.ResponseWriter, r *http.Request) {
	var input common.TestRequest
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
	pathDir := input.Path
	if !fi.IsDir() {
		pathDir = filepath.Dir(input.Path)
	}

	log.Printf("test %s\n", input.Path)

	// disable import graph feature for now
	// getImportGraph := s.asyncBuildImportGraph(pathDir)
	getImportGraph := func() *ImportGraph {
		return &ImportGraph{Root: pathDir, Inbounds: make(map[string][]string)}
	}

	var wg sync.WaitGroup
	var handleJob func(path string, depth int)
	handleJob = func(path string, depth int) {
		if err := s.repositoryManager.Watch(path, false); err != nil {
			fmt.Fprintf(w, "failed to watch the repository %s: %v\n", path, err)
			return
		}

		repo, _ := s.repositoryManager.Find(path)
		job, err := NewJob(path, repo, depth)
		if err != nil {
			fmt.Fprintf(w, "failed to generate a new job: %v\n", err)
			return
		}
		s.jobManager.Partition(job, s.workerManager.NumWorkers())
		s.runAndWaitJob(w, job)

		if job.Status != JobStatusSuccessful || depth == s.depthLimit {
			return
		}

		importGraph := getImportGraph()
		for _, inbound := range importGraph.Inbounds[path] {
			inbound := inbound
			wg.Add(1)
			go func() {
				defer wg.Done()
				handleJob(inbound, depth+1)
			}()
		}
	}

	handleJob(pathDir, 0)
	wg.Wait()
}

func (s HornetServer) runAndWaitJob(w http.ResponseWriter, job *Job) {
	log.Debugf("add the job id %d\n", job.ID)
	s.jobManager.AddJob(job)

	var wg sync.WaitGroup
	for _, taskSet := range job.TaskSets {
		wg.Add(1)
		go func(taskSet *TaskSet) {
			defer wg.Done()
			s.writeTaskSetLog(w, job, taskSet)
			// Note that the data is not flushed if \n is not appended.
			w.(http.Flusher).Flush()
		}(taskSet)
	}
	wg.Wait()

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
	taskSet.WaitFinished()

	result := "FAIL"
	if taskSet.Status == TaskSetStatusSuccessful {
		result = "PASS"
	}
	// TODO: protect the writer.
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
