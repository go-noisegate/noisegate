package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ks888/hornet/common"
)

func TestRun(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, filename, _, _ := runtime.Caller(0)
	thisDir := filepath.Dir(filename)

	testCases := []struct {
		input  nextTaskSet
		expect bool
	}{
		{
			input: nextTaskSet{
				JobID:           1,
				TaskSetID:       1,
				LogPath:         filepath.Join(tempDir, "testlog"),
				RepoArchivePath: filepath.Join(thisDir, "testdata", "repo.tar"),
				TestBinaryPath:  "echo",
			},
			expect: true,
		},
		{
			input: nextTaskSet{
				JobID:           1,
				TaskSetID:       1,
				LogPath:         filepath.Join(tempDir, "testlog"),
				RepoArchivePath: "/path/to/not/exist/file",
				TestBinaryPath:  "echo",
			},
			expect: false,
		},
		{
			input: nextTaskSet{
				JobID:           1,
				TaskSetID:       1,
				LogPath:         filepath.Join(tempDir, "testlog"),
				RepoArchivePath: filepath.Join(thisDir, "testdata", "repo.tar"),
				TestBinaryPath:  "cmd-not-exist",
			},
			expect: false,
		},
	}

	for _, testCase := range testCases {
		done := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-done
			cancel()
		}()

		mux := http.NewServeMux()
		first := true
		mux.HandleFunc(common.NextTaskSetPath, func(w http.ResponseWriter, r *http.Request) {
			if !first {
				w.WriteHeader(http.StatusNotFound)
				done <- struct{}{}
				return
			}

			resp := common.NextTaskSetResponse(testCase.input)
			out, _ := json.Marshal(&resp)
			w.Write(out)
			first = false
		})
		var rawReport []byte
		mux.HandleFunc(common.ReportResultPath, func(w http.ResponseWriter, r *http.Request) {
			rawReport, _ = ioutil.ReadAll(r.Body)
		})
		server := httptest.NewServer(mux)

		e := Executor{GroupName: "test", ID: 0, Addr: strings.TrimPrefix(server.URL, "http://"), Workspace: tempDir}
		if err := e.Run(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("not canceled error: %v", err)
		}
		if !strings.Contains(string(rawReport), fmt.Sprintf(`"successful":%v`, testCase.expect)) {
			t.Errorf("invalid report: %s", string(rawReport))
		}
	}
}

func TestNextTaskSet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.NextTaskSetPath, func(w http.ResponseWriter, r *http.Request) {
		resp := common.NextTaskSetResponse{
			JobID:         1,
			TaskSetID:     1,
			TestFunctions: []string{"f1"},
		}
		out, err := json.Marshal(&resp)
		if err != nil {
			t.Fatalf("failed to encode: %v", err)
		}
		w.Write(out)
	})
	server := httptest.NewServer(mux)

	e := Executor{GroupName: "test", ID: 0, Addr: strings.TrimPrefix(server.URL, "http://")}
	nextTaskSet, err := e.nextTaskSet(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nextTaskSet.JobID != 1 {
		t.Errorf("invalid job id: %d", nextTaskSet.JobID)
	}
	if nextTaskSet.TaskSetID != 1 {
		t.Errorf("invalid task set id: %d", nextTaskSet.TaskSetID)
	}
	if len(nextTaskSet.TestFunctions) != 1 || nextTaskSet.TestFunctions[0] != "f1" {
		t.Errorf("invalid test functions: %#v", nextTaskSet.TestFunctions)
	}
}

