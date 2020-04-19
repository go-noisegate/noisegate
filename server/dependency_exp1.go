// +build exp1

package server

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"path/filepath"
	"strings"

	"github.com/ks888/noisegate/common/log"
	"golang.org/x/tools/go/ast/astutil"
)

type influence struct {
	from identity
	to   map[string]struct{}
}

// findInfluencedTests finds the test functions which affected by the specified changes.
// summary:
// 1. Finds the top-level declaration which encloses the specified offset.
// 2-1. If the decl is the test function itself, returns the test function.
// 2-2. Otherwise, lists the test functions which use the identity step 1 finds. It only searches for the files in the same package.
//   For example, if the offset specifies the last line of some non-test function, it finds the name of the function (e.g. `Sum`) first,
//   and then finds the test functions which directly call the function (e.g. `TestSum1` and `TestSum2`).
// Note that if the found test function is a part of the test suite, the runner function of the test suite is returned.
func findInfluencedTests(ctxt *build.Context, dirPath string, changes []Change) ([]influence, error) {
	if len(changes) == 0 {
		return nil, nil
	}
	pkg, err := newParsedPackage(ctxt, dirPath)
	if err != nil {
		if _, ok := err.(*build.NoGoError); ok {
			return nil, nil
		}
		return nil, err
	}

	var ins []influence
	for _, ch := range changes {
		for offset := ch.Begin; offset <= ch.End; offset++ {
			in, err := pkg.findInfluence(ch.Basename, offset)
			if err != nil {
				log.Print(err)
				continue
			}
			if in.from != nil {
				ins = append(ins, in)
			}
		}
	}
	return ins, nil
}

type parsedPackage struct {
	pkgDir string
	pkg    *ast.Package
	fset   *token.FileSet
	info   *types.Info
	found  map[string]struct{}
}

// `packageDir` must be abs.
func newParsedPackage(ctxt *build.Context, packageDir string) (parsedPackage, error) {
	pkg, err := ctxt.ImportDir(packageDir, build.IgnoreVendor)
	if err != nil {
		return parsedPackage{}, err
	}

	filenames := pkg.GoFiles
	filenames = append(filenames, pkg.TestGoFiles...)
	filenames = append(filenames, pkg.XTestGoFiles...) // TODO: better XTest package support

	fset := token.NewFileSet()
	parsedFiles := make(map[string]*ast.File)
	var files []*ast.File // redundant, but types package needs this
	for _, file := range filenames {
		path := filepath.Join(packageDir, file)
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			log.Printf("failed to parse %s: %v\n", path, err)
		}
		if f != nil {
			parsedFiles[path] = f
			files = append(files, f)
		}
	}

	// NewPackage returns the error when there are unresolved identities, which is ignorable here.
	astPkg, _ := ast.NewPackage(fset, parsedFiles, nil, nil)

	info := types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	var conf types.Config
	conf.Error = func(err error) {
		// log.Debugf("type check error: %v", err) // too verbose and less important in our case
	}
	_, _ = conf.Check(astPkg.Name, fset, files, &info)
	return parsedPackage{pkgDir: packageDir, pkg: astPkg, fset: fset, info: &info, found: make(map[string]struct{})}, nil
}

func (p parsedPackage) findInfluence(filename string, offset int64) (influence, error) {
	id, err := p.findEnclosingIdentity(filename, offset)
	if err != nil {
		return influence{}, err
	}
	if id == nil {
		return influence{}, nil
	}

	if _, ok := p.found[id.Name()]; ok {
		return influence{}, nil
	}
	p.found[id.Name()] = struct{}{}

	var users []*ast.Ident
	if id.IsTestFunc() {
		users = append(users, id.ASTIdentity())
	} else {
		users, err = p.findUsers(id)
		if err != nil {
			return influence{}, err
		}
	}

	testFunctions := make(map[string]struct{})
	testSuites := make(map[string]*ast.Ident)
	for _, u := range users {
		if r, f := p.findTestFunction(u); r == nil && f != "" {
			testFunctions[f] = struct{}{}
		} else if r != nil && f != "" {
			testSuites[r.Name] = r
		}
	}

	for _, s := range testSuites {
		if r := p.findTestSuiteRunner(s); r != "" {
			testFunctions[r] = struct{}{}
		}
	}

	return influence{from: id, to: testFunctions}, nil
}

// findEnclosingIdentity finds the top level declaration to which the node at the specified `offset` belongs.
// For example, if the `offset` specifies the position in the function body, it returns the identity of that function.
func (p parsedPackage) findEnclosingIdentity(filename string, offset int64) (identity, error) {
	if !path.IsAbs(filename) {
		filename = filepath.Join(p.pkgDir, filename)
	}
	var pos token.Pos
	p.fset.Iterate(func(f *token.File) bool {
		if f.Name() == filename {
			if int(offset) <= f.Size() {
				pos = f.Pos(int(offset))
			}
			return false
		}
		return true
	})
	if pos == token.NoPos {
		return nil, fmt.Errorf("invalid filename or offset: %s:#%d (build tags are not specified?)", filename, offset)
	}

	f := p.pkg.Files[filename]
	nodes, _ := astutil.PathEnclosingInterval(f, pos, pos)
	for _, n := range nodes {
		if decl, ok := n.(*ast.FuncDecl); ok {
			if decl.Recv == nil {
				// sometimes the package name for test is used
				return functionIdentity{strings.TrimSuffix(p.pkg.Name, "_test"), filename, decl.Name}, nil
			}

			receiverType := decl.Recv.List[0].Type
			return methodIdentity{
				filename:             filename,
				funcIdentity:         decl.Name,
				receiverTypeIdentity: p.findIdentityFromType(receiverType),
			}, nil
		}

		if decl, ok := n.(*ast.GenDecl); ok {
			switch decl.Tok {
			case token.VAR, token.CONST:
				// TODO: support multiple vars case. Pick the nearest var using offset or return multiple identities?
				s := decl.Specs[0].(*ast.ValueSpec)     // assumes there is at least one var.
				return defaultIdentity{s.Names[0]}, nil // When `var a, b int`, always pick the `a` for now.
			case token.TYPE:
				return defaultIdentity{decl.Specs[0].(*ast.TypeSpec).Name}, nil
			}
		}
	}
	return nil, nil
}

