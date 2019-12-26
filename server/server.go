package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type testRequest struct {
	Path string `json:"path"`
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("http.Flusher is not implemented")
	}

	var input testRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&input); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Note that the data is not flushed if \n is not appended.
	fmt.Fprintf(w, "test %s...\n", input.Path)
	flusher.Flush()
	time.Sleep(100 * time.Millisecond)

	fmt.Fprintf(w, "ok")
	flusher.Flush()
}

// Run starts the new server.
func Run(addr string) error {
	mux := http.NewServeMux()
	// Want to be consistent with hornet cli. No need to be RESTful.
	mux.HandleFunc("/test", testHandler)

	server := &http.Server{
		Handler: mux,
		Addr:    addr,
	}
	return server.ListenAndServe()
}
