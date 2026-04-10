package mutant

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

// hasSiblingField reports whether the composite literal containing the
// KeyValueExpr at site.Line/site.Field also has a field named siblingField.
func hasSiblingField(site *analyzer.CallSite, siblingField string) bool {
	src, err := os.ReadFile(site.File)
	if err != nil {
		return false
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, site.File, src, 0)
	if err != nil {
		return false
	}

	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		containsTarget, hasSibling := false, false
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			id, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			if fset.Position(kv.Pos()).Line == site.Line && id.Name == site.Field {
				containsTarget = true
			}
			if id.Name == siblingField {
				hasSibling = true
			}
		}
		if containsTarget && hasSibling {
			found = true
		}
		return true
	})
	return found
}

// wouldCauseUnusedVar reports whether replacing the value of the field at site
// with nil would leave any locally-declared variable without any remaining reads.
// This prevents mutations that produce "declared and not used" compile errors.
func wouldCauseUnusedVar(site *analyzer.CallSite) bool {
	src, err := os.ReadFile(site.File)
	if err != nil {
		return false
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, site.File, src, 0)
	if err != nil {
		return false
	}

	// Locate the target KV and its enclosing function.
	var targetKV *ast.KeyValueExpr
	var enclosingFunc *ast.FuncDecl
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			id, ok := kv.Key.(*ast.Ident)
			if !ok {
				return true
			}
			if fset.Position(kv.Pos()).Line == site.Line && id.Name == site.Field {
				targetKV = kv
				enclosingFunc = fn
				return false
			}
			return true
		})
		if targetKV != nil {
			break
		}
	}
	if targetKV == nil || enclosingFunc == nil {
		return false
	}

	// Collect identifier names referenced inside the value being replaced.
	valueIdents := map[string]bool{}
	ast.Inspect(targetKV.Value, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			valueIdents[id.Name] = true
		}
		return true
	})
	if len(valueIdents) == 0 {
		return false
	}

	// Find variables declared locally in the function (via := or var).
	localVars := map[string]bool{}
	ast.Inspect(enclosingFunc.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			if v.Tok == token.DEFINE {
				for _, lhs := range v.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name != "_" {
						localVars[id.Name] = true
					}
				}
			}
		case *ast.GenDecl:
			for _, spec := range v.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range vs.Names {
						if name.Name != "_" {
							localVars[name.Name] = true
						}
					}
				}
			}
		}
		return true
	})

	// For each local var referenced in the value being removed, count its
	// appearances in the function body outside the value range.
	// A count of ≤1 means only the declaration site remains → unused after removal.
	valueStart := targetKV.Value.Pos()
	valueEnd := targetKV.Value.End()

	for name := range valueIdents {
		if !localVars[name] {
			continue
		}
		outside := 0
		ast.Inspect(enclosingFunc.Body, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			// Skip the value subtree being replaced.
			if n.Pos() >= valueStart && n.End() <= valueEnd {
				return false
			}
			if id, ok := n.(*ast.Ident); ok && id.Name == name {
				outside++
			}
			return true
		})
		// outside == 1 means only the declaration remains → variable unused after removal.
		if outside <= 1 {
			return true
		}
	}
	return false
}

// applyRewrite re-parses site.File, locates the KeyValueExpr at site.Line whose
// key is site.Field, calls rewrite to mutate the node in-place, then returns the
// go/format-formatted result.
func applyRewrite(site *analyzer.CallSite, rewrite func(*ast.KeyValueExpr)) ([]byte, error) {
	src, err := os.ReadFile(site.File)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", site.File, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, site.File, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", site.File, err)
	}

	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok {
			return true
		}
		if fset.Position(kv.Pos()).Line == site.Line && ident.Name == site.Field {
			rewrite(kv)
			found = true
			return false
		}
		return true
	})

	if !found {
		return nil, fmt.Errorf("node not found at %s:%d (field %q)", site.File, site.Line, site.Field)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, fmt.Errorf("format %s: %w", site.File, err)
	}
	return buf.Bytes(), nil
}
