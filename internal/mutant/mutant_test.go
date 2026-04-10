package mutant_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
	"github.com/kurakura967/go-elasticsearch-mutant/internal/mutant"
)

// --- test fixtures ---

const boolSrc = `package test

func build() {
	_ = BoolQuery{
		Must:    foo(),
		Should:  foo(),
		Filter:  bar(),
		MustNot: baz(),
	}
}
`

// filterOnlySrc is a BoolQuery with Filter but no Must,
// used to test FilterToMust without triggering the sibling-field guard.
const filterOnlySrc = `package test

func build() {
	_ = BoolQuery{
		Filter: bar(),
	}
}
`

const rangeSrc = `package test

func build() {
	_ = NumberRangeQuery{
		Gte: &min,
		Lte: &max,
	}
}
`

// --- helpers ---

// writeFixture writes src to a temp file and returns its path.
func writeFixture(t *testing.T, src string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "fix.go")
	if err := os.WriteFile(f, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	return f
}

// lineOf returns the 1-based line number of the first line containing substr.
func lineOf(src, substr string) int {
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, substr) {
			return i + 1
		}
	}
	return -1
}

// makeSite builds a CallSite for a field in src (the content used for lineOf).
func makeSite(file, src, field, nodeType string) *analyzer.CallSite {
	return &analyzer.CallSite{
		File:     file,
		Line:     lineOf(src, field+":"),
		Field:    field,
		NodeType: nodeType,
	}
}

// hasField reports whether the Go source contains a KeyValueExpr whose key is fieldName.
func hasField(src, fieldName string) bool {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		return false
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		if id, ok := kv.Key.(*ast.Ident); ok && id.Name == fieldName {
			found = true
		}
		return !found
	})
	return found
}

// isNilField reports whether the first KeyValueExpr with key fieldName has value nil.
func isNilField(src, fieldName string) bool {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		return false
	}
	result := false
	ast.Inspect(f, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != fieldName {
			return true
		}
		val, ok := kv.Value.(*ast.Ident)
		result = ok && val.Name == "nil"
		return false
	})
	return result
}

// --- operator tests ---

func TestRemoveClause(t *testing.T) {
	for _, field := range []string{"Must", "Should"} {
		t.Run(field, func(t *testing.T) {
			f := writeFixture(t, boolSrc)
			ms, err := (&mutant.RemoveClause{}).Apply(makeSite(f, boolSrc, field, "BoolQuery"))
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if len(ms) != 1 {
				t.Fatalf("want 1 mutant, got %d", len(ms))
			}
			got := string(ms[0].ModifiedSrc)
			if !isNilField(got, field) {
				t.Errorf("%s: want nil value, got:\n%s", field, got)
			}
		})
	}

	t.Run("skip_Filter", func(t *testing.T) {
		f := writeFixture(t, boolSrc)
		ms, _ := (&mutant.RemoveClause{}).Apply(makeSite(f, boolSrc, "Filter", "BoolQuery"))
		if len(ms) != 0 {
			t.Errorf("RemoveClause must not apply to Filter, got %d mutant(s)", len(ms))
		}
	})

	t.Run("skip_MustNot", func(t *testing.T) {
		f := writeFixture(t, boolSrc)
		ms, _ := (&mutant.RemoveClause{}).Apply(makeSite(f, boolSrc, "MustNot", "BoolQuery"))
		if len(ms) != 0 {
			t.Errorf("RemoveClause must not apply to MustNot, got %d mutant(s)", len(ms))
		}
	})

	t.Run("skip_non_BoolQuery", func(t *testing.T) {
		f := writeFixture(t, boolSrc)
		ms, _ := (&mutant.RemoveClause{}).Apply(makeSite(f, boolSrc, "Must", "OtherQuery"))
		if len(ms) != 0 {
			t.Errorf("RemoveClause must not apply to non-BoolQuery, got %d mutant(s)", len(ms))
		}
	})
}

func TestFilterToMust(t *testing.T) {
	t.Run("Filter_to_Must", func(t *testing.T) {
		// Use filterOnlySrc (no Must sibling) so the guard does not trigger.
		f := writeFixture(t, filterOnlySrc)
		ms, err := (&mutant.FilterToMust{}).Apply(makeSite(f, filterOnlySrc, "Filter", "BoolQuery"))
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if len(ms) != 1 {
			t.Fatalf("want 1 mutant, got %d", len(ms))
		}
		if ms[0].SkipReason != "" {
			t.Fatalf("expected non-skipped mutant, got skip reason: %s", ms[0].SkipReason)
		}
		got := string(ms[0].ModifiedSrc)
		if !hasField(got, "Must") {
			t.Errorf("expected Must field in output:\n%s", got)
		}
		if hasField(got, "Filter") {
			t.Errorf("unexpected Filter field still present:\n%s", got)
		}
	})

	t.Run("skip_when_Must_sibling_exists", func(t *testing.T) {
		// boolSrc has both Must and Filter → guard should trigger.
		f := writeFixture(t, boolSrc)
		ms, err := (&mutant.FilterToMust{}).Apply(makeSite(f, boolSrc, "Filter", "BoolQuery"))
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if len(ms) != 1 {
			t.Fatalf("want 1 skipped mutant, got %d", len(ms))
		}
		if ms[0].SkipReason == "" {
			t.Errorf("expected SkipReason to be set, got empty")
		}
		if ms[0].ModifiedSrc != nil {
			t.Errorf("expected ModifiedSrc to be nil for skipped mutant")
		}
	})

	t.Run("inapplicable_Must_field", func(t *testing.T) {
		f := writeFixture(t, boolSrc)
		ms, _ := (&mutant.FilterToMust{}).Apply(makeSite(f, boolSrc, "Must", "BoolQuery"))
		if len(ms) != 0 {
			t.Errorf("FilterToMust must not apply to Must, got %d mutant(s)", len(ms))
		}
	})

	t.Run("inapplicable_non_BoolQuery", func(t *testing.T) {
		f := writeFixture(t, boolSrc)
		ms, _ := (&mutant.FilterToMust{}).Apply(makeSite(f, boolSrc, "Filter", "OtherQuery"))
		if len(ms) != 0 {
			t.Errorf("FilterToMust must not apply to non-BoolQuery, got %d mutant(s)", len(ms))
		}
	})
}

