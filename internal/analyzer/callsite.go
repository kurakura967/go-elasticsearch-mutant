package analyzer

import "go/ast"

// CallSite represents a detected field assignment of the go-elasticsearch Typed API.
type CallSite struct {
	File     string   // absolute path to the source file
	Line     int      // line number of the field assignment
	NodeType string   // struct type name, e.g. "BoolQuery", "NumberRangeQuery"
	Field    string   // field name, e.g. "Must", "Gte"
	FuncName string   // enclosing function name, e.g. "BuildSearchByCategory"
	Node     ast.Node // the *ast.KeyValueExpr node (used by rewriter)
}
