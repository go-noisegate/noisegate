package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleNextTaskSet(t *testing.T) {
	manager := NewManager("")
	job := &Job{ID: 1, DirPath: "/path/to/dir/", TestBinaryPath: "/path/to/binary"}
	task := &Task{TestFunction: "TestFunc1", Job: job}
	taskSet := &TaskSet{ID: 1, Tasks: []*Task{task}}
	job.TaskSets = append(job.TaskSets, taskSet)
	manager.scheduler.Add(taskSet, 0)

	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, strings.NewReader(`{"worker_id": 1}`))
	resp := httptest.NewRecorder()
	manager.handleNextTaskSet(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("unexpected status: %d", resp.Code)
	}

	decodedResp := &nextTaskSetResponse{}
	if err := json.Unmarshal(resp.Body.Bytes(), decodedResp); err != nil {
		t.Fatalf("failed to unmarshal resp body: %v", err)
	}
	if decodedResp.TaskSetID != 1 {
		t.Errorf("unexpected task set id: %d", decodedResp.TaskSetID)
	}
	if decodedResp.JobID != 1 {
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

	if taskSet.Status != TaskSetStatusStarted {
		t.Errorf("unexpected task set status: %v", taskSet.Status)
	}
}

func TestHandleNextTaskSet_NoTaskSet(t *testing.T) {
	manager := NewManager("")
	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, strings.NewReader(`{}`))
	resp := httptest.NewRecorder()
	manager.handleNextTaskSet(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}

func TestHandleNextTaskSet_EmptyTaskSet(t *testing.T) {
	manager := NewManager("")
	manager.scheduler.Add(&TaskSet{ID: 1}, 0) // empty
	task := &Task{TestFunction: "TestFunc1", Job: &Job{ID: 1}}
	manager.scheduler.Add(&TaskSet{ID: 2, Tasks: []*Task{task}}, 1) // not empty

	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, strings.NewReader(`{}`))
	resp := httptest.NewRecorder()
	manager.handleNextTaskSet(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("unexpected status: %d", resp.Code)
	}
	decodedResp := &nextTaskSetResponse{}
	if err := json.Unmarshal(resp.Body.Bytes(), decodedResp); err != nil {
		t.Fatalf("failed to unmarshal resp body: %v", err)
	}
	if decodedResp.TaskSetID != 2 {
		t.Errorf("unexpected task set id: %d", decodedResp.TaskSetID)
	}
}

func TestHandleNextTaskSet_InvalidReqBody(t *testing.T) {
	manager := NewManager("")
	task := &Task{TestFunction: "TestFunc1", Job: &Job{ID: 1}}
	manager.scheduler.Add(&TaskSet{ID: 1, Tasks: []*Task{task}}, 0)

	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, strings.NewReader(`{`))
	resp := httptest.NewRecorder()
	manager.handleNextTaskSet(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}