// Ignores the star part which is not important for this package.
// For example, it returns `T` when the type is `*T`.
func (p parsedPackage) findIdentityFromType(e ast.Expr) *ast.Ident {
	switch v := e.(type) {
	case *ast.Ident:
		return v
	case *ast.StarExpr:
		return v.X.(*ast.Ident)
	default:
		log.Debugf("unexpected type: %#v", e)
		return nil
	}
}

func (p parsedPackage) findUsers(id identity) ([]*ast.Ident, error) {
	var users []*ast.Ident
	ast.Inspect(p.pkg, func(n ast.Node) bool {
		if other, ok := id.Match(n); ok {
			users = append(users, other)
			return false
		}
		return true
	})

	return users, nil
}

// findTestFunction returns the test function name which uses the specified identity.
func (p parsedPackage) findTestFunction(id *ast.Ident) (receiver *ast.Ident, funcName string) {
	position := p.fset.Position(id.Pos())
	if !strings.HasSuffix(position.Filename, "_test.go") {
		return nil, ""
	}

	nodes, _ := astutil.PathEnclosingInterval(p.pkg.Files[position.Filename], id.Pos(), id.Pos())
	for _, n := range nodes {
		if decl, ok := n.(*ast.FuncDecl); ok {
			if decl.Recv == nil {
				if strings.HasPrefix(decl.Name.Name, "Test") {
					return nil, decl.Name.Name
				}
				continue
			}

			if !isTestSuiteFunction(decl.Name.Name) {
				continue
			}

			receiverType := decl.Recv.List[0].Type
			receiverIdentity := p.findIdentityFromType(receiverType)
			if receiverIdentity == nil {
				continue
			}
			return receiverIdentity, decl.Name.Name
		}
	}
	return nil, ""
}

func (p parsedPackage) findTestSuiteRunner(id *ast.Ident) string {
	users, err := p.findUsers(defaultIdentity{id})
	if err != nil {
		return ""
	}

	for _, u := range users {
		if r, f := p.findTestFunction(u); r == nil && f != "" {
			return f
		}
	}
	return ""
}

type identity interface {
	Match(ast.Node) (*ast.Ident, bool)
	Name() string
	IsTestFunc() bool
	ASTIdentity() *ast.Ident
}

type defaultIdentity struct {
	*ast.Ident
}

func (id defaultIdentity) Match(n ast.Node) (*ast.Ident, bool) {
	if other, ok := n.(*ast.Ident); ok {
		if other.Name == id.Ident.Name && other.Pos() != id.Pos() {
			return other, true
		}
	}
	return nil, false
}

func (id defaultIdentity) Name() string {
	return id.Ident.Name
}

func (id defaultIdentity) IsTestFunc() bool {
	return false
}

func (id defaultIdentity) ASTIdentity() *ast.Ident {
	return id.Ident
}

type functionIdentity struct {
	pkgname, filename string
	*ast.Ident
}

func (id functionIdentity) Match(n ast.Node) (*ast.Ident, bool) {
	// case 1: call from same pkg
	if call, ok := n.(*ast.CallExpr); ok {
		if nameIdentity, ok := call.Fun.(*ast.Ident); ok && nameIdentity.Name == id.Ident.Name {
			return nameIdentity, true
		}
	}

	// case 2: call from test pkg
	if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == id.Ident.Name {
		if exp, ok := sel.X.(*ast.Ident); ok && exp.Name == id.pkgname {
			return sel.Sel, true
		}
	}

	return nil, false
}

func (id functionIdentity) Name() string {
	return id.Ident.Name
}

func (id functionIdentity) IsTestFunc() bool {
	return strings.HasSuffix(id.filename, "_test.go") && strings.HasPrefix(id.Ident.Name, "Test")
}

func (id functionIdentity) ASTIdentity() *ast.Ident {
	return id.Ident
}

type methodIdentity struct {
	filename             string
	funcIdentity         *ast.Ident
	receiverTypeIdentity *ast.Ident
}

func (id methodIdentity) Match(n ast.Node) (*ast.Ident, bool) {
	if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == id.funcIdentity.Name {
		// do not check the type of the receiver to support the method call via interface at a cost of false positive.
		return sel.Sel, true
	}
	return nil, false
}

func (id methodIdentity) Name() string {
	return fmt.Sprintf("%s.%s", id.receiverTypeIdentity.Name, id.funcIdentity.Name)
}

func (id methodIdentity) ASTIdentity() *ast.Ident {
	return id.funcIdentity
}

func (id methodIdentity) IsTestFunc() bool {
	return strings.HasSuffix(id.filename, "_test.go") && isTestSuiteFunction(id.funcIdentity.Name)
}

// `method` must be the method name. Do not specify the function name.
func isTestSuiteFunction(method string) bool {
	if strings.HasPrefix(method, "Test") {
		return true
	}

	// a part of test suite?
	return method == "AfterTest" || method == "BeforeTest" || method == "SetupSuite" ||
		method == "SetupTest" || method == "TearDownSuite" || method == "TearDownTest"
}
