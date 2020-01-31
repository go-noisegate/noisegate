package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/ks888/hornet/common/log"
)

// PackageManager manages the packages the hornetd monitors.
type PackageManager struct {
	pkgs map[string]*Package
	mtx  sync.Mutex
}

// NewPackageManager initializes the package manager and returns its object.
func NewPackageManager() *PackageManager {
	return &PackageManager{pkgs: make(map[string]*Package)}
}

// Watch watches the package to which the specified `path` belongs.
func (m *PackageManager) Watch(path string) error {
	path = m.findPath(path)

	m.mtx.Lock()
	defer m.mtx.Unlock()
	if _, ok := m.pkgs[path]; ok {
		return nil
	}

	m.pkgs[path] = &Package{path: path}
	return nil
}

func (m *PackageManager) findPath(path string) string {
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		log.Printf("%s not found", path)
		return path
	}
	if fi.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

// Find finds the package to which the specified `path` belongs.
func (m *PackageManager) Find(path string) (*Package, bool) {
	path = m.findPath(path)

	m.mtx.Lock()
	defer m.mtx.Unlock()

	pkg, ok := m.pkgs[path]
	return pkg, ok
}

// Package represents the go package.
type Package struct {
	path       string
	mtx        sync.Mutex
	cancelFunc context.CancelFunc
}

// Prebuild runs the build process for the preparation. If the pre-build process is already running,
// the process is killed.
// One point of this preparation is to compile dependent packages in advance.
func (p *Package) Prebuild() error {
	var ctx context.Context
	setup := func() {
		p.mtx.Lock()
		defer p.mtx.Unlock()

		if p.cancelFunc != nil {
			p.cancelFunc()
		}
		ctx, p.cancelFunc = context.WithCancel(context.Background())
	}
	setup()
	return p.buildContext(ctx, "/dev/null")
}

// Prebuild builds the package.
func (p *Package) Build(artifactPath string) error {
	return p.buildContext(context.Background(), artifactPath)
}

func (p *Package) buildContext(ctx context.Context, artifactPath string) error {
	cmd := exec.CommandContext(ctx, "go", "test", "-c", "-o", artifactPath, ".")
	cmd.Env = append(os.Environ(), "GOOS=linux")
	cmd.Dir = p.path

	buildLog, err := cmd.CombinedOutput()
	if err != nil {
		if patternNoGoFiles.Match(buildLog) {
			return errNoGoTestFiles
		}
		return fmt.Errorf("failed to build: %w\nbuild log:\n%s", err, string(buildLog))
	}

	if patternNoTestFiles.Match(buildLog) {
		return errNoGoTestFiles
	}
	return nil
}
