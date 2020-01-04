package common

// Assume one API for each cli command and no need to be RESTful so far.

// TestPath represents the path of the test API.
const TestPath = "/test"

// TestRequest represents the input data of the test API.
type TestRequest struct {
	Path string `json:"path"`
}
