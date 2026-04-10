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

	// funcStack tracks the name of enclosing FuncDecl/FuncLit as we descend.
	// ast.Inspect has no "on-exit" hook, so we use a stack updated on entry
	// and detect exits by checking whether we returned to a shallower depth.
	var funcStack []string

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			// ast.Inspect calls f(nil) on exit — pop the stack if we pushed.
			// We can't distinguish which node is exiting, so we manage depth
			// via the FuncDecl/FuncLit entry points below.
			return false
		}

		switch v := n.(type) {
		case *ast.FuncDecl:
			funcStack = append(funcStack, v.Name.Name)
			ast.Inspect(v.Body, func(inner ast.Node) bool {
				return inspectNode(inner, fset, info, funcStack, &callSites)
			})
			funcStack = funcStack[:len(funcStack)-1]
			return false // already handled body above
		case *ast.GenDecl:
			// Package-level var/const: inspect with no enclosing function name.
			for _, spec := range v.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, val := range vs.Values {
					ast.Inspect(val, func(inner ast.Node) bool {
						return inspectNode(inner, fset, info, funcStack, &callSites)
					})
				}
			}
			return false
		}
		return true
	})

	return callSites
}

func inspectNode(n ast.Node, fset *token.FileSet, info *gotypes.Info, funcStack []string, callSites *[]*CallSite) bool {
	if n == nil {
		return false
	}
	cl, ok := n.(*ast.CompositeLit)
	if !ok {
		return true
	}

	structName, ok := esStructName(info, cl)
	if !ok {
		return true
	}

	funcName := ""
	if len(funcStack) > 0 {
		funcName = funcStack[len(funcStack)-1]
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
		*callSites = append(*callSites, &CallSite{
			File:     pos.Filename,
			Line:     pos.Line,
			NodeType: structName,
			Field:    ident.Name,
			FuncName: funcName,
			Node:     kv,
		})
	}

	return true
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
