package mutant

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

// FilterToMust changes BoolQuery.Filter to Must, making the clause affect
// relevance scoring. This tests whether the test suite can detect the
// intentional use of a non-scoring filter vs a scoring must clause.
type FilterToMust struct{}

func (f *FilterToMust) Name() string { return "FilterToMust" }

func (f *FilterToMust) Apply(site *analyzer.CallSite) ([]*Mutant, error) {
	if site.NodeType != "BoolQuery" || site.Field != "Filter" {
		return nil, nil
	}

	// If the same BoolQuery already has a Must field, renaming Filter to Must
	// would create a duplicate struct key — a compile error. Skip with a reason.
	if hasSiblingField(site, "Must") {
		return []*Mutant{{
			Site:        site,
			Operator:    f.Name(),
			Description: "BoolQuery.Filter → Must",
			SkipReason:  "BoolQuery already has a Must field; renaming Filter would create a duplicate struct key",
		}}, nil
	}

	src, err := applyRewrite(site, func(kv *ast.KeyValueExpr) {
		kv.Key.(*ast.Ident).Name = "Must"
	})
	if err != nil {
		return nil, err
	}

	return []*Mutant{{
		Site:        site,
		Operator:    f.Name(),
		Description: "BoolQuery.Filter → Must",
		ModifiedSrc: src,
	}}, nil
}

// hasSiblingField reports whether the composite literal that contains the
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
		containsTarget := false
		hasSibling := false
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			ident, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			if fset.Position(kv.Pos()).Line == site.Line && ident.Name == site.Field {
				containsTarget = true
			}
			if ident.Name == siblingField {
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
