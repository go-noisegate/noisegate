package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-noisegate/noisegate/common"
)

func TestHandleHint_InputIsFileAndRange(t *testing.T) {
	server := NewServer("")

	curr, _ := os.Getwd()
	path := filepath.Join(curr, "testdata", "typical", "sum.go")
	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(fmt.Sprintf(`{"path": "%s", "ranges": [{"begin": 1, "end": 2}]}`, path)))
	w := httptest.NewRecorder()
	server.handleHint(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	if string(out) != "accepted\n" {
		t.Errorf("unexpected response body: %s", string(out))
	}

	changes := server.changeManager.Find(filepath.Dir(path))
	if len(changes) != 1 || changes[0] != (Change{filepath.Base(path), 1, 2}) {
		t.Errorf("wrong changes: %#v", changes)
	}
}

func TestHandleHint_InputIsFile(t *testing.T) {
	server := NewServer("")

	curr, _ := os.Getwd()
	path := filepath.Join(curr, "testdata", "typical", "sum.go")
	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	server.handleHint(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}

	changes := server.changeManager.Find(filepath.Dir(path))
	if len(changes) != 0 {
		t.Errorf("wrong changes: %#v", changes)
	}
}

func TestHandleHint_InputIsDirectory(t *testing.T) {
	server := NewServer("")

	curr, _ := os.Getwd()
	path := filepath.Join(curr, "testdata", "typical")
	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(fmt.Sprintf(`{"path": "%s", "ranges": [{"begin": 1, "end": 2}]}`, path)))
	w := httptest.NewRecorder()
	server.handleHint(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}

	changes := server.changeManager.Find(filepath.Dir(path))
	if len(changes) != 0 {
		t.Errorf("wrong changes: %#v", changes)
	}
}

func TestHandleHint_InvalidJSON(t *testing.T) {
	server := NewServer("")

	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(`{`))
	w := httptest.NewRecorder()
	server.handleHint(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleHint_RelativePath(t *testing.T) {
	server := NewServer("")

	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(`{"path": "rel/path"}`))
	w := httptest.NewRecorder()
	server.handleHint(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleHint_PathNotFound(t *testing.T) {
	server := NewServer("")

	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(`{"path": "/path/to/not/exist/file"}`))
	w := httptest.NewRecorder()
	server.handleHint(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleHint_IdentifyMultiplePathRepresentations(t *testing.T) {
	server := NewServer("")

	curr, _ := os.Getwd()
	pathList := []string{
		filepath.Join(curr, "testdata", "typical", "sum.go"),
		filepath.Join(curr, "testdata", "typical", ".", "sum.go"),
		filepath.Join(curr, "testdata", "typical", "..", "typical", "sum.go"),
	}
	for _, p := range pathList {
		req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(fmt.Sprintf(`{"path": "%s", "ranges": [{"begin": 1, "end": 2}]}`, p)))
		w := httptest.NewRecorder()
		server.handleHint(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("unexpected code: %d", w.Code)
		}
	}

	changes := server.changeManager.Find(filepath.Dir(pathList[0]))
	if len(changes) != len(pathList) {
		t.Errorf("wrong changes: %#v", changes)
	}
}

func TestHandleTest_InputIsDirectory(t *testing.T) {
	server := NewServer("")

	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	path := filepath.Join(curr, "testdata", "typical", "sum_test.go")
	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(fmt.Sprintf(`{"path": "%s", "ranges": [{"begin": 0, "end": 99}]}`, path)))
	w := httptest.NewRecorder()
	server.handleHint(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	req = httptest.NewRequest("GET", common.TestPath, strings.NewReader(fmt.Sprintf(`{"path": "%s", "go_test_options": ["-v"]}`, filepath.Dir(path))))
	w = httptest.NewRecorder()
	server.handleTest(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	if !strings.Contains(string(out), "PASS: TestSum") {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestHandleTest_InputIsFile(t *testing.T) {
	server := NewServer("")

	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	path := filepath.Join(curr, "testdata", "typical", "sum_test.go")
	req := httptest.NewRequest("GET", common.TestPath, strings.NewReader(fmt.Sprintf(`{"path": "%s", "go_test_options": ["-v"]}`, path)))
	w := httptest.NewRecorder()
	server.handleTest(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleTest_EmptyBody(t *testing.T) {
	server := NewServer("")

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	server.handleTest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleTest_RelativePath(t *testing.T) {
	server := NewServer("")

	req := httptest.NewRequest("GET", "/test", strings.NewReader(`{"path": "rel/path"}`))
	w := httptest.NewRecorder()
	server.handleTest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}
