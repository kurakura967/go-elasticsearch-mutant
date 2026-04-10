package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/packages"
)

const esTypesPackagePath = "github.com/elastic/go-elasticsearch/v8/typedapi/types"

// targetFields is the whitelist of ES struct fields to detect as mutation targets.
var targetFields = map[string]bool{
	"Must":    true,
	"Should":  true,
	"Filter":  true,
	"MustNot": true,
	"Gte":     true,
	"Gt":      true,
	"Lte":     true,
	"Lt":      true,
}

// Analyzer analyzes Go packages and extracts ES Typed API call sites.
type Analyzer struct {
	Dir string // working directory for package loading
}

// Analyze loads the package matching pattern and returns all detected CallSites.
func (a *Analyzer) Analyze(pattern string) ([]*CallSite, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps,
		Dir:  a.Dir,
		Fset: fset,
	}

	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}

	var callSites []*CallSite
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			callSites = append(callSites, extractCallSites(fset, pkg.TypesInfo, file)...)
		}
	}

	return callSites, nil
}

func extractCallSites(fset *token.FileSet, info *gotypes.Info, file *ast.File) []*CallSite {
	var callSites []*CallSite

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		funcName := fn.Name.Name

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}

			structName, ok := esStructName(info, cl)
			if !ok {
				return true
			}

			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				ident, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				if !targetFields[ident.Name] {
					continue
				}

				pos := fset.Position(kv.Pos())
				callSites = append(callSites, &CallSite{
					File:     pos.Filename,
					Line:     pos.Line,
					NodeType: structName,
					Field:    ident.Name,
					FuncName: funcName,
					Node:     kv,
				})
			}

			return true
		})
	}

	return callSites
}

// esStructName returns the struct name if cl is a composite literal of an ES types struct.
func esStructName(info *gotypes.Info, cl *ast.CompositeLit) (string, bool) {
	tv, ok := info.Types[cl]
	if !ok {
		return "", false
	}

	typ := tv.Type
	if ptr, ok := typ.(*gotypes.Pointer); ok {
		typ = ptr.Elem()
	}

	named, ok := typ.(*gotypes.Named)
	if !ok {
		return "", false
	}

	pkg := named.Obj().Pkg()
	if pkg == nil || pkg.Path() != esTypesPackagePath {
		return "", false
	}

	return named.Obj().Name(), true
}
