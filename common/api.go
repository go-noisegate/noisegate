package common

// These are just the internal APIs and no need to be the RESTful so far.

//////////////////////////
// APIs for the CLI
//////////////////////////

const cliAPIPrefix = "/cli"

// the API pathes
const (
	TestPath = cliAPIPrefix + "/test"
	HintPath = cliAPIPrefix + "/hint"
)

// TestRequest represents the input data to the test API.
type TestRequest struct {
	Path      string `json:"path"`
	Begin     int    `json:"begin"`
	End       int    `json:"end"`
	Parallel  string `json:"parallel"`
	BuildTags string `json:"build_tags"`
}

// valid parallel values
const (
	ParallelOn   = "on"
	ParallelOff  = "off"
	ParallelAuto = "auto"
)

// HintRequest represents the input data to the hint API.
type HintRequest struct {
	Path  string `json:"path"`
	Begin int    `json:"begin"`
	End   int    `json:"end"`
}
