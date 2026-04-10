package report

import (
	"path/filepath"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/mutant"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/runner"
)

// Summary holds aggregate counts for a mutation run.
// Skipped mutants are not included in Total or Score.
type Summary struct {
	Total    int
	Killed   int
	Survived int
	Timeouts int
	Errors   int
	Skipped  int
}

// Score returns the mutation score as a percentage (0-100).
func (s Summary) Score() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Killed) / float64(s.Total) * 100
}

// MutantDetail correlates a mutant's metadata with its run result.
type MutantDetail struct {
	ID          int
	Status      runner.Status
	Operator    string
	Description string
	File        string // path relative to projectDir
	Line        int
	FuncName    string             // enclosing function name
	Output      string             // go test output (non-empty when verbose)
	TestResults []runner.TestResult // per-test pass/fail outcomes
}

// Build correlates mutants with their results.
// File paths in MutantDetail are made relative to projectDir when possible.
func Build(projectDir string, mutants []*mutant.Mutant, results []runner.Result) (Summary, []MutantDetail) {
	byID := make(map[int]*mutant.Mutant, len(mutants))
	for _, m := range mutants {
		byID[m.ID] = m
	}

	var summary Summary
	details := make([]MutantDetail, 0, len(results))

	for _, r := range results {
		switch r.Status {
		case runner.Killed:
			summary.Total++
			summary.Killed++
		case runner.Survived:
			summary.Total++
			summary.Survived++
		case runner.Timeout:
			summary.Total++
			summary.Timeouts++
		case runner.Error:
			summary.Total++
			summary.Errors++
		case runner.Skipped:
			summary.Skipped++
		}

		d := MutantDetail{ID: r.MutantID, Status: r.Status, Output: r.Output, TestResults: r.TestResults}
		if m := byID[r.MutantID]; m != nil {
			d.Operator = m.Operator
			d.Description = m.Description
			d.Line = m.Site.Line
			d.File = relPath(projectDir, m.Site.File)
			d.FuncName = m.Site.FuncName
		}
		details = append(details, d)
	}

	return summary, details
}

func relPath(base, abs string) string {
	if rel, err := filepath.Rel(base, abs); err == nil {
		return rel
	}
	return abs
}
