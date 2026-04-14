package mutant

import (
	"fmt"
	"go/ast"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

// RemoveFunctionScoreFilter sets FunctionScore.Filter to nil, testing that the
// weight boost is correctly scoped to matching documents and not applied globally.
type RemoveFunctionScoreFilter struct{}

func (r *RemoveFunctionScoreFilter) Name() string { return "RemoveFunctionScoreFilter" }

func (r *RemoveFunctionScoreFilter) Apply(site *analyzer.CallSite) ([]*Mutant, error) {
	if site.NodeType != "FunctionScore" || site.Field != "Filter" {
		return nil, nil
	}

	if wouldCauseUnusedVar(site) {
		return []*Mutant{{
			Site:        site,
			Operator:    r.Name(),
			Description: "FunctionScore.Filter → nil",
			SkipReason:  "removing this filter would leave a locally-declared variable unused",
		}}, nil
	}

	if wouldCauseUnusedImport(site) {
		return []*Mutant{{
			Site:        site,
			Operator:    r.Name(),
			Description: "FunctionScore.Filter → nil",
			SkipReason:  "removing this filter would leave an imported package unused",
		}}, nil
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
		Description: fmt.Sprintf("%s.%s → nil", site.NodeType, site.Field),
		ModifiedSrc: src,
	}}, nil
}
