package mutant

import (
	"fmt"
	"go/ast"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

var rangeDirectionSwap = map[string]string{
	"Gte": "Lte",
	"Lte": "Gte",
	"Gt":  "Lt",
	"Lt":  "Gt",
}

// RangeDirection swaps the direction of a range boundary (Gte↔Lte, Gt↔Lt),
// testing that tests distinguish between lower-bound and upper-bound conditions.
// Automatically skipped when the target field already exists as a sibling
// (renaming would create a duplicate struct key, which is a compile error).
type RangeDirection struct{}

func (r *RangeDirection) Name() string { return "RangeDirection" }

func (r *RangeDirection) Apply(site *analyzer.CallSite) ([]*Mutant, error) {
	if !rangeNodeTypes[site.NodeType] {
		return nil, nil
	}
	newField, ok := rangeDirectionSwap[site.Field]
	if !ok {
		return nil, nil
	}

	desc := fmt.Sprintf("%s.%s → %s", site.NodeType, site.Field, newField)

	if hasSiblingField(site, newField) {
		return []*Mutant{{
			Site:        site,
			Operator:    r.Name(),
			Description: desc,
			SkipReason:  fmt.Sprintf("%s already has a %s field; renaming %s would create a duplicate struct key", site.NodeType, newField, site.Field),
		}}, nil
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
		Description: desc,
		ModifiedSrc: src,
	}}, nil
}
