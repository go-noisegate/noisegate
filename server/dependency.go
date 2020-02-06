package server

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"

	"golang.org/x/tools/go/ast/astutil"
)

type parsedPackage struct {
	pkg  *ast.Package
	fset *token.FileSet
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
	for _, file := range filenames {
		f, _ := parser.ParseFile(fset, filepath.Join(packageDir, file), nil, 0)
		if f != nil {
			parsedFiles[file] = f
		}
	}

	// NewPackage returns the error when there are unresolved identities, which is ignorable here.
	astPkg, _ := ast.NewPackage(fset, parsedFiles, nil, nil)
	return parsedPackage{pkg: astPkg, fset: fset}, nil
}

func (p parsedPackage) findEnclosingIdentity(filename string, offset int) (*ast.Ident, error) {
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
	fmt.Printf("nodes: %#v\n", nodes)
	for _, n := range nodes {
		if decl, ok := n.(*ast.FuncDecl); ok {
			return decl.Name, nil
		}
		if decl, ok := n.(*ast.GenDecl); ok {
			switch decl.Tok {
			case token.VAR, token.CONST:
				// TODO: support multiple vars case. Pick the nearest var using offset or return multiple identities?
				s := decl.Specs[0].(*ast.ValueSpec) // assumes there is at least one var.
				return s.Names[0], nil              // When `var a, b int`, always pick the `a` for now.
			case token.TYPE:
				return decl.Specs[0].(*ast.TypeSpec).Name, nil
			}
		}
	}
	return nil, nil
}

// assumes `filename` is abs
func FindTestFunctions(ctxt *build.Context, filename string, offset int) ([]token.Position, error) {
	return nil, nil
}
