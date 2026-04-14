package mutant

import (
	"fmt"
	"go/ast"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

// ShouldToFilter renames BoolQuery.Should to Filter, testing that tests
// distinguish between scoring (Should) and non-scoring (Filter) clauses.
// Automatically skipped when the same BoolQuery already contains a Filter field
// (renaming would create a duplicate struct key, which is a compile error in Go).
type ShouldToFilter struct{}

func (s *ShouldToFilter) Name() string { return "ShouldToFilter" }

func (s *ShouldToFilter) Apply(site *analyzer.CallSite) ([]*Mutant, error) {
	if site.NodeType != "BoolQuery" || site.Field != "Should" {
		return nil, nil
	}

	desc := fmt.Sprintf("BoolQuery.%s → Filter", site.Field)

	if hasSiblingField(site, "Filter") {
		return []*Mutant{{
			Site:        site,
			Operator:    s.Name(),
			Description: desc,
			SkipReason:  "BoolQuery already has a Filter field; renaming Should would create a duplicate struct key",
		}}, nil
	}

	src, err := applyRewrite(site, func(kv *ast.KeyValueExpr) {
		kv.Key.(*ast.Ident).Name = "Filter"
	})
	if err != nil {
		return nil, err
	}

	return []*Mutant{{
		Site:        site,
		Operator:    s.Name(),
		Description: desc,
		ModifiedSrc: src,
	}}, nil
}
