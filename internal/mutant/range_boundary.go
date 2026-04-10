package mutant

import (
	"fmt"
	"go/ast"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

var rangeBoundarySwap = map[string]string{
	"Gte": "Gt",
	"Lte": "Lt",
}

var rangeNodeTypes = map[string]bool{
	"NumberRangeQuery":  true,
	"DateRangeQuery":    true,
	"TermRangeQuery":    true,
	"UntypedRangeQuery": true,
}

// RangeBoundary changes Gte→Gt and Lte→Lt in range queries (exclusive boundary mutation).
type RangeBoundary struct{}

func (r *RangeBoundary) Name() string { return "RangeBoundary" }

func (r *RangeBoundary) Apply(site *analyzer.CallSite) ([]*Mutant, error) {
	if !rangeNodeTypes[site.NodeType] {
		return nil, nil
	}
	newField, ok := rangeBoundarySwap[site.Field]
	if !ok {
		return nil, nil
	}

	src, err := applyRewrite(site, func(kv *ast.KeyValueExpr) {
		kv.Key.(*ast.Ident).Name = newField
	})
	if err != nil {
		return nil, err
	}

	return []*Mutant{{
		Site:        site,
		Operator:    r.Name(),
		Description: fmt.Sprintf("%s.%s → %s", site.NodeType, site.Field, newField),
		ModifiedSrc: src,
	}}, nil
}
