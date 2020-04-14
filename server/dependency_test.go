package server

import (
	"go/ast"
	"go/build"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestFindInfluencedTests_Function(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum.go", 35, 35}})
	if err != nil {
		t.Fatal(err)
	}
	if len(influences) != 1 {
		t.Fatalf("wrong # of influences: %d", len(influences))
	}
	if len(influences[0].to) != 2 {
		t.Fatalf("wrong # of funcs: %d", len(influences[0].to))
	}
	if _, ok := influences[0].to["TestSum"]; !ok {
		t.Errorf("no expected func")
	}
	if _, ok := influences[0].to["TestExampleTestSuite"]; !ok {
		t.Errorf("no expected func")
	}
}

func TestFindInfluencedTests_TestFunction(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum_test.go", 67, 67}})
	if err != nil {
		t.Fatal(err)
	}
	if len(influences) != 1 {
		t.Fatalf("wrong # of influences: %d", len(influences))
	}
	if len(influences[0].to) != 1 {
		t.Fatalf("wrong # of funcs: %d", len(influences[0].to))
	}
	if _, ok := influences[0].to["TestSum"]; !ok {
		t.Errorf("no expected func")
	}
}

func TestFindInfluencedTests_TestSuite(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum_test.go", 304, 304}})
	if err != nil {
		t.Fatal(err)
	}
	if len(influences) != 1 {
		t.Fatalf("wrong # of influences: %d", len(influences))
	}
	if len(influences[0].to) != 1 {
		t.Fatalf("wrong # of funcs: %d", len(influences[0].to))
	}
	if _, ok := influences[0].to["TestExampleTestSuite"]; !ok {
		t.Errorf("no expected func: %#v", influences[0].to)
	}
}

func TestFindInfluencedTests_IdentityNotFound(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum_test.go", 0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(influences) != 0 {
		t.Fatalf("wrong # of influences: %d", len(influences))
	}
}

func TestFindInfluencedTests_NoGoDirectory(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "no_go_files")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"README.md", 0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(influences) != 0 {
		t.Fatalf("wrong # of influences: %d", len(influences))
	}
}

func TestFindInfluencedTests_MultipleChanges(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum.go", 35, 35}, {"sum.go", 304, 304}})
	if err != nil {
		t.Fatal(err)
	}
	if len(influences) != 2 {
		t.Fatalf("wrong # of influences: %d", len(influences))
	}
	for i, from := range []string{"Sum", "T1.Inc"} {
		if influences[i].from.Name() != from {
			t.Errorf("wrong 'from': %s", influences[i].from.Name())
		}
	}
}

func TestFindInfluencedTests_ChangeWithRange(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum_test.go", 0, 362}})
	if err != nil {
		t.Fatal(err)
	}
	if len(influences) != 4 {
		t.Fatalf("wrong # of influences: %d", len(influences))
	}
	for i, from := range []string{"TestSum", "ExampleTestSuite", "ExampleTestSuite.TestExample", "TestExampleTestSuite"} {
		if influences[i].from.Name() != from {
			t.Errorf("wrong 'from': %s", influences[i].from.Name())
		}
	}
}

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
			if f.Name() == filename {
				found = true
				return false
			}
			return true
		})
		return found
	}

	if !findFilename(filepath.Join(pkgPath, "sum.go")) {
		t.Error("no sum.go in file set")
	}
	if !findFilename(filepath.Join(pkgPath, "sum_test.go")) {
		t.Error("no sum_test.go in file set")
	}
}

func TestFindEnclosingIdentity_PackageDecl(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	begin, end := int64(0), int64(35)
	for o := begin; o < end; o++ {
		id, err := pkg.findEnclosingIdentity("sum.go", o)
		if err != nil {
			t.Fatal(err)
		}
		if id != nil {
			t.Error(id)
		}
	}
}

func TestFindEnclosingIdentity_SimpleFunc(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	begin, end := int64(35), int64(75)
	for _, o := range []int64{begin - 1, end} {
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
		if id.Name() != "Sum" {
			t.Errorf("invalid identity: %s", id.Name())
		}
	}
}

func TestFindEnclosingIdentity_InvalidOffset(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	_, err := pkg.findEnclosingIdentity("sum.go", 1024*1024)
	if err == nil {
		t.Fatal("not nil")
	}
}

func TestFindEnclosingIdentity_NestedFunc(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	id, err := pkg.findEnclosingIdentity("sum.go", 151)
	if err != nil {
		t.Fatal(err)
	}
	if id.Name() != "NestedSum" {
		t.Errorf("invalid identity: %s", id.Name())
	}
}

func TestFindEnclosingIdentity_TopLevelVar(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	begin, end := int64(177), int64(191)
	for _, o := range []int64{begin - 1, end} {
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
		if id.Name() != "v1" {
			t.Errorf("invalid identity: %s", id.Name())
		}
	}
}

