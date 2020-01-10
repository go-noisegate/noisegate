package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ks888/hornet/common"
)

func Test_NextTaskSet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.NextTaskSetPath, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	})
	server := httptest.NewServer(mux)

	w := Executor{GroupName: "test", ID: 0, Addr: strings.TrimPrefix(server.URL, "http://")}
	_, err := w.nextTaskSet(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
