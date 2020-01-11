package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ks888/hornet/common"
)

func TestHandleTest_Depth0(t *testing.T) {
	manager := NewJobManager()
	server := NewHornetServer("", sharedDir, manager)

	_, filename, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(filename), "testdata")
	req := httptest.NewRequest("GET", "/test", strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	go func() {
		executeTaskSet(t, manager)
	}()
	server.handleTest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	matched, _ := regexp.Match(`(?m)^=== PASS \(job: \d+, task set: 0, path: `+path+`\)$`, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
	matched, _ = regexp.Match(`(?m)^ok$`, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestHandleTest_Depth1(t *testing.T) {
	manager := NewJobManager()
	server := NewHornetServer("", sharedDir, manager)
	server.depthLimit = 1

	_, filename, _, _ := runtime.Caller(0)
	// Importgraph builder ignores testdata dir
	path := filepath.Dir(filename)
	req := httptest.NewRequest("GET", "/test", strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	go func() {
		executeTaskSet(t, manager)
		executeTaskSet(t, manager)
	}()
	server.handleTest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	matched, _ := regexp.Match(`(?m)^=== PASS \(job: \d+, task set: 0, path: `+path+`\)$`, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
	matched, _ = regexp.Match(`(?m)^=== PASS \(job: \d+, task set: 0, path: `+filepath.Dir(path)+`/cmd/hornetd\)$`, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestHandleTest_EmptyBody(t *testing.T) {
	manager := NewJobManager()
	server := NewHornetServer("", sharedDir, manager)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	server.handleTest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func executeTaskSet(t *testing.T, manager *JobManager) {
	var job *Job
	var taskSet *TaskSet
	for {
		var err error
		job, taskSet, err = manager.NextTaskSet(workerGroupName, 0)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	err := ioutil.WriteFile(filepath.Join(sharedDir, taskSet.LogPath), []byte("ok"), 0644)
	if err != nil {
		t.Fatalf("failed to write log %s: %v", taskSet.LogPath, err)
	}

	if err := manager.ReportResult(job.ID, taskSet.ID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleNextTaskSet(t *testing.T) {
	manager := NewJobManager()
	job := &Job{ID: 1, DirPath: "/path/to/dir/", TestBinaryPath: "/path/to/binary", RepoArchivePath: "/path/to/archive", finishedCh: make(chan struct{})}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job})
	manager.AddJob(job)

	server := NewHornetServer("", sharedDir, manager)
	req := httptest.NewRequest(http.MethodGet, common.NextTaskSetPath, strings.NewReader(`{"worker_id": 1}`))
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
	if decodedResp.DirPath != "/path/to/dir/" {
		t.Errorf("unexpected dir path: %s", decodedResp.DirPath)
	}
	if decodedResp.TestBinaryPath != "/path/to/binary" {
		t.Errorf("unexpected test binary path: %s", decodedResp.TestBinaryPath)
	}
	if decodedResp.RepoArchivePath != "/path/to/archive" {
		t.Errorf("unexpected test binary path: %s", decodedResp.TestBinaryPath)
	}

	if taskSet.Status != TaskSetStatusStarted {
		t.Errorf("unexpected task set status: %v", taskSet.Status)
	}
}

func TestHandleNextTaskSet_NoTaskSet(t *testing.T) {
	manager := NewJobManager()
	server := NewHornetServer("", sharedDir, manager)
	req := httptest.NewRequest(http.MethodGet, common.NextTaskSetPath, strings.NewReader(`{}`))
	resp := httptest.NewRecorder()
	server.handleNextTaskSet(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}

func TestHandleNextTaskSet_EmptyTaskSet(t *testing.T) {
	manager := NewJobManager()
	emptyJob := &Job{ID: 1, finishedCh: make(chan struct{})}
	manager.AddJob(emptyJob)

	job := &Job{ID: 2, finishedCh: make(chan struct{})}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job})
	manager.AddJob(job)

	server := NewHornetServer("", sharedDir, manager)
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
	manager := NewJobManager()
	server := NewHornetServer("", sharedDir, manager)
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
	manager := NewJobManager()
	manager.AddJob(job)

	logContent := "=== RUN   TestJobManager_ReportResult\n--- PASS: TestJobManager_ReportResult (1.00s)"
	err := ioutil.WriteFile(filepath.Join(sharedDir, job.TaskSets[0].LogPath), []byte(logContent), 0644)
	if err != nil {
		t.Fatalf("failed to write log %s: %v", job.TaskSets[0].LogPath, err)
	}

	server := NewHornetServer("", sharedDir, manager)
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
	if _, ok := manager.jobs[job.ID]; ok {
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
	manager := NewJobManager()
	server := NewHornetServer("", sharedDir, manager)
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
	manager := NewJobManager()
	server := NewHornetServer("", sharedDir, manager)

	req := httptest.NewRequest(http.MethodGet, common.ReportResultPath, strings.NewReader("{"))
	resp := httptest.NewRecorder()
	server.handleReportResult(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}
