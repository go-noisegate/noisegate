package common

import (
	"fmt"
	"strings"
)

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
	Bypass        bool     `json:"bypass"`
	Path          string   `json:"path"`
	GoTestOptions []string `json:"go_test_options"`
}

// HintRequest represents the input data to the hint API.
type HintRequest struct {
	Path   string  `json:"path"`
	Ranges []Range `json:"ranges"`
}

// Range represents the some range of the file.
type Range struct {
	Begin int64 `json:"begin"`
	End   int64 `json:"end"`
}

// RangesToQuery converts the specified ranges to the query.
func RangesToQuery(ranges []Range) string {
	var rs []string
	for _, r := range ranges {
		rs = append(rs, fmt.Sprintf("#%d-%d", r.Begin, r.End))
	}
	return strings.Join(rs, ",")
}
