package server

import (
	"go/build"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestNewParsedPackage(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, err := newParsedPackage(&build.Default, pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	findFilename := func(filename string) bool {
		var found bool
		pkg.fset.Iterate(func(f *token.File) bool {
			if filepath.Base(f.Name()) == filepath.Base(filename) {
				found = true
				return false
			}
			return true
		})
		return found
	}

	if !findFilename("sum.go") {
		t.Error("no sum.go in file set")
	}
	if !findFilename("sum_test.go") {
		t.Error("no sum_test.go in file set")
	}
}

func TestNewParsedPackage_NoGoFiles(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "no_go_files")
	_, err := newParsedPackage(&build.Default, pkgPath)
	if err == nil {
		t.Fatal("nil err")
	}
}

func TestFindEnclosingIdentity_SimpleFunc(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, err := newParsedPackage(&build.Default, pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	begin, end := 20, 60
	for _, o := range []int{begin - 1, end} {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if id != nil || err != nil {
			t.Errorf("wrong id (%d): %#v, %v", o, id, err)
		}
	}
	for o := begin; o < end; o++ {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if err != nil {
			t.Fatal(err)
		}
		if id.Name != "Sum" {
			t.Errorf("invalid identity: %s", id.Name)
		}
	}
}

func TestFindEnclosingIdentity_InvalidOffset(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, err := newParsedPackage(&build.Default, pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pkg.findEnclosingIdentity("sum.go", 1024*1024)
	if err == nil {
		t.Fatal("not nil")
	}
}

func TestFindEnclosingIdentity_NestedFunc(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, err := newParsedPackage(&build.Default, pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	id, err := pkg.findEnclosingIdentity("sum.go", 136)
	if err != nil {
		t.Fatal(err)
	}
	if id.Name != "NestedSum" {
		t.Errorf("invalid identity: %s", id.Name)
	}
}

func TestFindEnclosingIdentity_PackageDecl(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, err := newParsedPackage(&build.Default, pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	id, err := pkg.findEnclosingIdentity("sum.go", 1)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Error(id)
	}
}

func TestFindEnclosingIdentity_TopLevelVar(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, err := newParsedPackage(&build.Default, pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	begin, end := 162, 176
	for _, o := range []int{begin - 1, end} {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if id != nil || err != nil {
			t.Errorf("wrong id (%d): %#v, %v", o, id, err)
		}
	}
	for o := begin; o < end; o++ {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if err != nil {
			t.Fatal(err)
		}
		if id.Name != "v1" {
			t.Errorf("invalid identity: %s", id.Name)
		}
	}
}

func TestFindEnclosingIdentity_TopLevelVarList(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, err := newParsedPackage(&build.Default, pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	begin, end := 178, 209
	for _, o := range []int{begin - 1, end} {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if id != nil || err != nil {
			t.Errorf("wrong id (%d): %#v, %v", o, id, err)
		}
	}
	for o := begin; o < end; o++ {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if err != nil {
			t.Fatal(err)
		}
		if id.Name != "v2" {
			t.Errorf("invalid identity: %s", id.Name)
		}
	}
}

func TestFindEnclosingIdentity_TopLevelConst(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, err := newParsedPackage(&build.Default, pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	begin, end := 211, 223
	for _, o := range []int{begin - 1, end} {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if id != nil || err != nil {
			t.Errorf("wrong id (%d): %#v, %v", o, id, err)
		}
	}
	for o := begin; o < end; o++ {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if err != nil {
			t.Fatal(err)
		}
		if id.Name != "c1" {
			t.Errorf("invalid identity: %s", id.Name)
		}
	}
}

func TestFindEnclosingIdentity_Type(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, err := newParsedPackage(&build.Default, pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	begin, end := 225, 250
	for _, o := range []int{begin - 1, end} {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if id != nil || err != nil {
			t.Errorf("wrong id (%d): %#v, %v", o, id, err)
		}
	}
	for o := begin; o < end; o++ {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if err != nil {
			t.Fatal(err)
		}
		if id.Name != "T1" {
			t.Errorf("invalid identity: %s", id.Name)
		}
	}
}

func TestFindEnclosingIdentity_Method(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, err := newParsedPackage(&build.Default, pkgPath)
	if err != nil {
		t.Fatal(err)
	}

	begin, end := 252, 296
	for _, o := range []int{begin - 1, end} {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if id != nil || err != nil {
			t.Errorf("wrong id (%d): %#v, %v", o, id, err)
		}
	}
	for o := begin; o < end; o++ {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if err != nil {
			t.Fatal(err)
		}
		if id.Name != "Inc" {
			t.Errorf("invalid identity: %s", id.Name)
		}
	}
}
