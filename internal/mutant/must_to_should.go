package mutant

import (
	"go/ast"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

// MustToShould changes BoolQuery.Must to Should.
type MustToShould struct{}

func (m *MustToShould) Name() string { return "MustToShould" }

func (m *MustToShould) Apply(site *analyzer.CallSite) ([]*Mutant, error) {
	if site.NodeType != "BoolQuery" || site.Field != "Must" {
		return nil, nil
	}

	src, err := applyRewrite(site, func(kv *ast.KeyValueExpr) {
		kv.Key.(*ast.Ident).Name = "Should"
	})
	if err != nil {
		return nil, err
	}

	return []*Mutant{{
		Site:        site,
		Operator:    m.Name(),
		Description: "BoolQuery.Must → Should",
		ModifiedSrc: src,
	}}, nil
}
