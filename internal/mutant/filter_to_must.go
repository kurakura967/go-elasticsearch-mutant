package mutant

import (
	"go/ast"

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
