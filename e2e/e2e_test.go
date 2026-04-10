//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/mutant"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/report"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/runner"
)

const (
	e2eThreshold = 80.0
	e2eTimeout   = 10 * time.Minute
	perTestLimit = 60 * time.Second
	// workers=1 to avoid concurrent go test processes competing over the same
	// Elasticsearch indices (each test invocation runs TestMain which creates
	// and later deletes indices). Parallel execution causes race conditions
	// that produce spurious ERROR results.
	e2eWorkers = 1
)

// moduleRoot returns the absolute path to the repository root.
func moduleRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..")
}

// esURL returns the Elasticsearch URL from the environment, defaulting to localhost.
func esURL() string {
	if u := os.Getenv("ELASTICSEARCH_URL"); u != "" {
		return u
	}
	return "http://localhost:9200"
}

// requireES skips the test if Elasticsearch is not reachable.
func requireES(t *testing.T) {
	t.Helper()
	url := esURL()
	resp, err := http.Get(url) //nolint:noctx
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skipf("Elasticsearch not available at %s — skipping E2E test", url)
	}
	resp.Body.Close()
}

// TestE2E_MutationPipeline runs the full mutation pipeline against example/
// and verifies the mutation score meets the configured threshold.
func TestE2E_MutationPipeline(t *testing.T) {
	requireES(t)

	root := moduleRoot()
	exampleDir := filepath.Join(root, "example")

	// 1. Analyze
	t.Log("--- Analyzing example/ ---")
	a := &analyzer.Analyzer{Dir: exampleDir}
	sites, err := a.Analyze(".")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	t.Logf("Found %d mutation target(s)", len(sites))
	if len(sites) == 0 {
		t.Fatal("no mutation targets found in example/")
	}

	// 2. Generate
	t.Log("--- Generating mutants ---")
	mutants, err := mutant.Generate(sites, mutant.DefaultOperators)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	t.Logf("Generated %d mutant(s)", len(mutants))
	for _, m := range mutants {
		rel, _ := filepath.Rel(root, m.Site.File)
		t.Logf("  [%d] %s:%d  %s  [%s]", m.ID, rel, m.Site.Line, m.Description, m.Operator)
	}

	// 3. Run
	t.Log("--- Running mutants ---")
	mgr, err := runner.NewOverlayManager()
	if err != nil {
		t.Fatalf("overlay manager: %v", err)
	}
	t.Cleanup(func() { mgr.Cleanup() })

	ctx, cancel := context.WithTimeout(context.Background(), e2eTimeout)
	defer cancel()

	exec := &runner.Executor{ProjectDir: root, Timeout: perTestLimit}
	results, err := runner.RunAll(ctx, mutants, "./example/...", exec, mgr, e2eWorkers, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// 4. Report
	t.Log("--- Results ---")
	summary, details := report.Build(root, mutants, results)

	for _, d := range details {
		t.Logf("  [%s] %s:%d  %s  [%s]", d.Status, d.File, d.Line, d.Description, d.Operator)
	}
	t.Logf("Mutation Score: %d/%d (%.1f%%)", summary.Killed, summary.Total, summary.Score())

	// 5. Assertions
	if summary.Total == 0 {
		t.Fatal("no mutants were executed")
	}
	if summary.Errors > 0 {
		t.Logf("WARNING: %d mutant(s) had build/execution errors", summary.Errors)
	}
	if summary.Score() < e2eThreshold {
		t.Errorf("mutation score %.1f%% is below threshold %.1f%%", summary.Score(), e2eThreshold)
	}
}

// TestE2E_KilledByOperator verifies that each operator kills at least one mutant,
// ensuring every test in example/ exercises a different aspect of the query.
func TestE2E_KilledByOperator(t *testing.T) {
	requireES(t)

	root := moduleRoot()
	exampleDir := filepath.Join(root, "example")

	a := &analyzer.Analyzer{Dir: exampleDir}
	sites, err := a.Analyze(".")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	mutants, err := mutant.Generate(sites, mutant.DefaultOperators)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	mgr, err := runner.NewOverlayManager()
	if err != nil {
		t.Fatalf("overlay manager: %v", err)
	}
	t.Cleanup(func() { mgr.Cleanup() })

	ctx, cancel := context.WithTimeout(context.Background(), e2eTimeout)
	defer cancel()

	exec := &runner.Executor{ProjectDir: root, Timeout: perTestLimit}
	results, err := runner.RunAll(ctx, mutants, "./example/...", exec, mgr, e2eWorkers, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	_, details := report.Build(root, mutants, results)

	// Build sets of operators by outcome.
	appliedBy := map[string]bool{} // generated >= 1 non-skipped mutant
	killedBy := map[string]bool{}
	for _, d := range details {
		if d.Status != runner.Skipped {
			appliedBy[d.Operator] = true
		}
		if d.Status == runner.Killed {
			killedBy[d.Operator] = true
		}
	}

	for _, op := range mutant.DefaultOperators {
		if !appliedBy[op.Name()] {
			t.Logf("operator %q: all mutations skipped — no applicable sites in example/", op.Name())
			continue
		}
		if !killedBy[op.Name()] {
			t.Errorf("operator %q did not kill any mutant — check example/ test coverage", op.Name())
		} else {
			t.Logf("operator %q: ✓ kills at least one mutant", op.Name())
		}
	}
}

// TestE2E_SurvivedList logs surviving mutants as actionable hints for test improvement.
// This test never fails; it only surfaces coverage gaps.
func TestE2E_SurvivedList(t *testing.T) {
	requireES(t)

	root := moduleRoot()
	exampleDir := filepath.Join(root, "example")

	a := &analyzer.Analyzer{Dir: exampleDir}
	sites, err := a.Analyze(".")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	mutants, err := mutant.Generate(sites, mutant.DefaultOperators)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	mgr, err := runner.NewOverlayManager()
	if err != nil {
		t.Fatalf("overlay manager: %v", err)
	}
	t.Cleanup(func() { mgr.Cleanup() })

	ctx, cancel := context.WithTimeout(context.Background(), e2eTimeout)
	defer cancel()

	exec := &runner.Executor{ProjectDir: root, Timeout: perTestLimit}
	results, err := runner.RunAll(ctx, mutants, "./example/...", exec, mgr, e2eWorkers, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	_, details := report.Build(root, mutants, results)

	survived := 0
	for _, d := range details {
		if d.Status == runner.Survived {
			survived++
			t.Logf("SURVIVED: %s:%d  %s  [%s]", d.File, d.Line, d.Description, d.Operator)
			t.Logf("  → consider adding a test that catches this mutation")
		}
	}
	if survived == 0 {
		t.Log("All mutants were killed — excellent test coverage!")
	} else {
		t.Logf("%d mutant(s) survived — see above for improvement hints", survived)
	}
	fmt.Println() // blank line for readability
}
