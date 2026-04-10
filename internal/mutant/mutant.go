package mutant

import "github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"

// Mutant represents a single mutation of a source file.
type Mutant struct {
	ID          int
	Site        *analyzer.CallSite
	Operator    string
	Description string
	ModifiedSrc []byte // full file content after rewrite, formatted by go/format
	SkipReason  string // non-empty: mutation was skipped; ModifiedSrc will be nil
}
