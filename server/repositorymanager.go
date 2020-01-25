package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ks888/hornet/common/log"
)

// RepositoryManager manages the repositories the hornetd monitors.
type RepositoryManager struct {
	repos map[string]*SyncedRepository
	mtx   sync.Mutex
}

// NewRepositoryManager initializes the repository manager and returns its object.
func NewRepositoryManager() *RepositoryManager {
	return &RepositoryManager{repos: make(map[string]*SyncedRepository)}
}

// Watch watches the repository to which the specified `path` belongs.
// If `sync` is true, the repository is synced on background (that is, the method doesn't block).
// No-op if the repository is already watched.
func (m *RepositoryManager) Watch(path string, sync bool) error {
	srcPath := m.srcPath(path)

	addRepo := func() (*SyncedRepository, error) {
		m.mtx.Lock()
		defer m.mtx.Unlock()

		if _, ok := m.repos[srcPath]; ok {
			return nil, nil
		}

		repo := NewSyncedRepository(srcPath)
		repo.Lock(nil) // prevent other go routines touch the repo before sync.
		m.repos[srcPath] = repo
		return repo, nil
	}
	repo, err := addRepo()
	if err != nil {
		return err
	} else if repo == nil {
		// already watched
		return nil
	}

	go func() {
		defer repo.Unlock()

		if !sync {
			return
		}
		if err := repo.SyncInLock(); err != nil {
			log.Printf("failed to sync: %v", err)
		}
	}()
	return nil
}

func (m *RepositoryManager) srcPath(path string) string {
	src := findRepoRoot(path)
	if !strings.HasSuffix(src, string(filepath.Separator)) {
		src += string(filepath.Separator)
	}
	return src
}

// Find finds the repository to which the specified `path` belongs.
func (m *RepositoryManager) Find(path string) (*SyncedRepository, bool) {
	srcPath := m.srcPath(path)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	repo, ok := m.repos[srcPath]
	return repo, ok
}

// SyncedRepository represents the repository which is shared among all the workers.
// The repository is copied from the original repository.
type SyncedRepository struct {
	srcPath               string
	destPath              string
	destPathFromSharedDir string
	mtx                   sync.Mutex
	cond                  *sync.Cond
	used                  bool
	usedBy                *Job
}

// NewSyncedRepository returns the initialized SyncedRepository object.
// if `srcPath` ends with `/`, the contents of the `srcPath` is copied.
// Otherwise, the `srcPath` directory itself is also copied.
func NewSyncedRepository(srcPath string) *SyncedRepository {
	destPathFromSharedDir := filepath.Join("src", srcPath)
	destPath := filepath.Join(sharedDir, destPathFromSharedDir)
	repo := &SyncedRepository{
		srcPath:               srcPath,
		destPath:              destPath,
		destPathFromSharedDir: destPathFromSharedDir,
	}
	repo.cond = sync.NewCond(&repo.mtx)

	return repo
}

// SyncInLock copies the sources from the original repository.
// The caller of this function must lock the repository.
func (r *SyncedRepository) SyncInLock() error {
	if err := os.MkdirAll(r.destPath, os.ModePerm); err != nil {
		return err
	}

	// The quick experiment using 200MB size repository shows the `-z` option is not necessary.
	cmd := exec.Command("rsync", "-a", "--delete", r.srcPath, r.destPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to sync dir: %v\nlog:\n%s", err, string(out))
	}
	return nil
}

// Lock locks the repository. The `job` arg is just the information and may be nil.
// Lock blocks if the repository is already locked.
func (r *SyncedRepository) Lock(job *Job) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	for r.used {
		r.cond.Wait()
	}
	r.used = true
	r.usedBy = job
}

// Unlock unlocks the repository.
func (r *SyncedRepository) Unlock() {
	r.mtx.Lock()
	defer func() {
		r.mtx.Unlock()
		r.cond.Broadcast()
	}()
	r.used = false
	r.usedBy = nil
}
