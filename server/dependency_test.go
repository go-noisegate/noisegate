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

const (
	// sum.go
	FuncSumDeclBegin           = 35
	FuncSumBodyEnd             = 75
	FuncNestedSumNestedFuncEnd = 151
	VarV1DeclBegin             = 177
	VarV1DeclEnd               = 191
	VarsDeclBegin              = 193
	VarsDeclEnd                = 224
	ConstC1DeclBegin           = 226
	ConstC1DeclEnd             = 238
	TypeT1DeclBegin            = 240
	TypeT1DeclEnd              = 265
	FuncT1IncDeclBegin         = 267
	FuncT1IncBodyBegin         = 304
	FuncT1IncBodyEnd           = 311
	FuncT1DecDeclBegin         = 358
	FuncT1DecBodyEnd           = 403
	MethodCalcSumBodyBegin     = 555
	FuncXSumBodyBegin          = 626
	// sum_test.go
	FuncTestSumBodyBegin              = 110
	TypeExampleTestSuiteDeclBegin     = 269
	FuncTestExampleBodyBegin          = 363
	FuncSetupTestBodyBegin            = 422
	FuncTestExampleTestSuiteBodyBegin = 467
)

func TestFindInfluencedTests_Function(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum.go", FuncSumDeclBegin, FuncSumDeclBegin}})
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
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum_test.go", FuncTestSumBodyBegin, FuncTestSumBodyBegin}})
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

func TestFindInfluencedTests_TestSuiteFunction(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum_test.go", FuncTestExampleBodyBegin, FuncTestExampleBodyBegin}})
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

func TestFindInfluencedTests_TestSuiteType(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum_test.go", TypeExampleTestSuiteDeclBegin, TypeExampleTestSuiteDeclBegin}})
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

func TestFindInfluencedTests_TestSuiteSetup(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum_test.go", FuncSetupTestBodyBegin, FuncSetupTestBodyBegin}})
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

func TestFindInfluencedTests_Interface(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum.go", MethodCalcSumBodyBegin, MethodCalcSumBodyBegin}})
	if err != nil {
		t.Fatal(err)
	}
	if len(influences) != 1 {
		t.Fatalf("wrong # of influences: %d", len(influences))
	}
	if len(influences[0].to) != 1 {
		t.Fatalf("wrong # of funcs: %d", len(influences[0].to))
	}
	if _, ok := influences[0].to["TestCalculatorSum"]; !ok {
		t.Errorf("no expected func: %#v", influences[0].to)
	}
}

func TestFindInfluencedTests_XTestPackage(t *testing.T) {
	cwd, _ := os.Getwd()
	dirPath := filepath.Join(cwd, "testdata", "dependency")
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum.go", FuncXSumBodyBegin, FuncXSumBodyBegin}})
	if err != nil {
		t.Fatal(err)
	}
	if len(influences) != 1 {
		t.Fatalf("wrong # of influences: %d", len(influences))
	}
	if len(influences[0].to) != 1 {
		t.Fatalf("wrong # of funcs: %d", len(influences[0].to))
	}
	if _, ok := influences[0].to["TestXSum"]; !ok {
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

func TestFindInfluencedTests_NoGoFile(t *testing.T) {
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
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum.go", FuncSumDeclBegin, FuncSumDeclBegin}, {"sum.go", FuncT1IncBodyBegin, FuncT1IncBodyBegin}})
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
	influences, err := findInfluencedTests(&build.Default, dirPath, []Change{{"sum_test.go", 0, FuncTestExampleTestSuiteBodyBegin}})
	if err != nil {
		t.Fatal(err)
	}
	if len(influences) != 5 {
		t.Fatalf("wrong # of influences: %d", len(influences))
	}
	for i, from := range []string{"TestSum", "ExampleTestSuite", "ExampleTestSuite.TestExample", "ExampleTestSuite.SetupTest", "TestExampleTestSuite"} {
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

	begin, end := int64(0), int64(FuncSumDeclBegin)
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

	begin, end := int64(FuncSumDeclBegin), int64(FuncSumBodyEnd)
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

	id, err := pkg.findEnclosingIdentity("sum.go", FuncNestedSumNestedFuncEnd)
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

	begin, end := int64(VarV1DeclBegin), int64(VarV1DeclEnd)
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

	begin, end := int64(VarsDeclBegin), int64(VarsDeclEnd)
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

	begin, end := int64(ConstC1DeclBegin), int64(ConstC1DeclEnd)
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

	begin, end := int64(TypeT1DeclBegin), int64(TypeT1DeclEnd)
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

	begin, end := int64(FuncT1IncDeclBegin), int64(FuncT1IncBodyEnd)
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

	begin, end := int64(FuncT1DecDeclBegin), int64(FuncT1DecBodyEnd)
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

	id, _ := pkg.findEnclosingIdentity("sum.go", FuncSumDeclBegin)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 2 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	pos := pkg.fset.Position(users[0].NamePos)
	if !filepath.IsAbs(pos.Filename) || filepath.Base(pos.Filename) != "sum_test.go" || pos.Line != 10 {
		t.Errorf("wrong position: %#v", pos)
	}
}

func TestFindUsers_FuncUseVar(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	id, _ := pkg.findEnclosingIdentity("sum.go", VarV1DeclBegin)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 1 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	pos := pkg.fset.Position(users[0].NamePos)
	if !filepath.IsAbs(pos.Filename) || filepath.Base(pos.Filename) != "sum_test.go" || pos.Line != 13 {
		t.Errorf("wrong position: %#v", pos)
	}
}

func TestFindUsers_FuncUseConst(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	id, _ := pkg.findEnclosingIdentity("sum.go", ConstC1DeclBegin)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 1 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	pos := pkg.fset.Position(users[0].NamePos)
	if !filepath.IsAbs(pos.Filename) || filepath.Base(pos.Filename) != "sum_test.go" || pos.Line != 13 {
		t.Errorf("wrong position: %#v", pos)
	}
}

func TestFindUsers_FuncUseType(t *testing.T) {
	cwd, _ := os.Getwd()
	pkgPath := filepath.Join(cwd, "testdata", "dependency")
	pkg, _ := newParsedPackage(&build.Default, pkgPath)

	id, _ := pkg.findEnclosingIdentity("sum.go", TypeT1DeclBegin)
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
		{"sum_test.go", 14},
		{"sum_test.go", 17},
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

	id, _ := pkg.findEnclosingIdentity("sum.go", FuncT1IncDeclBegin)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 2 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	for i, expect := range []struct {
		line int
	}{{15}, {18}} {
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

	id, _ := pkg.findEnclosingIdentity("sum.go", FuncT1DecDeclBegin)
	users, err := pkg.findUsers(id)
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 2 {
		t.Fatalf("wrong # of users: %d", len(users))
	}
	for i, expect := range []struct {
		line int
	}{{16}, {19}} {
		pos := pkg.fset.Position(users[i].NamePos)
		if !filepath.IsAbs(pos.Filename) || filepath.Base(pos.Filename) != "sum_test.go" || pos.Line != expect.line {
			t.Errorf("wrong position: %#v", pos)
		}
	}
}
