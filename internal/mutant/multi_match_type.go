package mutant

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"

	"github.com/kurakura967/go-elasticsearch-mutant/internal/analyzer"
)

// multiMatchTypeTargets lists the alternative types to substitute for Bestfields.
// Phrase and Mostfields are chosen because they produce meaningfully different
// matching behavior that a well-written integration test should be able to detect.
var multiMatchTypeTargets = []string{"Phrase", "Mostfields"}

// MultiMatchType replaces MultiMatchQuery.Type from Bestfields to alternative
// match types, testing that tests verify the correct multi-match behavior.
// Only applied when the current value is Bestfields.
type MultiMatchType struct{}

func (m *MultiMatchType) Name() string { return "MultiMatchType" }

func (m *MultiMatchType) Apply(site *analyzer.CallSite) ([]*Mutant, error) {
	if site.NodeType != "MultiMatchQuery" || site.Field != "Type" {
		return nil, nil
	}

	current, ok := readTypeSelectorName(site)
	if !ok || current != "Bestfields" {
		return nil, nil
	}

	var mutants []*Mutant
	for _, target := range multiMatchTypeTargets {
		targetName := target
		src, err := applyRewrite(site, func(kv *ast.KeyValueExpr) {
			setTypeSelectorName(kv, targetName)
		})
		if err != nil {
			return nil, err
		}
		mutants = append(mutants, &Mutant{
			Site:        site,
			Operator:    m.Name(),
			Description: fmt.Sprintf("MultiMatchQuery.Type: Bestfields → %s", targetName),
			ModifiedSrc: src,
		})
	}
	return mutants, nil
}

// readTypeSelectorName reads the selector name from the value at site.Line/site.Field,
// e.g. "Bestfields" from &textquerytype.Bestfields.
func readTypeSelectorName(site *analyzer.CallSite) (string, bool) {
	src, err := os.ReadFile(site.File)
	if err != nil {
		return "", false
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, site.File, src, 0)
	if err != nil {
		return "", false
	}

	name := ""
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		id, ok := kv.Key.(*ast.Ident)
		if !ok {
			return true
		}
		if fset.Position(kv.Pos()).Line != site.Line || id.Name != site.Field {
			return true
		}
		name, ok = extractSelectorName(kv.Value)
		found = true
		return false
	})
	return name, found && name != ""
}

// extractSelectorName extracts the identifier name from &pkg.Name or pkg.Name.
func extractSelectorName(expr ast.Expr) (string, bool) {
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	return sel.Sel.Name, true
}

// setTypeSelectorName replaces the selector name in the KV value with targetName.
func setTypeSelectorName(kv *ast.KeyValueExpr, targetName string) {
	expr := kv.Value
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		sel.Sel.Name = targetName
	}
}
