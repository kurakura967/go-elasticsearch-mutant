package analyzer

import "go/ast"

// CallSite represents a detected field assignment of the go-elasticsearch Typed API.
type CallSite struct {
	File     string   // absolute path to the source file
	Line     int      // line number of the field assignment
	NodeType string   // struct type name, e.g. "BoolQuery", "NumberRangeQuery"; "RangeMap" for map[string]any
	Field    string   // field name, e.g. "Must", "Gte"; lowercase key e.g. "gte" for map sites
	FuncName string   // enclosing function name, e.g. "BuildSearchByCategory"
	Node     ast.Node // the *ast.KeyValueExpr node (used by rewriter)
	IsMapKey bool     // true when the target is a map[string]any string key, not a struct field
}
