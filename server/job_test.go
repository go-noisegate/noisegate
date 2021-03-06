package server

import (
	"context"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-noisegate/noisegate/common/log"
)

func TestMain(m *testing.M) {
	log.EnableDebugLog(true)

	os.Exit(m.Run())
}

func TestNewJob(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "typical")

	job, err := NewJob(dirPath, false, []Change{{filepath.Join(dirPath, "sum.go"), 0, 0}}, nil, &strings.Builder{})
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}
	if job.ID == 0 {
		t.Errorf("id is 0")
	}
	if job.Status != JobStatusCreated {
		t.Errorf("wrong status: %v", job.Status)
	}

	expectedTasks := []Task{
		{TestFunction: "TestSum"},
		{TestFunction: "TestSum_ErrorCase"},
		{TestFunction: "TestSum_Add1"},
	}
	if len(expectedTasks) != len(job.Tasks) {
		t.Errorf("invalid number of tasks: %d, %#v", len(job.Tasks), job.Tasks)
	}
	for i, actualTask := range job.Tasks {
		if !reflect.DeepEqual(expectedTasks[i], *actualTask) {
			t.Errorf("wrong task: %#v", actualTask)
		}
	}
}

func TestNewJob_WithBypass(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "typical")

	job, err := NewJob(dirPath, true, []Change{}, nil, &strings.Builder{})
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}

	expectedTasks := []Task{
		{TestFunction: "TestSum", Important: true},
		{TestFunction: "TestSum_ErrorCase", Important: true},
		{TestFunction: "TestSum_Add1", Important: true},
	}
	if len(expectedTasks) != len(job.Tasks) {
		t.Errorf("invalid number of tasks: %d, %#v", len(job.Tasks), job.Tasks)
	}
	for i, actualTask := range job.Tasks {
		if !reflect.DeepEqual(expectedTasks[i], *actualTask) {
			t.Errorf("wrong task: %#v", actualTask)
		}
	}
}

func TestNewJob_InvalidDirPath(t *testing.T) {
	dirPath := "/not/exist/dir"
	_, err := NewJob(dirPath, false, []Change{{filepath.Join(dirPath, "sum.go"), 0, 0}}, nil, &strings.Builder{})
	if err == nil {
		t.Fatalf("err should not be nil: %v", err)
	}
}

func TestNewJob_UniqueIDCheck(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "no_go_files")

	ch := make(chan int64)
	numGoRoutines := 10
	numIter := 10
	for i := 0; i < numGoRoutines; i++ {
		go func() {
			for j := 0; j < numIter; j++ {
				job, err := NewJob(dirPath, false, []Change{{filepath.Join(dirPath, "README.md"), 0, 0}}, nil, &strings.Builder{})
				if err != nil {
					panic(err)
				}
				ch <- job.ID
			}
		}()
	}

	usedIDs := make(map[int64]struct{})
	for i := 0; i < numGoRoutines*numIter; i++ {
		usedID := <-ch
		if _, ok := usedIDs[usedID]; ok {
			t.Errorf("duplicate id: %d", usedID)
		}
		usedIDs[usedID] = struct{}{}
	}
}

func TestNewJob_WithBuildTags(t *testing.T) {
	currDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	dirPath := filepath.Join(currDir, "testdata", "buildtags")

	job, err := NewJob(dirPath, false, []Change{{filepath.Join(dirPath, "sum.go"), 63, 63}}, []string{"-tags", "example"}, &strings.Builder{})
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}
	if !reflect.DeepEqual(job.GoTestOptions, []string{"-tags", "example"}) {
		t.Errorf("wrong bulid tags: %s", job.GoTestOptions)
	}
	if len(job.influences) != 1 {
		t.Fatalf("wrong # of influences: %v", len(job.influences))
	}
	if job.influences[0].from.Name() != "Sum" {
		t.Errorf("wrong influence from: %s", job.influences[0].from.Name())
	}
	if len(job.influences[0].to) != 1 {
		t.Errorf("wrong influence to: %d", len(job.influences[0].to))
	}
	if _, ok := job.influences[0].to["TestSum"]; !ok {
		t.Errorf("wrong influence to: TestSum not exist")
	}
}

func TestJob_Run(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "typical")

	job, err := NewJob(dirPath, false, []Change{{filepath.Join(dirPath, "sum.go"), 0, 0}}, nil, &strings.Builder{})
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}
	job.TaskSets = []*TaskSet{NewTaskSet(0, job), NewTaskSet(1, job)}

	job.Run(context.Background())
	if job.Status != JobStatusSuccessful {
		t.Errorf("wrong status: %v", job.Status)
	}
	if job.TaskSets[0].Status != TaskSetStatusSuccessful {
		t.Errorf("wrong status: %v", job.Status)
	}
	if job.TaskSets[1].Status != TaskSetStatusSuccessful {
		t.Errorf("wrong status: %v", job.Status)
	}
}

func TestJob_ChangedIdentityNames(t *testing.T) {
	j := &Job{influences: []influence{{from: defaultIdentity{ast.NewIdent("FuncA")}}, {from: defaultIdentity{ast.NewIdent("FuncB")}}}}
	names := j.changedIdentityNames()
	if !reflect.DeepEqual([]string{"FuncA", "FuncB"}, names) {
		t.Errorf("wrong list: %#v", names)
	}
}

func TestFindOptionValue(t *testing.T) {
	if result := findOptionValue([]string{"-tags", "integration_test"}, "tags"); result != "integration_test" {
		t.Errorf("wrong result: %s", result)
	}

	if result := findOptionValue([]string{"--tags", "integration_test"}, "tags"); result != "integration_test" {
		t.Errorf("wrong result: %s", result)
	}

	if result := findOptionValue([]string{"-tags", "tag1,tag2"}, "tags"); result != "tag1,tag2" {
		t.Errorf("wrong result: %s", result)
	}

	if result := findOptionValue([]string{"-tags"}, "tags"); result != "" {
		t.Errorf("wrong result: %s", result)
	}

	if result := findOptionValue([]string{"-wrong-tags", "tag1"}, "tags"); result != "" {
		t.Errorf("wrong result: %s", result)
	}

	if result := findOptionValue([]string{"-v"}, "tags"); result != "" {
		t.Errorf("wrong result: %s", result)
	}

	if result := findOptionValue([]string{""}, "tags"); result != "" {
		t.Errorf("wrong result: %s", result)
	}
}

func TestTaskSet(t *testing.T) {
	set := NewTaskSet(1, &Job{ID: 1})
	if err := set.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if set.Status != TaskSetStatusStarted {
		t.Errorf("wrong status: %v", set.Status)
	}
	if set.StartedAt.IsZero() {
		t.Errorf("StartedAt is zero")
	}
	if set.worker == nil {
		t.Errorf("nil worker")
	}

	set.Wait()
	if set.Status != TaskSetStatusSuccessful {
		t.Errorf("wrong status: %v", set.Status)
	}
	if set.FinishedAt.IsZero() {
		t.Errorf("FinishedAt is 0")
	}
}
