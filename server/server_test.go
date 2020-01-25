package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ks888/hornet/common"
)

func TestHandleWatch(t *testing.T) {
	repoManager := NewRepositoryManager()
	server := NewHornetServer("", NewJobManager(), &WorkerManager{}, repoManager)

	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	path := filepath.Join(curr, "testdata", "sum.go")
	req := httptest.NewRequest("GET", common.WatchPath, strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	server.handleWatch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	if string(out) != "accepted\n" {
		t.Errorf("unexpected response body: %s", string(out))
	}
}

func TestHandleWatch_InvalidJSON(t *testing.T) {
	repoManager := NewRepositoryManager()
	server := NewHornetServer("", NewJobManager(), &WorkerManager{}, repoManager)

	req := httptest.NewRequest("GET", common.WatchPath, strings.NewReader(`{`))
	w := httptest.NewRecorder()
	server.handleWatch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleWatch_PathNotFound(t *testing.T) {
	repoManager := NewRepositoryManager()
	server := NewHornetServer("", NewJobManager(), &WorkerManager{}, repoManager)

	req := httptest.NewRequest("GET", common.WatchPath, strings.NewReader(`{"path": "/path/to/not/exist/file"}`))
	w := httptest.NewRecorder()
	server.handleWatch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleTest_Depth0(t *testing.T) {
	jobManager := NewJobManager()
	workerManager := &WorkerManager{}
	server := NewHornetServer("", jobManager, workerManager, NewRepositoryManager())

	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	path := filepath.Join(curr, "testdata", "sum.go")
	req := httptest.NewRequest("GET", "/test", strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	go func() {
		executeTaskSet(t, jobManager)
	}()
	server.handleTest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	matched, _ := regexp.Match(`(?m)^PASS: Job#\d+/TaskSet#0 \(`+filepath.Dir(path)+`\) `, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
	matched, _ = regexp.Match(`(?m)^ok$`, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
	matched, _ = regexp.Match(`(?m)^PASS: Job#\d+ \(`+filepath.Dir(path)+`\) `, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestHandleTest_Depth1(t *testing.T) {
	jobManager := NewJobManager()
	workerManager := &WorkerManager{}
	server := NewHornetServer("", jobManager, workerManager, NewRepositoryManager())
	server.depthLimit = 1

	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	// Importgraph builder ignores testdata dir
	path := filepath.Join(curr, "server.go")
	req := httptest.NewRequest("GET", "/test", strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	go func() {
		executeTaskSet(t, jobManager)
		executeTaskSet(t, jobManager)
	}()
	server.handleTest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	matched, _ := regexp.Match(`(?m)^PASS: Job#\d+/TaskSet#0 \(`+filepath.Dir(path)+`\) `, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
	matched, _ = regexp.Match(`(?m)^PASS: Job#\d+/TaskSet#0 \(`+filepath.Dir(filepath.Dir(path))+`/cmd/hornetd\) `, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestHandleTest_EmptyBody(t *testing.T) {
	jobManager := NewJobManager()
	workerManager := &WorkerManager{}
	server := NewHornetServer("", jobManager, workerManager, NewRepositoryManager())

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	server.handleTest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func executeTaskSet(t *testing.T, jobManager *JobManager) {
	var job *Job
	var taskSet *TaskSet
	for {
		var err error
		job, taskSet, err = jobManager.NextTaskSet(workerGroupName, 0)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	err := ioutil.WriteFile(filepath.Join(sharedDir, taskSet.LogPath), []byte("ok"), 0644)
	if err != nil {
		t.Fatalf("failed to write log %s: %v", taskSet.LogPath, err)
	}

	if err := jobManager.ReportResult(job.ID, taskSet.ID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleNextTaskSet(t *testing.T) {
	jobManager := NewJobManager()
	job := &Job{ID: 1, DirPath: "/path/to/dir/", TestBinaryPath: "/path/to/binary", RepoArchivePath: "/path/to/archive", finishedCh: make(chan struct{})}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job})
	jobManager.AddJob(job)

	workerManager := &WorkerManager{Workers: []Worker{{Name: "test"}}}
	server := NewHornetServer("", jobManager, workerManager, NewRepositoryManager())
	req := httptest.NewRequest(http.MethodGet, common.NextTaskSetPath, strings.NewReader(`{"worker_id": 0}`))
	resp := httptest.NewRecorder()
	server.handleNextTaskSet(resp, req)
	if resp.Code != http.StatusOK {
		t.Errorf("unexpected status: %d", resp.Code)
	}

	decodedResp := &common.NextTaskSetResponse{}
	if err := json.Unmarshal(resp.Body.Bytes(), decodedResp); err != nil {
		t.Fatalf("failed to unmarshal resp body: %v", err)
	}
	taskSet := job.TaskSets[0]
	if decodedResp.TaskSetID != taskSet.ID {
		t.Errorf("unexpected task set id: %d", decodedResp.TaskSetID)
	}
	if decodedResp.JobID != job.ID {
		t.Errorf("unexpected job id: %d", decodedResp.JobID)
	}
	if len(decodedResp.TestFunctions) != 1 || decodedResp.TestFunctions[0] != "TestFunc1" {
		t.Errorf("unexpected test functions: %v", decodedResp.TestFunctions)
	}
	if decodedResp.LogPath == "/opt/hornet/shared" {
		t.Errorf("empty log path")
	}
	if decodedResp.TestBinaryPath != "/opt/hornet/shared/path/to/binary" {
		t.Errorf("unexpected test binary path: %s", decodedResp.TestBinaryPath)
	}
	if decodedResp.RepoArchivePath != "/opt/hornet/shared/path/to/archive" {
		t.Errorf("unexpected test binary path: %s", decodedResp.TestBinaryPath)
	}

	if taskSet.Status != TaskSetStatusStarted {
		t.Errorf("unexpected task set status: %v", taskSet.Status)
	}
}

func TestHandleNextTaskSet_NoTaskSet(t *testing.T) {
	jobManager := NewJobManager()
	workerManager := &WorkerManager{Workers: []Worker{{Name: "test"}}}
	server := NewHornetServer("", jobManager, workerManager, NewRepositoryManager())
	req := httptest.NewRequest(http.MethodGet, common.NextTaskSetPath, strings.NewReader(`{}`))
	resp := httptest.NewRecorder()
	server.handleNextTaskSet(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}

func TestHandleNextTaskSet_EmptyTaskSet(t *testing.T) {
	jobManager := NewJobManager()
	emptyJob := &Job{ID: 1, finishedCh: make(chan struct{})}
	jobManager.AddJob(emptyJob)

	job := &Job{ID: 2, finishedCh: make(chan struct{})}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job})
	jobManager.AddJob(job)

	workerManager := &WorkerManager{Workers: []Worker{{Name: "test"}}}
	server := NewHornetServer("", jobManager, workerManager, NewRepositoryManager())
	req := httptest.NewRequest(http.MethodGet, common.NextTaskSetPath, strings.NewReader(`{}`))
	resp := httptest.NewRecorder()
	server.handleNextTaskSet(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("unexpected status: %d", resp.Code)
	}
	decodedResp := &common.NextTaskSetResponse{}
	if err := json.Unmarshal(resp.Body.Bytes(), decodedResp); err != nil {
		t.Fatalf("failed to unmarshal resp body: %v", err)
	}
	if decodedResp.JobID != 2 {
		t.Errorf("unexpected job id: %d", decodedResp.JobID)
	}
}

func TestHandleNextTaskSet_InvalidReqBody(t *testing.T) {
	jobManager := NewJobManager()
	workerManager := &WorkerManager{Workers: []Worker{{Name: "test"}}}
	server := NewHornetServer("", jobManager, workerManager, NewRepositoryManager())
	req := httptest.NewRequest(http.MethodGet, common.NextTaskSetPath, strings.NewReader(`{`))
	resp := httptest.NewRecorder()
	server.handleNextTaskSet(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}

func TestHandleReportResult(t *testing.T) {
	job := &Job{ID: 1, finishedCh: make(chan struct{})}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestJobManager_ReportResult", Job: job})
	jobManager := NewJobManager()
	jobManager.AddJob(job)

	logContent := "=== RUN   TestJobManager_ReportResult\n--- PASS: TestJobManager_ReportResult (1.00s)"
	err := ioutil.WriteFile(filepath.Join(sharedDir, job.TaskSets[0].LogPath), []byte(logContent), 0644)
	if err != nil {
		t.Fatalf("failed to write log %s: %v", job.TaskSets[0].LogPath, err)
	}

	workerManager := &WorkerManager{}
	server := NewHornetServer("", jobManager, workerManager, NewRepositoryManager())
	reqBody := common.ReportResultRequest{
		JobID:      1,
		TaskSetID:  0,
		Successful: true,
	}
	data, _ := json.Marshal(&reqBody)
	req := httptest.NewRequest(http.MethodGet, common.ReportResultPath, bytes.NewReader(data))
	resp := httptest.NewRecorder()
	server.handleReportResult(resp, req)
	if resp.Code != http.StatusOK {
		t.Errorf("unexpected status: %d", resp.Code)
	}

	taskSet := job.TaskSets[0]
	if taskSet.Status != TaskSetStatusSuccessful {
		t.Errorf("unexpected status: %v", taskSet.Status)
	}
	if job.Status != JobStatusSuccessful {
		t.Errorf("unexpected status: %v", job.Status)
	}
	if _, ok := jobManager.jobs[job.ID]; ok {
		t.Errorf("the job is not removed")
	}
	if job.Tasks[0].Status != TaskStatusSuccessful {
		t.Errorf("unexpected status: %v", job.Tasks[0].Status)
	}
	if job.Tasks[0].ElapsedTime != time.Second {
		t.Errorf("unexpected elapsed time: %v", job.Tasks[0].ElapsedTime)
	}
}

func TestHandleReportResult_JobNotFound(t *testing.T) {
	jobManager := NewJobManager()
	workerManager := &WorkerManager{}
	server := NewHornetServer("", jobManager, workerManager, NewRepositoryManager())
	reqBody := common.ReportResultRequest{JobID: 1, TaskSetID: 0, Successful: true}
	data, _ := json.Marshal(&reqBody)
	req := httptest.NewRequest(http.MethodGet, common.ReportResultPath, bytes.NewReader(data))
	resp := httptest.NewRecorder()
	server.handleReportResult(resp, req)
	if resp.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}

func TestHandleReportResult_InvalidJSON(t *testing.T) {
	jobManager := NewJobManager()
	workerManager := &WorkerManager{}
	server := NewHornetServer("", jobManager, workerManager, NewRepositoryManager())

	req := httptest.NewRequest(http.MethodGet, common.ReportResultPath, strings.NewReader("{"))
	resp := httptest.NewRecorder()
	server.handleReportResult(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}
