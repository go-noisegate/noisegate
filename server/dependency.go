package server

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"

	"github.com/ks888/hornet/common/log"
	"golang.org/x/tools/go/ast/astutil"
)

type parsedPackage struct {
	pkg  *ast.Package
	fset *token.FileSet
	info *types.Info
}

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
		f, err := parser.ParseFile(fset, filepath.Join(packageDir, file), nil, 0)
		if err != nil {
			log.Printf("failed to parse %s: %v\n", file, err)
		}
		if f != nil {
			parsedFiles[file] = f
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
	return parsedPackage{pkg: astPkg, fset: fset, info: &info}, nil
}

// findEnclosingIdentity finds the top level declaration to which the node at the specified `offset` belongs.
// For example, if the `offset` specifies the position in the function body, it returns the identity of that function.
func (p parsedPackage) findEnclosingIdentity(filename string, offset int) (identity, error) {
	var pos token.Pos
	p.fset.Iterate(func(f *token.File) bool {
		if filepath.Base(f.Name()) == filepath.Base(filename) {
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
				return functionIdentity{decl.Name, p.pkg.Scope.Objects[decl.Name.Name]}, nil // decl.Name.Obj is nil
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

// assumes `filename` is abs
func FindTestFunctions(ctxt *build.Context, filename string, offset int) ([]token.Position, error) {
	return nil, nil
}

type identity interface {
	Match(ast.Node) (*ast.Ident, bool)
	Name() string
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

type functionIdentity struct {
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
