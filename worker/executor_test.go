package worker

import (
	"context"
	"encoding/json"
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

func Test_NextTaskSet(t *testing.T) {
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

	w := Executor{GroupName: "test", ID: 0, Addr: strings.TrimPrefix(server.URL, "http://")}
	nextTaskSet, err := w.nextTaskSet(context.Background())
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

func Test_NextTaskSet_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.NextTaskSetPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(mux)

	w := Executor{GroupName: "test", ID: 0, Addr: strings.TrimPrefix(server.URL, "http://")}
	_, err := w.nextTaskSet(context.Background())
	if err != errNoTaskSet {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Test_NextTaskSet_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.NextTaskSetPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{}`))
	})
	server := httptest.NewServer(mux)

	w := Executor{GroupName: "test", ID: 0, Addr: strings.TrimPrefix(server.URL, "http://")}
	_, err := w.nextTaskSet(context.Background())
	if err == nil {
		t.Fatalf("unexpected nil")
	}
}

func Test_ExtractRepoArchive(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, filename, _, _ := runtime.Caller(0)
	thisDir := filepath.Dir(filename)

	w := Executor{Workspace: tempDir}
	if err := w.extractRepoArchive(context.Background(), filepath.Join(thisDir, "testdata", "repo.tar")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "README.md")); os.IsNotExist(err) {
		t.Errorf("failed to extract some file(s)")
	}
}
