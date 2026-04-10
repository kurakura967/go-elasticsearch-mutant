package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/mutant"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/report"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/runner"
)

func makeMutants() []*mutant.Mutant {
	return []*mutant.Mutant{
		{ID: 1, Operator: "RemoveClause", Description: "BoolQuery.Must → nil",
			Site: &analyzer.CallSite{File: "/proj/search.go", Line: 41}},
		{ID: 2, Operator: "MustToShould", Description: "BoolQuery.Must → Should",
			Site: &analyzer.CallSite{File: "/proj/search.go", Line: 41}},
		{ID: 3, Operator: "RangeBoundary", Description: "NumberRangeQuery.Gte → Gt",
			Site: &analyzer.CallSite{File: "/proj/search.go", Line: 80}},
	}
}

func makeResults() []runner.Result {
	return []runner.Result{
		{MutantID: 1, Status: runner.Killed},
		{MutantID: 2, Status: runner.Survived},
		{MutantID: 3, Status: runner.Survived},
	}
}

// --- Build tests ---

func TestBuild_Summary(t *testing.T) {
	sum, _ := report.Build("/proj", makeMutants(), makeResults())

	if sum.Total != 3 {
		t.Errorf("Total: got %d, want 3", sum.Total)
	}
	if sum.Killed != 1 {
		t.Errorf("Killed: got %d, want 1", sum.Killed)
	}
	if sum.Survived != 2 {
		t.Errorf("Survived: got %d, want 2", sum.Survived)
	}
}

func TestBuild_Score(t *testing.T) {
	sum, _ := report.Build("/proj", makeMutants(), makeResults())
	want := 100.0 / 3.0
	if sum.Score() < want-0.01 || sum.Score() > want+0.01 {
		t.Errorf("Score: got %.2f, want ~%.2f", sum.Score(), want)
	}
}

func TestBuild_Score_Empty(t *testing.T) {
	sum, _ := report.Build("/proj", nil, nil)
	if sum.Score() != 0 {
		t.Errorf("Score of empty run: got %.2f, want 0", sum.Score())
	}
}

func TestBuild_RelativePaths(t *testing.T) {
	_, details := report.Build("/proj", makeMutants(), makeResults())
	for _, d := range details {
		if strings.HasPrefix(d.File, "/") {
			t.Errorf("File should be relative, got %q", d.File)
		}
	}
}

func TestBuild_MetadataCorrelated(t *testing.T) {
	_, details := report.Build("/proj", makeMutants(), makeResults())

	byID := map[int]report.MutantDetail{}
	for _, d := range details {
		byID[d.ID] = d
	}

	if byID[1].Operator != "RemoveClause" {
		t.Errorf("ID 1 Operator: got %q", byID[1].Operator)
	}
	if byID[2].Status != runner.Survived {
		t.Errorf("ID 2 Status: got %v", byID[2].Status)
	}
	if byID[3].Line != 80 {
		t.Errorf("ID 3 Line: got %d, want 80", byID[3].Line)
	}
}

// --- PrintConsole tests ---

func TestPrintConsole_ContainsMutationScore(t *testing.T) {
	sum, details := report.Build("/proj", makeMutants(), makeResults())
	var buf bytes.Buffer
	report.PrintConsole(&buf, sum, details, false)

	if !strings.Contains(buf.String(), "Mutation Score:") {
		t.Errorf("missing 'Mutation Score:' in output:\n%s", buf.String())
	}
}

func TestPrintConsole_SurvivedSection(t *testing.T) {
	sum, details := report.Build("/proj", makeMutants(), makeResults())
	var buf bytes.Buffer
	report.PrintConsole(&buf, sum, details, false)

	out := buf.String()
	if !strings.Contains(out, "SURVIVED (2)") {
		t.Errorf("expected 'SURVIVED (2)' in output:\n%s", out)
	}
	if !strings.Contains(out, "MustToShould") {
		t.Errorf("expected operator name in SURVIVED section:\n%s", out)
	}
}

func TestPrintConsole_NoSurvivedSection_WhenAllKilled(t *testing.T) {
	mutants := makeMutants()[:1]
	results := []runner.Result{{MutantID: 1, Status: runner.Killed}}
	sum, details := report.Build("/proj", mutants, results)

	var buf bytes.Buffer
	report.PrintConsole(&buf, sum, details, false)

	if strings.Contains(buf.String(), "SURVIVED") {
		t.Errorf("SURVIVED section should be absent when all mutants are killed:\n%s", buf.String())
	}
}

// --- PrintJSON tests ---

func TestPrintJSON_ValidJSON(t *testing.T) {
	sum, details := report.Build("/proj", makeMutants(), makeResults())
	var buf bytes.Buffer
	if err := report.PrintJSON(&buf, sum, details); err != nil {
		t.Fatalf("PrintJSON: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
}

func TestPrintJSON_Fields(t *testing.T) {
	sum, details := report.Build("/proj", makeMutants(), makeResults())
	var buf bytes.Buffer
	report.PrintJSON(&buf, sum, details)

	var out struct {
		Score    float64 `json:"score"`
		Total    int     `json:"total"`
		Killed   int     `json:"killed"`
		Survived int     `json:"survived"`
		Mutants  []struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
		} `json:"mutants"`
	}
	json.Unmarshal(buf.Bytes(), &out)

	if out.Total != 3 || out.Killed != 1 || out.Survived != 2 {
		t.Errorf("counts: Total=%d Killed=%d Survived=%d", out.Total, out.Killed, out.Survived)
	}
	if len(out.Mutants) != 3 {
		t.Errorf("mutants: got %d, want 3", len(out.Mutants))
	}
}
