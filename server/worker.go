package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ks888/hornet/common/log"
)

type Worker struct {
	ctx                                  context.Context
	testFuncs                            []string
	testBinaryPath, packagePath, logPath string
	testResultCh                         chan TestResult
	cmd                                  *exec.Cmd
}

// NewWorker returns the worker which executes the task set.
func NewWorker(ctx context.Context, job *Job, taskSet *TaskSet) *Worker {
	var testFuncs []string
	for _, t := range taskSet.Tasks {
		testFuncs = append(testFuncs, t.TestFunction)
	}

	return &Worker{
		ctx:            ctx,
		testFuncs:      testFuncs,
		testBinaryPath: job.TestBinaryPath,
		packagePath:    job.Package.path,
		logPath:        taskSet.LogPath,
		testResultCh:   job.testResultCh,
	}
}

// Start starts the new test.
func (w *Worker) Start() error {
	if len(w.testFuncs) == 0 {
		return nil
	}

	w.cmd = exec.CommandContext(w.ctx, "go", "tool", "test2json", w.testBinaryPath, "-test.v", "-test.run", "^"+strings.Join(w.testFuncs, "$|^")+"$")
	w.cmd.Dir = w.packagePath

	logFile, err := os.OpenFile(w.logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open the log file %s: %w\n", w.logPath, err)
	}
	writer := io.MultiWriter(newTestOutputWriter(w.testResultCh), logFile)
	w.cmd.Stdout = writer
	w.cmd.Stderr = writer
	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start the test: %w", err)
	}
	return nil
}

// Wait waits until the test finishes.
func (w *Worker) Wait() (bool, error) {
	if w.cmd == nil {
		return true, nil
	}
	err := w.cmd.Wait()
	return err == nil, err
}

type event struct {
	Action  string
	Test    string
	Elapsed float64
	Output  string
}

type testOutputWriter struct {
	buff         []byte
	runningTests map[string][]string
	resultCh     chan TestResult
}

func newTestOutputWriter(resultCh chan TestResult) *testOutputWriter {
	return &testOutputWriter{
		runningTests: make(map[string][]string),
		resultCh:     resultCh,
	}
}

// Write writes the data to the buffer, parses its contents line by line and send the test result if the result is finalized.
// If the line is not json format, it is ignored.
func (w *testOutputWriter) Write(p []byte) (n int, err error) {
	w.buff = append(w.buff, p...)
	n = len(p)

	for {
		advance, line, err := bufio.ScanLines(w.buff, false)
		if err != nil {
			return n, err
		} else if advance == 0 {
			break
		}
		w.buff = w.buff[advance:]

		ev := &event{}
		if err := json.Unmarshal([]byte(line), ev); err != nil {
			log.Printf("test output is not json (%s): %v\n", line, err)
			return n, nil
		}

		w.handleEvent(ev)
	}
	return n, nil
}

func (w *testOutputWriter) handleEvent(ev *event) {
	if ev.Test == "" {
		return
	}

	chunks := strings.SplitN(ev.Test, "/", 2)
	if len(chunks) == 2 {
		// merge the output to the parent test
		if ev.Action == "output" {
			parentTest := chunks[0]
			w.runningTests[parentTest] = append(w.runningTests[parentTest], ev.Output)
		}
		return
	}

	switch ev.Action {
	case "run":
		w.runningTests[ev.Test] = []string{}
	case "pause", "cont":
		// do nothing
	case "output":
		w.runningTests[ev.Test] = append(w.runningTests[ev.Test], ev.Output)
	case "pass", "fail", "skip", "bench":
		res := TestResult{TestName: ev.Test, Successful: ev.Action != "fail", ElapsedTime: time.Duration(ev.Elapsed * 1000 * 1000 * 1000), Output: w.runningTests[ev.Test]}
		w.resultCh <- res
		delete(w.runningTests, ev.Test)
	}
}
