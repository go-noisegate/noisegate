package server

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWorker_StartAndWait(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		// input
		LogPath, TestBinaryPath, PackagePath string
		Tasks                                []string
		// expect
		startErr bool
		waitErr  bool
	}{
		{
			LogPath:        filepath.Join(tempDir, "testlog"),
			TestBinaryPath: "echo",
			Tasks:          []string{"TestSum"},
			startErr:       false,
			waitErr:        false,
		},
		{
			LogPath:        filepath.Join(tempDir, "testlog"),
			PackagePath:    "/path/to/not/exist/file",
			TestBinaryPath: "echo",
			Tasks:          []string{"TestSum"},
			startErr:       true,
			waitErr:        false,
		},
		{
			LogPath:        filepath.Join(tempDir, "testlog"),
			TestBinaryPath: "cmd-not-exist",
			Tasks:          []string{"TestSum"},
			startErr:       false,
			waitErr:        true,
		},
		{
			LogPath:        filepath.Join(tempDir, "testlog"),
			TestBinaryPath: "echo",
			Tasks:          []string{},
			startErr:       false,
			waitErr:        false,
		},
	}

	for i, testCase := range testCases {
		w := &Worker{
			ctx:            context.Background(),
			testBinaryPath: testCase.TestBinaryPath,
			packagePath:    testCase.PackagePath,
			logPath:        testCase.LogPath,
			testFuncs:      testCase.Tasks,
		}
		err := w.Start()
		if (err != nil) != testCase.startErr {
			t.Errorf("[%d] %v", i, err)
		}
		if err != nil {
			continue
		}

		_, err = w.Wait()
		if (err != nil) != testCase.waitErr {
			t.Errorf("[%d] %v", i, err)
		}
	}
}

