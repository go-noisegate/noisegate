package common

// These are just the internal APIs and no need to be the RESTful so far.

//////////////////////////
// APIs for the CLI
//////////////////////////

const cliAPIPrefix = "/cli"

// TestPath represents the path of the test API.
const TestPath = cliAPIPrefix + "/test"

// TestRequest represents the input data to the test API.
type TestRequest struct {
	Path string `json:"path"`
}

//////////////////////////
// APIs for the workers
//////////////////////////

const workersAPIPrefix = "/workers"

const (
	NextTaskSetPath  = workersAPIPrefix + "/next"
	ReportResultPath = workersAPIPrefix + "/report"
)

// NextTaskSetRequest represents the input data to the next task set API.
type NextTaskSetRequest struct {
	WorkerID int64 `json:"worker_id"`
}

// NextTaskSetResponse represents the output data from the next task set API.
type NextTaskSetResponse struct {
	JobID         int64    `json:"job_id"`
	TaskSetID     int      `json:"task_set_id"`
	TestFunctions []string `json:"test_functions"`
	// The abs path in the manager fs.
	DirPath string `json:"dir_path"`
	// The path from the shared dir
	TestBinaryPath string `json:"test_binary_path"`
	// The path from the shared dir
	RepoArchivePath string `json:"repo_archive_path"`
}

// ReportResultRequest represents the input data to the report result API.
type ReportResultRequest struct {
	JobID      int64 `json:"job_id"`
	TaskSetID  int   `json:"task_set_id"`
	Successful bool  `json:"successful"`
}
