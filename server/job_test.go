package server

import (
	"context"
	"go/ast"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ks888/noisegate/common/log"
)

func TestMain(m *testing.M) {
	dir, err := ioutil.TempDir("", "noisegate")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	SetUpSharedDir(dir)

	log.EnableDebugLog(true)

	os.Exit(m.Run())
}

func TestNewJob(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "typical")

	job, err := NewJob(dirPath, []change{{filepath.Join(dirPath, "sum.go"), 0, 0}}, "", false, nil)
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
		{TestFunction: "TestSum", Job: job},
		{TestFunction: "TestSum_ErrorCase", Job: job},
		{TestFunction: "TestSum_Add1", Job: job},
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
	_, err := NewJob(dirPath, []change{{filepath.Join(dirPath, "sum.go"), 0, 0}}, "", false, nil)
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
				job, err := NewJob(dirPath, []change{{filepath.Join(dirPath, "README.md"), 0, 0}}, "", false, nil)
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

	job, err := NewJob(dirPath, []change{{filepath.Join(dirPath, "sum.go"), 63, 63}}, "example", false, nil)
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}
	if job.BuildTags != "example" {
		t.Errorf("wrong bulid tags: %s", job.BuildTags)
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

func TestJob_StartAndWait(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "typical")

	job, err := NewJob(dirPath, []change{{filepath.Join(dirPath, "sum.go"), 0, 0}}, "", false, nil)
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}

	job.Start(context.Background())
	job.Wait()
	if job.Status != JobStatusSuccessful {
		t.Errorf("wrong status: %v", job.Status)
	}
	<-job.jobFinishedCh
}

func TestJob_ChangedIdentityNames(t *testing.T) {
	j := &Job{influences: []influence{{from: defaultIdentity{ast.NewIdent("FuncA")}}, {from: defaultIdentity{ast.NewIdent("FuncB")}}}}
	names := j.ChangedIdentityNames()
	if !reflect.DeepEqual([]string{"FuncA", "FuncB"}, names) {
		t.Errorf("wrong list: %#v", names)
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
	if set.Worker == nil {
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