func TestWorker_CheckOutput(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	job := &Job{
		TestBinaryPath: filepath.Join("testdata", "worker", "output_pass"),
		Package:        &Package{},
		testResultCh:   make(chan TestResult, 1),
	}
	taskSet := &TaskSet{
		Tasks:   []*Task{{TestFunction: "Test"}},
		LogPath: filepath.Join(tempDir, "testlog"),
	}
	w := NewWorker(context.Background(), job, taskSet)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	passed, err := w.Wait()
	if err != nil || !passed {
		t.Fatalf("unexpected result: %v %v", passed, err)
	}

	result := <-job.testResultCh
	if result.TestName != "TestSum" {
		t.Errorf("unexpected test name: %s", result.TestName)
	}

	out, err := ioutil.ReadFile(taskSet.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `{"Action":"pass"}`) {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestWorker_SkipBuild(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	currDir, _ := os.Getwd()

	job := &Job{
		Package:        &Package{path: filepath.Join(currDir, "testdata")},
		testResultCh:   make(chan TestResult, 1),
		EnableParallel: false,
	}
	taskSet := &TaskSet{
		Tasks:   []*Task{{TestFunction: "TestSum"}},
		LogPath: filepath.Join(tempDir, "testlog"),
	}
	w := NewWorker(context.Background(), job, taskSet)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	passed, err := w.Wait()
	if err != nil || !passed {
		t.Fatalf("unexpected result: %v, %v", passed, err)
	}

	result := <-job.testResultCh
	if result.TestName != "TestSum" {
		t.Errorf("unexpected test name: %s", result.TestName)
	}
}

func TestTestOutputWriter(t *testing.T) {
	ch := make(chan TestResult, 1)
	w := newTestOutputWriter(ch)

	lines := []string{
		`{"Action":"run","Test":"TestSum"}`,
		`{"Action":"output","Test":"TestSum","Output":"=== RUN   TestSum\n"}`,
		`{"Action":"output","Test":"TestSum","Output":"--- PASS: TestSum (0.01s)\n"}`,
		`{"Action":"pass","Test":"TestSum","Elapsed":0.01}`,
		`{"Action":"output","Output":"PASS\n"}`,
		`{"Action":"output","Output":"ok \n"}`,
		`{"Action":"pass","Elapsed":0.01}`,
	}
	for _, line := range lines {
		_, err := w.Write([]byte(line + "\n"))
		if err != nil {
			t.Fatal(err)
		}
	}

	result := <-ch
	if result.TestName != "TestSum" {
		t.Errorf("wrong test name: %s", result.TestName)
	}
	if !result.Successful {
		t.Error("not successful")
	}
	if result.ElapsedTime != 10*time.Millisecond {
		t.Errorf("wrong duration: %v", result.ElapsedTime)
	}
	if !reflect.DeepEqual([]string{"=== RUN   TestSum\n", "--- PASS: TestSum (0.01s)\n"}, result.Output) {
		t.Errorf("wrong output: %v", result.Output)
	}
}

func TestTestOutputWriter_WriteOneByte(t *testing.T) {
	ch := make(chan TestResult, 1)
	w := newTestOutputWriter(ch)

	out := strings.Join([]string{
		`{"Action":"run","Test":"TestSum"}`,
		`{"Action":"output","Test":"TestSum","Output":"=== RUN   TestSum\n"}`,
		`{"Action":"output","Test":"TestSum","Output":"--- PASS: TestSum (0.01s)\n"}`,
		`{"Action":"pass","Test":"TestSum","Elapsed":0.01}`,
		`{"Action":"output","Output":"PASS\n"}`,
		`{"Action":"output","Output":"ok \n"}`,
		`{"Action":"pass","Elapsed":0.01}`,
	}, "\n")

	for i := 0; i < len(out); i++ {
		_, err := w.Write([]byte{out[i]})
		if err != nil {
			t.Fatal(err)
		}
	}

	result := <-ch
	if result.TestName != "TestSum" {
		t.Errorf("wrong test name: %s", result.TestName)
	}
}

func TestTestOutputWriter_WriteAtOnce(t *testing.T) {
	ch := make(chan TestResult, 1)
	w := newTestOutputWriter(ch)

	out := strings.Join([]string{
		`{"Action":"run","Test":"TestSum"}`,
		`{"Action":"output","Test":"TestSum","Output":"=== RUN   TestSum\n"}`,
		`{"Action":"output","Test":"TestSum","Output":"--- PASS: TestSum (0.01s)\n"}`,
		`{"Action":"pass","Test":"TestSum","Elapsed":0.01}`,
		`{"Action":"output","Output":"PASS\n"}`,
		`{"Action":"output","Output":"ok \n"}`,
		`{"Action":"pass","Elapsed":0.01}`,
	}, "\n")

	_, err := w.Write([]byte(out))
	if err != nil {
		t.Fatal(err)
	}

	result := <-ch
	if result.TestName != "TestSum" {
		t.Errorf("wrong test name: %s", result.TestName)
	}
}

func TestTestOutputWriter_MergeInnerTest(t *testing.T) {
	ch := make(chan TestResult, 1)
	w := newTestOutputWriter(ch)

	lines := []string{
		`{"Action":"run","Test":"TestSum"}`,
		`{"Action":"output","Test":"TestSum","Output":"=== RUN   TestSum\n"}`,
		`{"Action":"run","Test":"TestSum/Case1"}`,
		`{"Action":"output","Test":"TestSum/Case1","Output":"=== RUN   TestSum/Case1\n"}`,

		`{"Action":"output","Test":"TestSum","Output":"--- PASS: TestSum (0.01s)\n"}`,
		`{"Action":"output","Test":"TestSum/Case1","Output":"    --- PASS: TestSum/Case1 (0.00s)\n"}`,
		`{"Action":"pass","Test":"TestSum/Case1","Elapsed":0}`,
		`{"Action":"pass","Test":"TestSum","Elapsed":0.01}`,
	}
	for _, line := range lines {
		_, err := w.Write([]byte(line + "\n"))
		if err != nil {
			t.Fatal(err)
		}
	}

	result := <-ch
	if result.TestName != "TestSum" {
		t.Errorf("wrong test name: %s", result.TestName)
	}
	if !result.Successful {
		t.Error("not successful")
	}
	if result.ElapsedTime != 10*time.Millisecond {
		t.Errorf("wrong duration: %v", result.ElapsedTime)
	}
	if !reflect.DeepEqual([]string{"=== RUN   TestSum\n", "=== RUN   TestSum/Case1\n", "--- PASS: TestSum (0.01s)\n", "    --- PASS: TestSum/Case1 (0.00s)\n"}, result.Output) {
		t.Errorf("wrong output: %v", result.Output)
	}
}

func TestTestOutputWriter_InvalidJSON(t *testing.T) {
	ch := make(chan TestResult, 1)
	f := newTestOutputWriter(ch)

	_, err := f.Write([]byte("not json\n"))
	if err != nil {
		t.Fatal(err)
	}
}
