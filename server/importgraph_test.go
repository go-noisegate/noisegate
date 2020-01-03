package server

import (
	"go/build"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBuildImportGraph(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filename))
	ctxt := &build.Default
	importGraph := BuildImportGraph(ctxt, root)
	if importGraph.Root != root {
		t.Errorf("wrong root: %s", importGraph.Root)
	}

	toPkg, err := ctxt.Import("github.com/ks888/hornet/common/log", "", build.IgnoreVendor)
	if err != nil {
		t.Fatalf("failed to import dir: %v", err)
	}
	fromPkg, err := ctxt.Import("github.com/ks888/hornet/server", "", build.IgnoreVendor)
	if err != nil {
		t.Fatalf("failed to import dir: %v", err)
	}
	findExpectedEdge := false
	for _, p := range importGraph.Inbounds[toPkg.Dir] {
		if p == fromPkg.Dir {
			findExpectedEdge = true
		}
	}
	if !findExpectedEdge {
		t.Errorf("do not have the expected edge: %v <- %s", importGraph.Inbounds[toPkg.Dir], fromPkg.Dir)
	}
}
