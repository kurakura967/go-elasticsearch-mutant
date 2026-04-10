package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/runner"
)

// PrintConsole writes the final mutation report to w.
// When verbose is true, go test output is shown for non-killed mutants.
func PrintConsole(w io.Writer, summary Summary, details []MutantDetail, verbose bool) {
	fmt.Fprintf(w, "\nMutation Score: %d/%d (%.1f%%)\n",
		summary.Killed, summary.Total, summary.Score())
	if summary.Skipped > 0 {
		fmt.Fprintf(w, "Skipped: %d (not counted in score)\n", summary.Skipped)
	}

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
		if verbose && d.Output != "" {
			for _, line := range strings.Split(strings.TrimRight(d.Output, "\n"), "\n") {
				fmt.Fprintf(w, "    %s\n", line)
			}
		}
	}
}
