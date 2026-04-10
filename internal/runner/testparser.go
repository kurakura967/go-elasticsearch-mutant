package runner

import (
	"bufio"
	"encoding/json"
	"strings"
)

// TestResult holds the outcome of a single test function.
type TestResult struct {
	Name   string
	Passed bool // true = passed, false = failed
}

// testEvent represents one line of `go test -json` output.
type testEvent struct {
	Action string
	Test   string
	Output string
}

// parseTestJSON parses `go test -json` output.
// Returns:
//   - results: per-test pass/fail outcomes (only named tests, not package-level events)
//   - humanOutput: human-readable output reconstructed from the JSON stream
func parseTestJSON(raw string) (results []TestResult, humanOutput string) {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		var ev testEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// Non-JSON line (e.g. compiler errors): pass through as-is.
			sb.WriteString(line)
			sb.WriteByte('\n')
			continue
		}
		switch ev.Action {
		case "output":
			sb.WriteString(ev.Output)
		case "pass":
			if ev.Test != "" {
				results = append(results, TestResult{Name: ev.Test, Passed: true})
			}
		case "fail":
			if ev.Test != "" {
				results = append(results, TestResult{Name: ev.Test, Passed: false})
			}
		}
	}
	humanOutput = sb.String()
	return
}

// LeafFailures returns only the most specific failing tests.
// When TestFoo/sub1 fails, both TestFoo and TestFoo/sub1 appear as failed;
// this function returns only TestFoo/sub1 (the leaf).
func LeafFailures(results []TestResult) []TestResult {
	failing := make(map[string]bool)
	for _, r := range results {
		if !r.Passed {
			failing[r.Name] = true
		}
	}
	var leaves []TestResult
	for _, r := range results {
		if r.Passed {
			continue
		}
		isParent := false
		for name := range failing {
			if name != r.Name && strings.HasPrefix(name, r.Name+"/") {
				isParent = true
				break
			}
		}
		if !isParent {
			leaves = append(leaves, r)
		}
	}
	return leaves
}

// TopLevelPassing returns only top-level (non-subtest) passing tests.
func TopLevelPassing(results []TestResult) []TestResult {
	var out []TestResult
	for _, r := range results {
		if r.Passed && !strings.Contains(r.Name, "/") {
			out = append(out, r)
		}
	}
	return out
}