func TestFindEnclosingIdentity_TopLevelVarList(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	begin, end := int64(193), int64(224)
	for _, o := range []int64{begin - 1, end} {
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
		if id.Name() != "v2" {
			t.Errorf("invalid identity: %s", id.Name())
		}
	}
}

func TestFindEnclosingIdentity_TopLevelConst(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	begin, end := int64(226), int64(238)
	for _, o := range []int64{begin - 1, end} {
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
		if id.Name() != "c1" {
			t.Errorf("invalid identity: %s", id.Name())
		}
	}
}

func TestFindEnclosingIdentity_Type(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	begin, end := int64(240), int64(265)
	for _, o := range []int64{begin - 1, end} {
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
		if id.Name() != "T1" {
			t.Errorf("invalid identity: %s", id.Name())
		}
	}
}

func TestFindEnclosingIdentity_Method(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	begin, end := int64(267), int64(311)
	for _, o := range []int64{begin - 1, end} {
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
		if id.Name() != "T1.Inc" {
			t.Errorf("invalid identity: %s", id.Name())
		}
	}
}

func TestFindEnclosingIdentity_PointerReceiverMethod(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	begin, end := int64(358), int64(403)
	for _, o := range []int64{begin - 1, end} {
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
		if id.Name() != "T1.Dec" {
			t.Errorf("invalid identity: %s", id.Name())
		}
	}
}

func TestFindUsers_FuncUseFunc(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	id, _ := pkg.findEnclosingIdentity("sum.go", 35)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 2 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	pos := pkg.fset.Position(users[0].NamePos)
	if !filepath.IsAbs(pos.Filename) || filepath.Base(pos.Filename) != "sum_test.go" || pos.Line != 6 {
		t.Errorf("wrong position: %#v", pos)
	}
}

func TestFindUsers_FuncUseVar(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	id, _ := pkg.findEnclosingIdentity("sum.go", 177)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 1 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	pos := pkg.fset.Position(users[0].NamePos)
	if !filepath.IsAbs(pos.Filename) || filepath.Base(pos.Filename) != "sum_test.go" || pos.Line != 9 {
		t.Errorf("wrong position: %#v", pos)
	}
}

func TestFindUsers_FuncUseConst(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	id, _ := pkg.findEnclosingIdentity("sum.go", 226)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 1 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	pos := pkg.fset.Position(users[0].NamePos)
	if !filepath.IsAbs(pos.Filename) || filepath.Base(pos.Filename) != "sum_test.go" || pos.Line != 9 {
		t.Errorf("wrong position: %#v", pos)
	}
}

func TestFindUsers_FuncUseType(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	id, _ := pkg.findEnclosingIdentity("sum.go", 240)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 4 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	sortByPosition(pkg, users)

	for i, expect := range []struct {
		filename string
		line     int
	}{
		{"sum.go", 29},
		{"sum.go", 37},
		{"sum_test.go", 10},
		{"sum_test.go", 13},
	} {
		pos := pkg.fset.Position(users[i].NamePos)
		if !filepath.IsAbs(pos.Filename) || filepath.Base(pos.Filename) != expect.filename || pos.Line != expect.line {
			t.Errorf("wrong position: %v", pos)
		}
	}
}

func sortByPosition(pkg parsedPackage, ids []*ast.Ident) {
	sort.Slice(ids, func(i, j int) bool {
		posI := pkg.fset.Position(ids[i].NamePos)
		posJ := pkg.fset.Position(ids[j].NamePos)
		if posI.Filename == posJ.Filename {
			return posI.Line < posJ.Line
		}
		return posI.Filename < posJ.Filename
	})
}

func TestFindUsers_FuncUseMethod(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	id, _ := pkg.findEnclosingIdentity("sum.go", 267)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 2 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	for i, expect := range []struct {
		line int
	}{{11}, {14}} {
		pos := pkg.fset.Position(users[i].NamePos)
		if !filepath.IsAbs(pos.Filename) || filepath.Base(pos.Filename) != "sum_test.go" || pos.Line != expect.line {
			t.Errorf("wrong position: %#v", pos)
		}
	}
}

func TestFindUsers_FuncUsePointerReceiverMethod(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	id, _ := pkg.findEnclosingIdentity("sum.go", 358)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 2 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	for i, expect := range []struct {
		line int
	}{{12}, {15}} {
		pos := pkg.fset.Position(users[i].NamePos)
		if !filepath.IsAbs(pos.Filename) || filepath.Base(pos.Filename) != "sum_test.go" || pos.Line != expect.line {
			t.Errorf("wrong position: %#v", pos)
		}
	}
}
