package analyzer_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

func TestAnalyze(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	exampleDir := filepath.Join(filepath.Dir(filename), "../../example")

	a := &analyzer.Analyzer{Dir: exampleDir}
	sites, err := a.Analyze(".")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	type want struct {
		nodeType string
		field    string
	}

	// Expected call sites in order of AST traversal (depth-first, line order)
	expected := []want{
		{"BoolQuery", "Must"},           // BuildActiveUsersQuery
		{"BoolQuery", "Filter"},         // BuildActiveUsersQuery
		{"BoolQuery", "Must"},           // BuildPriceRangeQuery
		{"BoolQuery", "Filter"},         // BuildPriceRangeQuery
		{"NumberRangeQuery", "Gte"},     // BuildPriceRangeQuery
		{"NumberRangeQuery", "Lte"},     // BuildPriceRangeQuery
		{"BoolQuery", "Must"},           // BuildArticlesQuery
		{"BoolQuery", "MustNot"},        // BuildArticlesQuery
		{"BoolQuery", "Must"},           // BuildUserByEmailQuery
	}

	if len(sites) != len(expected) {
		t.Fatalf("got %d call sites, want %d; sites:\n%v", len(sites), len(expected), formatSites(sites))
	}

	for i, site := range sites {
		w := expected[i]
		if site.NodeType != w.nodeType || site.Field != w.field {
			t.Errorf("site[%d]: got {%s, %s}, want {%s, %s}",
				i, site.NodeType, site.Field, w.nodeType, w.field)
		}
		if site.File == "" {
			t.Errorf("site[%d]: File is empty", i)
		}
		if site.Line == 0 {
			t.Errorf("site[%d]: Line is 0", i)
		}
		if site.Node == nil {
			t.Errorf("site[%d]: Node is nil", i)
		}
	}
}

func formatSites(sites []*analyzer.CallSite) string {
	s := ""
	for i, site := range sites {
		s += "\t" + string(rune('0'+i)) + ": " + site.NodeType + "." + site.Field + "\n"
	}
	return s
}
