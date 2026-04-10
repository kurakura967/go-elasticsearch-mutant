package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/runner"
)

// printTestResults prints the test function names that ran for a mutant.
// For KILLED mutants it shows only the leaf-level failing tests.
// For SURVIVED mutants it shows only top-level passing tests.
func printTestResults(w io.Writer, d MutantDetail) {
	switch d.Status {
	case runner.Killed:
		leaves := runner.LeafFailures(d.TestResults)
		if len(leaves) == 0 {
			return
		}
		fmt.Fprintf(w, "    Detected by:\n")
		for _, tr := range leaves {
			fmt.Fprintf(w, "      %s  ✗ failed\n", tr.Name)
		}
	case runner.Survived:
		top := runner.TopLevelPassing(d.TestResults)
		if len(top) == 0 {
			return
		}
		fmt.Fprintf(w, "    Tested by (all passed):\n")
		for _, tr := range top {
			fmt.Fprintf(w, "      %s  ✓ passed\n", tr.Name)
		}
	}
}

// PrintConsole writes the final mutation report to w.
// When verbose is true, go test output is shown for survived/errored mutants.
func PrintConsole(w io.Writer, summary Summary, details []MutantDetail, verbose bool) {
	// Score line
	fmt.Fprintf(w, "\nMutation Score: %d/%d (%.1f%%)\n",
		summary.Killed, summary.Total, summary.Score())

	// Breakdown so the denominator is self-explanatory
	fmt.Fprintf(w, "  Killed: %d  Survived: %d  Timeouts: %d  Errors: %d",
		summary.Killed, summary.Survived, summary.Timeouts, summary.Errors)
	if summary.Skipped > 0 {
		fmt.Fprintf(w, "  |  Skipped: %d (not counted in score)", summary.Skipped)
	}
	fmt.Fprintln(w)

	printSection(w, "KILLED", runner.Killed, details, false)
	printSection(w, "SURVIVED", runner.Survived, details, verbose)
	if summary.Timeouts > 0 {
		printSection(w, "TIMEOUT", runner.Timeout, details, verbose)
	}
	if summary.Errors > 0 {
		printSection(w, "ERROR", runner.Error, details, verbose)
	}
	if summary.Skipped > 0 {
		printSkippedSection(w, details)
	}
}

func printSkippedSection(w io.Writer, details []MutantDetail) {
	var items []MutantDetail
	for _, d := range details {
		if d.Status == runner.Skipped {
			items = append(items, d)
		}
	}
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(w, "\nSKIPPED (%d):\n", len(items))
	for _, d := range items {
		fmt.Fprintf(w, "  %s:%d %s\t%-40s [%s]\n", d.File, d.Line, d.FuncName, d.Description, d.Operator)
		fmt.Fprintf(w, "    reason: %s\n", d.Output)
	}
}

func printSection(w io.Writer, title string, status runner.Status, details []MutantDetail, verbose bool) {
	var items []MutantDetail
	for _, d := range details {
		if d.Status == status {
			items = append(items, d)
		}
	}
	if len(items) == 0 {
		return
	}

	fmt.Fprintf(w, "\n%s (%d):\n", title, len(items))
	for _, d := range items {
		fmt.Fprintf(w, "  %s:%d %s\t%-40s [%s]\n", d.File, d.Line, d.FuncName, d.Description, d.Operator)
		printTestResults(w, d)
		if verbose && d.Output != "" {
			for _, line := range strings.Split(strings.TrimRight(d.Output, "\n"), "\n") {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}
	}
}