func TestMustToShould(t *testing.T) {
	t.Run("Must_to_Should", func(t *testing.T) {
		f := writeFixture(t, boolSrc)
		ms, err := (&mutant.MustToShould{}).Apply(makeSite(f, boolSrc, "Must", "BoolQuery"))
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if len(ms) != 1 {
			t.Fatalf("want 1 mutant, got %d", len(ms))
		}
		got := string(ms[0].ModifiedSrc)
		if !hasField(got, "Should") {
			t.Errorf("expected Should field in output:\n%s", got)
		}
		if hasField(got, "Must") {
			t.Errorf("unexpected Must field still present:\n%s", got)
		}
	})

	t.Run("skip_Filter", func(t *testing.T) {
		f := writeFixture(t, boolSrc)
		ms, _ := (&mutant.MustToShould{}).Apply(makeSite(f, boolSrc, "Filter", "BoolQuery"))
		if len(ms) != 0 {
			t.Errorf("MustToShould must not apply to Filter, got %d mutant(s)", len(ms))
		}
	})
}

func TestRangeBoundary(t *testing.T) {
	tests := []struct {
		field    string
		wantKey  string
		skipKey  string
	}{
		{"Gte", "Gt", "Gte"},
		{"Lte", "Lt", "Lte"},
	}
	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			f := writeFixture(t, rangeSrc)
			ms, err := (&mutant.RangeBoundary{}).Apply(makeSite(f, rangeSrc, tc.field, "NumberRangeQuery"))
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if len(ms) != 1 {
				t.Fatalf("want 1 mutant, got %d", len(ms))
			}
			got := string(ms[0].ModifiedSrc)
			if !hasField(got, tc.wantKey) {
				t.Errorf("expected field %q in output:\n%s", tc.wantKey, got)
			}
			if hasField(got, tc.skipKey) {
				t.Errorf("original field %q still present:\n%s", tc.skipKey, got)
			}
		})
	}

	t.Run("skip_BoolQuery", func(t *testing.T) {
		f := writeFixture(t, boolSrc)
		ms, _ := (&mutant.RangeBoundary{}).Apply(makeSite(f, boolSrc, "Must", "BoolQuery"))
		if len(ms) != 0 {
			t.Errorf("RangeBoundary must not apply to BoolQuery, got %d mutant(s)", len(ms))
		}
	})
}

func TestRemoveMustNot(t *testing.T) {
	t.Run("MustNot_to_nil", func(t *testing.T) {
		f := writeFixture(t, boolSrc)
		ms, err := (&mutant.RemoveMustNot{}).Apply(makeSite(f, boolSrc, "MustNot", "BoolQuery"))
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if len(ms) != 1 {
			t.Fatalf("want 1 mutant, got %d", len(ms))
		}
		got := string(ms[0].ModifiedSrc)
		if !isNilField(got, "MustNot") {
			t.Errorf("MustNot: want nil value, got:\n%s", got)
		}
	})

	t.Run("skip_Must", func(t *testing.T) {
		f := writeFixture(t, boolSrc)
		ms, _ := (&mutant.RemoveMustNot{}).Apply(makeSite(f, boolSrc, "Must", "BoolQuery"))
		if len(ms) != 0 {
			t.Errorf("RemoveMustNot must not apply to Must, got %d mutant(s)", len(ms))
		}
	})
}

// --- Generate tests ---

func TestGenerate_IDs(t *testing.T) {
	f := writeFixture(t, boolSrc)
	sites := []*analyzer.CallSite{
		makeSite(f, boolSrc, "Must", "BoolQuery"),
		makeSite(f, boolSrc, "MustNot", "BoolQuery"),
	}

	ops := []mutant.Operator{
		&mutant.RemoveClause{},  // Must→nil (1)
		&mutant.MustToShould{}, // Must→Should (1)
		&mutant.RemoveMustNot{}, // MustNot→nil (1)
	}
	// Must site:    RemoveClause(1) + MustToShould(1) + RemoveMustNot(0) = 2
	// MustNot site: RemoveClause(0) + MustToShould(0) + RemoveMustNot(1) = 1
	// Total: 3

	ms, err := mutant.Generate(sites, ops)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(ms) != 3 {
		t.Fatalf("want 3 mutants, got %d", len(ms))
	}
	for i, m := range ms {
		if m.ID != i+1 {
			t.Errorf("ms[%d].ID = %d, want %d", i, m.ID, i+1)
		}
	}
}

func TestGenerate_Metadata(t *testing.T) {
	f := writeFixture(t, boolSrc)
	sites := []*analyzer.CallSite{
		makeSite(f, boolSrc, "Must", "BoolQuery"),
	}

	ms, err := mutant.Generate(sites, mutant.DefaultOperators)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	for _, m := range ms {
		if m.Operator == "" {
			t.Error("Operator field is empty")
		}
		if m.Description == "" {
			t.Error("Description field is empty")
		}
		if len(m.ModifiedSrc) == 0 {
			t.Error("ModifiedSrc is empty")
		}
		if m.Site == nil {
			t.Error("Site is nil")
		}
	}
}
