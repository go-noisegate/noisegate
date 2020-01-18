package server

import (
	"go/build"
	"os"
	"path/filepath"
	"sync"

	"github.com/ks888/hornet/common/log"
)

// ImportGraph represents the dependencies among packages
type ImportGraph struct {
	Root string
	// The key is the package and the value is the list of packages which depend on the key package.
	// Note that all the packages are represented by the abs path.
	Inbounds map[string][]string
}

type edge struct {
	from, to string
}

// BuildImportGraph recursively visits the directories under root and builds the import graph.
// Vendor directory is ignored.
func BuildImportGraph(ctxt *build.Context, root string) ImportGraph {
	importGraph := ImportGraph{
		Root:     root,
		Inbounds: make(map[string][]string),
	}
	depCh := make(chan edge)

	go func() {
		var wg sync.WaitGroup
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("failed to walk the path %s: %v", path, err)
				return nil
			}
			if !info.IsDir() {
				return nil
			}
			if shouldSkipDir(path) {
				return filepath.SkipDir
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				findImports(ctxt, path, depCh)
			}()
			return nil
		})

		wg.Wait()
		close(depCh)
	}()

	for dep := range depCh {
		importGraph.Inbounds[dep.to] = append(importGraph.Inbounds[dep.to], dep.from)
	}
	return importGraph
}

func shouldSkipDir(path string) bool {
	base := filepath.Base(path)
	if base[0] == '.' || base[0] == '_' || base == "testdata" || base == "vendor" {
		return true
	}
	return false
}

func findImports(ctxt *build.Context, path string, ch chan edge) {
	pkg, err := ctxt.ImportDir(path, 0)
	if err != nil {
		if _, ok := err.(*build.NoGoError); !ok {
			log.Printf("failed to import %s: %v", path, err)
		}
		return
	}

	for _, imp := range pkg.Imports {
		ch <- edge{from: path, to: importPathToAbsPath(ctxt, imp, pkg.Dir)}
	}
	for _, imp := range pkg.TestImports {
		ch <- edge{from: path, to: importPathToAbsPath(ctxt, imp, pkg.Dir)}
	}
	for _, imp := range pkg.XTestImports {
		ch <- edge{from: path, to: importPathToAbsPath(ctxt, imp, pkg.Dir)}
	}
}

func importPathToAbsPath(ctxt *build.Context, importPath, srcDir string) string {
	impPkg, err := ctxt.Import(importPath, srcDir, build.IgnoreVendor)
	if err != nil {
		log.Printf("failed to import %s (srcDir: %s): %v", importPath, srcDir, err)
	}
	return impPkg.Dir
}
