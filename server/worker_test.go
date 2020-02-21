package server

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			testEventCh:    make(chan TestEvent, 64),
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
		testEventCh:    make(chan TestEvent, 64),
		EnableParallel: true,
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

	ev := <-job.testEventCh
	if ev.Test != "TestSum" {
		t.Errorf("unexpected test name: %s", ev.Test)
	}

	out, err := ioutil.ReadFile(taskSet.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `{"Action":"pass"}`) {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestWorker_ParallelDisabled(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	currDir, _ := os.Getwd()

	job := &Job{
		Package:        &Package{path: filepath.Join(currDir, "testdata")},
		testEventCh:    make(chan TestEvent, 64),
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

	ev := <-job.testEventCh
	if ev.Test != "TestSum" {
		t.Errorf("unexpected test name: %s", ev.Test)
	}
}

func TestTestOutputWriter(t *testing.T) {
	ch := make(chan TestEvent, 1)
	w := newTestOutputWriter(ch)

	_, err := w.Write([]byte(`{"Action":"run","Test":"TestSum"}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}

	ev := <-ch
	if ev.Test != "TestSum" {
		t.Errorf("wrong test name: %s", ev.Test)
	}
	if ev.Action != "run" {
		t.Errorf("wrong action: %s", ev.Action)
	}
}

func TestTestOutputWriter_WriteOneByte(t *testing.T) {
	ch := make(chan TestEvent, 1)
	w := newTestOutputWriter(ch)

	out := `{"Action":"run","Test":"TestSum"}` + "\n"
	for i := 0; i < len(out); i++ {
		_, err := w.Write([]byte{out[i]})
		if err != nil {
			t.Fatal(err)
		}
	}

	ev := <-ch
	if ev.Test != "TestSum" {
		t.Errorf("wrong test name: %s", ev.Test)
	}
}

func TestTestOutputWriter_WriteAtOnce(t *testing.T) {
	ch := make(chan TestEvent, 64)
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

	ev := <-ch
	if ev.Test != "TestSum" {
		t.Errorf("wrong test name: %s", ev.Test)
	}
	ev = <-ch
	if ev.Test != "TestSum" {
		t.Errorf("wrong test name: %s", ev.Test)
	}
}

func TestTestOutputWriter_InvalidJSON(t *testing.T) {
	ch := make(chan TestEvent, 1)
	f := newTestOutputWriter(ch)

	_, err := f.Write([]byte("not json\n"))
	if err != nil {
		t.Fatal(err)
	}
	ev := <-ch
	if ev.Action != "unknown" {
		t.Errorf("wrong action: %s", ev.Action)
	}
	if ev.Output != "not json\n" {
		t.Errorf("wrong output: %s", ev.Output)
	}
}
