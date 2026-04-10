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
