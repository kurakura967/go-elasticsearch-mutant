package mutant

import (
	"fmt"
	"go/ast"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

var removeClauseFields = map[string]bool{
	"Must":   true,
	"Should": true,
}

// RemoveClause sets a BoolQuery clause (Must / Should) to nil.
// Filter is intentionally excluded: filter clauses do not affect relevance
// scoring, so removing them tests the same property as removing Must.
// Use FilterToMust to test the scoring-vs-non-scoring distinction instead.
type RemoveClause struct{}

func (r *RemoveClause) Name() string { return "RemoveClause" }

func (r *RemoveClause) Apply(site *analyzer.CallSite) ([]*Mutant, error) {
	if site.NodeType != "BoolQuery" || !removeClauseFields[site.Field] {
		return nil, nil
	}

	src, err := applyRewrite(site, func(kv *ast.KeyValueExpr) {
		kv.Value = ast.NewIdent("nil")
	})
	if err != nil {
		return nil, err
	}

	return []*Mutant{{
		Site:        site,
		Operator:    r.Name(),
		Description: fmt.Sprintf("BoolQuery.%s → nil", site.Field),
		ModifiedSrc: src,
	}}, nil
}
