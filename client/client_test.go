package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ks888/hornet/client"
	"github.com/ks888/hornet/common"
)

func Test_TestAction(t *testing.T) {
	out := "ok"
	mux := http.NewServeMux()
	mux.HandleFunc(common.TestPath, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(out))
	})
	server := httptest.NewServer(mux)

	testfile := "/path/to/test/file"
	logger := &strings.Builder{}
	options := client.TestOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://"), TestLogger: logger}
	err := client.TestAction(context.Background(), testfile, options)
	if err != nil {
		t.Error(err)
	}

	if logger.String() != out {
		t.Errorf("unexpected log: %v", logger.String())
	}
}

func Test_TestAction_BadRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.TestPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	server := httptest.NewServer(mux)

	testfile := "/path/to/test/file"
	logger := &strings.Builder{}
	options := client.TestOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://"), TestLogger: logger}
	err := client.TestAction(context.Background(), testfile, options)
	if err == nil {
		t.Errorf("nil error")
	}
}

func Test_SetupAction(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.SetupPath, func(w http.ResponseWriter, r *http.Request) {})
	server := httptest.NewServer(mux)

	testfile := "/path/to/test/file"
	options := client.SetupOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://")}
	err := client.SetupAction(context.Background(), testfile, options)
	if err != nil {
		t.Error(err)
	}
}
