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

	"github.com/ks888/hornet/common/log"
	"golang.org/x/tools/go/ast/astutil"
)

// FindTestFunctions finds the test functions which related to the specified `filename` and offset`. `filename` must be abs.
// summary:
// 1. Finds the top-level declaration which encloses the specified offset.
// 2-1. If the decl is the test function itself, just returns that test function.
// 2-2. Otherwise, lists the test functions which uses the identity step 1 finds. It only searches for the files in the same package.
//   For example, if the offset specifies the last line of some non-test function, it finds the name of the function (e.g. `Sum`) first,
//   and then finds the test functions which directly call the function (e.g. `TestSum1` and `TestSum2`).
func FindTestFunctions(ctxt *build.Context, filename string, offset int) (map[string]struct{}, error) {
	pkg, err := newParsedPackage(ctxt, filepath.Dir(filename))
	if err != nil {
		if _, ok := err.(*build.NoGoError); ok {
			return nil, nil
		}
		return nil, err
	}

	id, err := pkg.findEnclosingIdentity(filename, offset)
	if err != nil {
		return nil, err
	}

	if id == nil {
		return nil, nil
	} else if id.IsTestFunc() {
		return map[string]struct{}{id.Name(): struct{}{}}, nil
	}

	users, err := pkg.findUsers(id)
	if err != nil {
		return nil, err
	}

	set := make(map[string]struct{})
	for _, u := range users {
		if f := pkg.findTestFunction(u); f != "" {
			set[f] = struct{}{}
		}
	}

	if log.DebugLogEnabled() {
		var fs []string
		for f := range set {
			fs = append(fs, f)
		}
		log.Debugf("%s affects [%v]\n", id.Name(), strings.Join(fs, ", "))
	}

	return set, nil
}

type parsedPackage struct {
	pkgDir string
	pkg    *ast.Package
	fset   *token.FileSet
	info   *types.Info
}

// `packageDir` must be abs.
func newParsedPackage(ctxt *build.Context, packageDir string) (parsedPackage, error) {
	pkg, err := ctxt.ImportDir(packageDir, build.IgnoreVendor)
	if err != nil {
		return parsedPackage{}, err
	}

	filenames := pkg.GoFiles
	filenames = append(filenames, pkg.TestGoFiles...)
	filenames = append(filenames, pkg.CgoFiles...)

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
		log.Debugf("type check error: %v", err)
	}
	_, _ = conf.Check(astPkg.Name, fset, files, &info)
	return parsedPackage{pkgDir: packageDir, pkg: astPkg, fset: fset, info: &info}, nil
}

// findEnclosingIdentity finds the top level declaration to which the node at the specified `offset` belongs.
// For example, if the `offset` specifies the position in the function body, it returns the identity of that function.
func (p parsedPackage) findEnclosingIdentity(filename string, offset int) (identity, error) {
	if !path.IsAbs(filename) {
		filename = filepath.Join(p.pkgDir, filename)
	}
	var pos token.Pos
	p.fset.Iterate(func(f *token.File) bool {
		if f.Name() == filename {
			if offset <= f.Size() {
				pos = f.Pos(offset)
			}
			return false
		}
		return true
	})
	if pos == token.NoPos {
		return nil, fmt.Errorf("invalid filename or offset: %s %d", filename, offset)
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
			if decl.Recv == nil && strings.HasPrefix(decl.Name.Name, "Test") {
				return decl.Name.Name
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
	return false
}
