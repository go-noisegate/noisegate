package common

// These are just the internal APIs and no need to be the RESTful so far.

//////////////////////////
// APIs for the CLI
//////////////////////////

const cliAPIPrefix = "/cli"

// the API pathes
const (
	TestPath  = cliAPIPrefix + "/test"
	SetupPath = cliAPIPrefix + "/setup"
)

// TestRequest represents the input data to the test API.
type TestRequest struct {
	Path     string `json:"path"`
	Offset   int    `json:"offset"`
	Parallel string `json:"parallel"`
}

// valid parallel values
const (
	ParallelOn   = "on"
	ParallelOff  = "off"
	ParallelAuto = "auto"
)

// SetupRequest represents the input data to the setup API.
type SetupRequest struct {
	Path string `json:"path"`
}
