package mutant

import (
	"fmt"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

// DefaultOperators is the standard set of mutation operators.
var DefaultOperators = []Operator{
	&RemoveClause{},
	&MustToShould{},
	&ShouldToFilter{},
	&FilterToMust{},
	&RangeBoundary{},
	&RangeDirection{},
	&RemoveMustNot{},
	&RemoveFunctionScoreFilter{},
	&MultiMatchType{},
}

// Generate applies all operators to all sites and returns mutants with sequential IDs.
func Generate(sites []*analyzer.CallSite, operators []Operator) ([]*Mutant, error) {
	var mutants []*Mutant
	id := 1
	for _, site := range sites {
		for _, op := range operators {
			ms, err := op.Apply(site)
			if err != nil {
				return nil, fmt.Errorf("operator %s on %s:%d: %w", op.Name(), site.File, site.Line, err)
			}
			for _, m := range ms {
				m.ID = id
				id++
			}
			mutants = append(mutants, ms...)
		}
	}
	return mutants, nil
}
