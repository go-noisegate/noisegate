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

	if id.IsTestFunc() {
		to := make(map[string]struct{})
		name := id.Name()
		if index := strings.Index(name, "."); index != -1 {
			// workaround to support test suite. TODO: find more precise approach.
			for _, t := range []string{"Test_", "Test"} {
				if p.pkg.Scope.Lookup(t+name[:index]) != nil {
					to[t+name[:index]] = struct{}{}
				}
			}
		} else {
			to[name] = struct{}{}
		}
		return influence{from: id, to: to}, nil
	}

	users, err := p.findUsers(id)
	if err != nil {
		return influence{}, err
	}

	set := make(map[string]struct{})
	for _, u := range users {
		if f := p.findTestFunction(u); f != "" {
			set[f] = struct{}{}
		}
	}
	return influence{from: id, to: set}, nil
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
				return functionIdentity{filename, decl.Name, p.pkg.Scope.Objects[decl.Name.Name]}, nil // decl.Name.Obj is nil
			}

			receiverType := decl.Recv.List[0].Type
			return methodIdentity{
				filename:         filename,
				functionName:     decl.Name.Name,
				receiverTypename: p.findTypenameFromType(receiverType),
				findTypename:     p.findTypenameFromVar,
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
func (p parsedPackage) findTypenameFromType(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		id := v.X.(*ast.Ident)
		return id.Name
	default:
		log.Debugf("unexpected type: %#v", e)
		return ""
	}
}

// Ignores the pointer part which is not important for this package.
// For example, it returns `T` when the variable is `var t *T`.
func (p parsedPackage) findTypenameFromVar(e ast.Expr) string {
	if typ := p.info.TypeOf(e); typ != nil {
		for {
			ptr, ok := typ.(*types.Pointer)
			if !ok {
				break
			}
			typ = ptr.Elem()
		}
		if named, ok := typ.(*types.Named); ok {
			return named.Obj().Name()
		}
	}
	return ""
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

func (p parsedPackage) findTestFunction(id *ast.Ident) string {
	position := p.fset.Position(id.Pos())
	if !strings.HasSuffix(position.Filename, "_test.go") {
		return ""
	}

	nodes, _ := astutil.PathEnclosingInterval(p.pkg.Files[position.Filename], id.Pos(), id.Pos())
	for _, n := range nodes {
		if decl, ok := n.(*ast.FuncDecl); ok {
			if !strings.HasPrefix(decl.Name.Name, "Test") {
				continue
			}
			if decl.Recv == nil {
				return decl.Name.Name
			}

			// workaround to support test suite. TODO: Find more precise approach.
			receiverType := decl.Recv.List[0].Type
			suiteName := p.findTypenameFromType(receiverType)
			for _, t := range []string{"Test_", "Test"} {
				if p.pkg.Scope.Lookup(t+suiteName) != nil {
					return t + suiteName
				}
			}
		}
	}
	return ""
}

type identity interface {
	Match(ast.Node) (*ast.Ident, bool)
	Name() string
	IsTestFunc() bool
}

type defaultIdentity struct {
	*ast.Ident
}

func (id defaultIdentity) Match(n ast.Node) (*ast.Ident, bool) {
	if other, ok := n.(*ast.Ident); ok {
		if other.Obj == id.Obj && other.Pos() != id.Obj.Pos() {
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

type functionIdentity struct {
	filename string
	*ast.Ident
	obj *ast.Object
}

func (id functionIdentity) Match(n ast.Node) (*ast.Ident, bool) {
	if other, ok := n.(*ast.Ident); ok {
		defer func() {
			// sometimes panic. Need more context.
			if r := recover(); r != nil {
				log.Printf("panic: %v\n", r)
				log.Printf("id: %#v, %#v\n", id, id.obj)
				log.Printf("other: %#v, %#v\n", other, other.Obj)
			}
		}()

		if other.Obj == id.obj && other.Pos() != id.obj.Pos() {
			return other, true
		}
	}
	return nil, false
}

func (id functionIdentity) Name() string {
	return id.Ident.Name
}

func (id functionIdentity) IsTestFunc() bool {
	return strings.HasSuffix(id.filename, "_test.go") && strings.HasPrefix(id.Name(), "Test")
}

type methodIdentity struct {
	filename         string
	functionName     string
	receiverTypename string
	findTypename     func(ast.Expr) string
}

func (id methodIdentity) Match(n ast.Node) (*ast.Ident, bool) {
	if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == id.functionName {
		if typename := id.findTypename(sel.X); typename != "" && typename == id.receiverTypename {
			return sel.Sel, true
		}
	}
	return nil, false
}

func (id methodIdentity) Name() string {
	return fmt.Sprintf("%s.%s", id.receiverTypename, id.functionName)
}

func (id methodIdentity) IsTestFunc() bool {
	return strings.HasSuffix(id.filename, "_test.go") && strings.HasPrefix(id.functionName, "Test")
}
