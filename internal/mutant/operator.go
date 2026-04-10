package mutant

import "github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"

// Operator is a mutation operator that generates Mutants from a CallSite.
type Operator interface {
	Name() string
	Apply(site *analyzer.CallSite) ([]*Mutant, error)
}
