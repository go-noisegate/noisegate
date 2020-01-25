package server

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestRepositoryManager_Watch(t *testing.T) {
	m := NewRepositoryManager()
	if err := m.Watch(".", true); err != nil {
		t.Fatalf("failed to watch repo: %v", err)
	}

	repo, ok := m.Find(".")
	if !ok {
		t.Fatalf("repo not found")
	}
	defer os.RemoveAll(repo.destPath)

	// wait the sync to be finished
	repo.Lock(nil)
	repo.Unlock()

	if _, err := os.Stat(repo.destPath); os.IsNotExist(err) {
		t.Errorf("dest path not exist: %s", repo.destPath)
	}
	readme := filepath.Join(repo.destPath, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		t.Errorf("file not exist: %s", readme)
	}
}

func TestRepositoryManager_SkipAlreadyWatchedRepo(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	m := NewRepositoryManager()
	if err := m.Watch(".", true); err != nil {
		t.Fatalf("failed to watch repo: %v", err)
	}
	repo1st, _ := m.Find(".")

	if err := m.Watch(".", true); err != nil {
		t.Fatalf("failed to watch repo: %v", err)
	}
	repo2nd, _ := m.Find(".")
	if repo1st != repo2nd {
		t.Errorf("repository is updated")
	}
}

func TestRepositoryManager_Watch_NotSync(t *testing.T) {
	m := NewRepositoryManager()
	if err := m.Watch(".", false); err != nil {
		t.Fatalf("failed to watch repo: %v", err)
	}

	repo, ok := m.Find(".")
	if !ok {
		t.Fatalf("repo not found")
	}
	defer os.RemoveAll(repo.destPath)

	// wait the background go routine to be finished
	repo.Lock(nil)
	repo.Unlock()

	if _, err := os.Stat(repo.destPath); os.IsNotExist(err) {
		t.Errorf("dest path not exist: %s", repo.destPath)
	}
	readme := filepath.Join(repo.destPath, "README.md")
	if _, err := os.Stat(readme); !os.IsNotExist(err) {
		t.Errorf("file exists: %s", readme)
	}
}

func TestSyncedRepository_SyncInLock(t *testing.T) {
	repo := NewSyncedRepository(filepath.Join("testdata", "repositorymanager") + string(filepath.Separator))
	defer os.RemoveAll(repo.destPath)
	repo.Lock(nil)

	if err := repo.SyncInLock(); err != nil {
		t.Fatalf("sync error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.destPath, "sum.go")); os.IsNotExist(err) {
		t.Errorf("sum.go not exist")
	}
	repo.Unlock()
}

func TestSyncedRepository_LockAndUnlock(t *testing.T) {
	repo := NewSyncedRepository(filepath.Join("testdata", "repositorymanager") + string(filepath.Separator))

	job := &Job{}
	repo.Lock(job)
	if !repo.used {
		t.Errorf("used is false")
	}
	if repo.usedBy != job {
		t.Errorf("invalid job: %v", repo.usedBy)
	}

	repo.Unlock()
	if repo.used {
		t.Errorf("used is true")
	}
	if repo.usedBy != nil {
		t.Errorf("invalid job: %v", repo.usedBy)
	}
}

func TestSyncedRepository_ConcurrentUse(t *testing.T) {
	repo := NewSyncedRepository(filepath.Join("testdata", "repositorymanager") + string(filepath.Separator))

	const numGoRoutines = 10
	errCh := make(chan error)
	var sharedValue int64
	for i := 0; i < numGoRoutines; i++ {
		go func() {
			repo.Lock(nil)

			if atomic.LoadInt64(&sharedValue) != 0 {
				errCh <- errors.New("shared value is not 0")
				return
			}
			atomic.StoreInt64(&sharedValue, 1)
			time.Sleep(time.Millisecond)
			atomic.StoreInt64(&sharedValue, 0)

			repo.Unlock()
			errCh <- nil
		}()
	}

	for i := 0; i < numGoRoutines; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent use error: %v", err)
		}
	}
}
