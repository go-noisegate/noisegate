package server

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestWorker_Start(t *testing.T) {
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
			startErr:       true,
			waitErr:        false,
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
			t.Fatalf("[%d] %v", i, err)
		}
		if err != nil {
			continue
		}

		_, err = w.Wait()
		if (err != nil) != testCase.waitErr {
			t.Fatalf("[%d] %v", i, err)
		}
	}
}