func TestNextTaskSet_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.NextTaskSetPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(mux)

	e := Executor{GroupName: "test", ID: 0, Addr: strings.TrimPrefix(server.URL, "http://")}
	_, err := e.nextTaskSet(context.Background())
	if err != errNoTaskSet {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNextTaskSet_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.NextTaskSetPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{}`))
	})
	server := httptest.NewServer(mux)

	e := Executor{GroupName: "test", ID: 0, Addr: strings.TrimPrefix(server.URL, "http://")}
	_, err := e.nextTaskSet(context.Background())
	if err == nil {
		t.Fatalf("unexpected nil")
	}
}

func TestCreateWorkspace(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, filename, _, _ := runtime.Caller(0)
	thisDir := filepath.Dir(filename)

	taskSet := nextTaskSet{
		JobID:           1,
		LogPath:         filepath.Join(tempDir, "testlog"),
		RepoArchivePath: filepath.Join(thisDir, "testdata", "repo.tar"),
	}
	e := Executor{Workspace: tempDir}
	if err := e.createWorkspace(context.Background(), taskSet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(e.workspacePath(taskSet), "README.md")); os.IsNotExist(err) {
		t.Errorf("failed to create ws")
	}
}

func TestCreateWorkspace_InvalidPath(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "testlog")
	taskSet := nextTaskSet{
		LogPath:         logPath,
		RepoArchivePath: "/path/to/not/exist/file",
	}
	e := Executor{Workspace: tempDir}
	if err := e.createWorkspace(context.Background(), taskSet); err == nil {
		t.Fatalf("nil error: %v", err)
	}
	out, err := ioutil.ReadFile(logPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) == "" {
		t.Errorf("empty log")
	}
}

func TestCreateWorkspace_LogPathNotFound(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	thisDir := filepath.Dir(filename)

	taskSet := nextTaskSet{
		LogPath:        "/path/to/not/exist/file",
		TestBinaryPath: "echo",
	}
	e := Executor{Workspace: thisDir}
	if err := e.createWorkspace(context.Background(), taskSet); err == nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestCreateWorkspace_LogFileHasExistingData(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet-test")
	if err != nil {
		t.Fatalf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "testlog")
	if err := ioutil.WriteFile(logPath, []byte("initial data"), os.ModePerm); err != nil {
		t.Fatalf("failed to write the data to temp file %s: %v", logPath, err)
	}

	taskSet := nextTaskSet{
		LogPath:         logPath,
		RepoArchivePath: "/path/to/not/exist/file",
	}
	e := Executor{Workspace: tempDir}
	if err := e.createWorkspace(context.Background(), taskSet); err == nil {
		t.Fatalf("nil error: %v", err)
	}
	out, err := ioutil.ReadFile(logPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(string(out), "initial data") {
		t.Errorf("existing data is removed")
	}
}

func TestExecute(t *testing.T) {
	tempFile, err := ioutil.TempFile("", "hornet-test")
	if err != nil {
		t.Errorf("failed to create the temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	_, filename, _, _ := runtime.Caller(0)
	thisDir := filepath.Dir(filename)

	taskSet := nextTaskSet{
		TestFunctions:  []string{"f1", "f2"},
		LogPath:        tempFile.Name(),
		TestBinaryPath: "echo",
	}
	e := Executor{Workspace: thisDir}
	if err := e.createWorkspace(context.Background(), taskSet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := e.execute(context.Background(), taskSet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args, _ := ioutil.ReadFile(tempFile.Name())
	if "-test.v -test.run f1|f2\n" != string(args) {
		t.Errorf("unexpected command args: %s", string(args))
	}
}

func TestExecute_ExitCodeNot0(t *testing.T) {
	tempFile, err := ioutil.TempFile("", "hornet-test")
	if err != nil {
		t.Errorf("failed to create the temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	taskSet := nextTaskSet{
		LogPath:        tempFile.Name(),
		TestBinaryPath: "cmd-not-exist",
	}
	e := Executor{}
	if err := e.createWorkspace(context.Background(), taskSet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := e.execute(context.Background(), taskSet); err == nil {
		t.Fatalf("nil error")
	}
}

func TestReportResult(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.ReportResultPath, func(w http.ResponseWriter, r *http.Request) {
	})
	server := httptest.NewServer(mux)

	taskSet := nextTaskSet{JobID: 1, TaskSetID: 1}
	e := Executor{Addr: strings.TrimPrefix(server.URL, "http://")}
	if err := e.reportResult(context.Background(), taskSet, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReportResult_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.ReportResultPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)

	taskSet := nextTaskSet{JobID: 1, TaskSetID: 1}
	e := Executor{Addr: strings.TrimPrefix(server.URL, "http://")}
	if err := e.reportResult(context.Background(), taskSet, true); err == nil {
		t.Fatalf("nil error: %v", err)
	}
}
