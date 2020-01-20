package common

// These are just the internal APIs and no need to be the RESTful so far.

//////////////////////////
// APIs for the CLI
//////////////////////////

const cliAPIPrefix = "/cli"

// the API pathes
const (
	TestPath  = cliAPIPrefix + "/test"
	WatchPath = cliAPIPrefix + "/watch"
)

// TestRequest represents the input data to the test API.
type TestRequest struct {
	Path string `json:"path"`
}

// WatchRequest represents the input data to the watch API.
type WatchRequest struct {
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
	WorkerGroupName string `json:"worker_group_name"`
	WorkerID        int    `json:"worker_id"`
}

// NextTaskSetResponse represents the output data from the next task set API.
type NextTaskSetResponse struct {
	JobID         int64    `json:"job_id"`
	TaskSetID     int      `json:"task_set_id"`
	TestFunctions []string `json:"test_functions"`
	// The path from the shared dir
	LogPath string `json:"log_path"`
	// The path from the shared dir
	TestBinaryPath string `json:"test_binary_path"`
	// The path from the shared dir
	RepoArchivePath string `json:"repo_archive_path"`
	// The path from the repository root
	RepoToPackagePath string `json:"repo_to_package_path"`
}

// ReportResultRequest represents the input data to the report result API.
type ReportResultRequest struct {
	JobID      int64 `json:"job_id"`
	TaskSetID  int   `json:"task_set_id"`
	Successful bool  `json:"successful"`
}
