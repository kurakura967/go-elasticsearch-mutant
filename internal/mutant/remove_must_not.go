package mutant

import (
	"go/ast"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

// RemoveMustNot sets BoolQuery.MustNot to nil, testing that exclusion filters are enforced.
type RemoveMustNot struct{}

func (r *RemoveMustNot) Name() string { return "RemoveMustNot" }

func (r *RemoveMustNot) Apply(site *analyzer.CallSite) ([]*Mutant, error) {
	if site.NodeType != "BoolQuery" || site.Field != "MustNot" {
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
		Description: "BoolQuery.MustNot → nil",
		ModifiedSrc: src,
	}}, nil
}
