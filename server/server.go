package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ks888/hornet/common"
)

var sharedDir string

func testHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("http.Flusher is not implemented")
	}

	var input common.TestRequest
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

type HornetServer struct {
	*http.Server
}

// NewHornetServer returns the new hornet server.
// We can use only one server instance in the process even if the address is different.
func NewHornetServer(addr, dir string) HornetServer {
	sharedDir = dir

	mux := http.NewServeMux()
	// Want to be consistent with hornet cli. No need to be RESTful.
	mux.HandleFunc(common.TestPath, testHandler)

	return HornetServer{
		&http.Server{
			Handler: mux,
			Addr:    addr,
		},
	}
}
