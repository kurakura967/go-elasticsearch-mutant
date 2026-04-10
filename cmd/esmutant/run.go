package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/mutant"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/report"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/runner"
)

type runOptions struct {
	dir         string
	testPattern string
	workers     int
	timeoutSec  int
	threshold   float64
	output      string
	verbose     bool
}

func newRunCmd() *cobra.Command {
	var opts runOptions

	cmd := &cobra.Command{
		Use:   "run [pattern]",
		Short: "Run mutation testing against a Go package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMutation(cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().StringVarP(&opts.dir, "dir", "d", ".", "project root directory (where go.mod lives)")
	cmd.Flags().StringVar(&opts.testPattern, "test", "", "package pattern for running tests (defaults to [pattern] when empty)")
	cmd.Flags().IntVarP(&opts.workers, "workers", "w", 4, "number of parallel workers")
	cmd.Flags().IntVarP(&opts.timeoutSec, "timeout", "t", 30, "per-test timeout in seconds")
	cmd.Flags().Float64Var(&opts.threshold, "threshold", 80.0, "minimum mutation score (0-100); exit 1 if below")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "console", "output format: console or json")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "show test output for non-killed mutants")

	return cmd
}

func runMutation(ctx context.Context, pattern string, opts runOptions) error {
	dir, err := filepath.Abs(opts.dir)
	if err != nil {
		return fmt.Errorf("resolve dir: %w", err)
	}
	timeout := time.Duration(opts.timeoutSec) * time.Second

	// Resolve test pattern: defaults to source pattern when not specified.
	testPattern := opts.testPattern
	if testPattern == "" {
		testPattern = pattern
	}

	// 1. Analyze
	fmt.Fprintf(os.Stdout, "Analyzing %s\n", pattern)
	if testPattern != pattern {
		fmt.Fprintf(os.Stdout, "Running tests in %s\n", testPattern)
	}
	a := &analyzer.Analyzer{Dir: dir}
	sites, err := a.Analyze(pattern)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	fmt.Fprintf(os.Stdout, "  Found %d mutation target(s)\n\n", len(sites))
	if len(sites) == 0 {
		return nil
	}

	// 2. Generate
	fmt.Fprintln(os.Stdout, "Generating mutants...")
	mutants, err := mutant.Generate(sites, mutant.DefaultOperators)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	for _, m := range mutants {
		rel := relFromDir(dir, m.Site.File)
		fmt.Fprintf(os.Stdout, "  %s:%d %s\t%s\t[%s]\n", rel, m.Site.Line, m.Site.FuncName, m.Description, m.Operator)
	}
	fmt.Fprintf(os.Stdout, "  %d mutant(s) total\n\n", len(mutants))

	// 3. Run
	mgr, err := runner.NewOverlayManager()
	if err != nil {
		return fmt.Errorf("overlay manager: %w", err)
	}
	defer mgr.Cleanup()

	exec := &runner.Executor{ProjectDir: dir, Timeout: timeout}

	fmt.Fprintf(os.Stdout, "Running %d mutant(s) (%d worker(s)) ...\n", len(mutants), opts.workers)
	results, err := runner.RunAll(ctx, mutants, testPattern, exec, mgr, opts.workers, nil)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	// 4. Report
	summary, details := report.Build(dir, mutants, results)

	switch opts.output {
	case "json":
		return report.PrintJSON(os.Stdout, summary, details)
	default:
		report.PrintConsole(os.Stdout, summary, details, opts.verbose)
		if summary.Score() < opts.threshold {
			return fmt.Errorf("mutation score %.1f%% is below threshold %.1f%%",
				summary.Score(), opts.threshold)
		}
	}
	return nil
}

func relFromDir(dir, absPath string) string {
	if rel, err := filepath.Rel(dir, absPath); err == nil {
		return rel
	}
	return absPath
}
