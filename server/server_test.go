package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTestHandler(t *testing.T) {
	path := "/go/src/github.com/ks888/hornet/server/server.go"
	req := httptest.NewRequest("GET", "/test", strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	testHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, err := ioutil.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "test "+path+"...\nok" {
		t.Errorf("unexpected content: %v", string(out))
	}
}
