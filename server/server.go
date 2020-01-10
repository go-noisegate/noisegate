package server

import (
	"encoding/json"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/ks888/hornet/common"
	"github.com/ks888/hornet/common/log"
)

var sharedDir string

// HornetServer serves the APIs for the cli client.
type HornetServer struct {
	*http.Server
	manager    *JobManager
	depthLimit int
}

// NewHornetServer returns the new hornet server.
// We can use only one server instance in the process even if the address is different.
func NewHornetServer(addr, dir string, manager *JobManager) HornetServer {
	setSharedDir(dir)

	s := HornetServer{manager: manager}

	mux := http.NewServeMux()
	mux.HandleFunc(common.TestPath, s.handleTest)
	s.Server = &http.Server{
		Handler: mux,
		Addr:    addr,
	}
	return s
}

func setSharedDir(dir string) {
	sharedDir = dir
	os.Mkdir(filepath.Join(sharedDir, "bin"), os.ModePerm)
	os.Mkdir(filepath.Join(sharedDir, "lib"), os.ModePerm)
	os.Mkdir(filepath.Join(sharedDir, "log"), os.ModePerm)
}

func (s HornetServer) handleTest(w http.ResponseWriter, r *http.Request) {
	var input common.TestRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&input); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	getImportGraph := s.asyncBuildImportGraph(input.Path)

	var wg sync.WaitGroup
	var handleJob func(path string, depth int)
	handleJob = func(path string, depth int) {
		job, err := NewJob("", path, depth)
		if err != nil {
			log.Printf("failed to generate a new job: %v\n", err)
			return
		}
		s.runAndWaitJob(w, job)

		if job.Status != JobStatusSuccessful || depth == s.depthLimit {
			return
		}

		importGraph := getImportGraph()
		for _, inbound := range importGraph.Inbounds[input.Path] {
			inbound := inbound
			wg.Add(1)
			go func() {
				defer wg.Done()
				handleJob(inbound, depth+1)
			}()
		}
	}

	handleJob(input.Path, 0)
	wg.Wait()
}

func (s HornetServer) runAndWaitJob(w http.ResponseWriter, job *Job) {
	s.manager.AddJob(job)

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
}

// asyncBuildImportGraph builds the import graph and returns the func to get the built import graph.
// The returned func returns immediately if the import graph is available. Wait otherwise.
func (s HornetServer) asyncBuildImportGraph(path string) (getImportGraph func() *ImportGraph) {
	importGraphCh := make(chan *ImportGraph, 1)
	go func() {
		repoRoot, err := findRepoRoot(path)
		if err != nil {
			log.Printf("failed to find the repository root of %s: %v", path, err)
			repoRoot = path
		}
		ctxt := &build.Default
		importGraph := BuildImportGraph(ctxt, repoRoot)
		importGraphCh <- &importGraph
	}()

	var importGraph *ImportGraph
	return func() *ImportGraph {
		if importGraph == nil {
			importGraph = <-importGraphCh
		}
		return importGraph
	}
}

func (s HornetServer) writeTaskSetLog(w io.Writer, job *Job, taskSet *TaskSet) {
	taskSet.WaitFinished()

	var result string
	if taskSet.Status == TaskSetStatusSuccessful {
		result = "PASS"
	} else {
		result = "FAIL"
	}
	// TODO: protect the writer.
	fmt.Fprintf(w, "=== %s (job: %d, task set: %d, path: %s)\n", result, job.ID, taskSet.ID, job.DirPath)
	content, err := ioutil.ReadFile(filepath.Join(sharedDir, taskSet.LogPath))
	if err != nil {
		log.Debugf("failed to read the log file %s: %v", taskSet.LogPath, err)
	} else {
		fmt.Fprintf(w, "%s\n", string(content))
	}
	fmt.Fprintf(w, "Total time: %v\n", taskSet.FinishedAt.Sub(taskSet.StartedAt))
}
