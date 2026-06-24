package ast

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// parseCodeBlock parses a Go code string into DST statements.
// Copied from ast/custom/util.go — v2 packages are independent.
func parseCodeBlock(code string) ([]dst.Stmt, error) {
	if code == "" {
		return nil, nil
	}

	wrapped := "package p\nfunc f() {\n" + code + "\n}"

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", wrapped, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	dstFile, err := decorator.DecorateFile(fset, f)
	if err != nil {
		return nil, err
	}

	fn := dstFile.Decls[0].(*dst.FuncDecl)
	return fn.Body.List, nil
}

// nodeToString converts a DST node to its string representation.
// Copied from ast/custom/transform.go — v2 packages are independent.
func nodeToString(node dst.Node) string {
	if node == nil {
		return ""
	}

	switch n := node.(type) {
	case *dst.Ident:
		return n.Name
	case *dst.CallExpr:
		fnStr := nodeToString(n.Fun)
		argsStr := argsToString(n.Args)
		if n.Ellipsis {
			return fmt.Sprintf("%s(%s...)", fnStr, argsStr)
		}
		return fmt.Sprintf("%s(%s)", fnStr, argsStr)
	case *dst.SelectorExpr:
		return fmt.Sprintf("%s.%s", nodeToString(n.X), n.Sel.Name)
	case *dst.BasicLit:
		return n.Value
	case *dst.UnaryExpr:
		return fmt.Sprintf("%s%s", n.Op.String(), nodeToString(n.X))
	case *dst.BinaryExpr:
		return fmt.Sprintf("%s %s %s", nodeToString(n.X), n.Op.String(), nodeToString(n.Y))
	case *dst.StarExpr:
		return fmt.Sprintf("*%s", nodeToString(n.X))
	case *dst.CompositeLit:
		elts := make([]string, len(n.Elts))
		for i, elt := range n.Elts {
			elts[i] = nodeToString(elt)
		}
		// §254 — Type 없는 nested composite literal (e.g., slice element
		// `{Role: "system"}`) 도 정확히 render. 이전엔 "{...}" placeholder
		// 였는데 parser 가 `...` 토큰으로 오인식해서 sashabaranov
		// CreateChatCompletion 등 nested struct 인자 변환이 실패했음.
		if n.Type != nil {
			return fmt.Sprintf("%s{%s}", nodeToString(n.Type), strings.Join(elts, ", "))
		}
		return fmt.Sprintf("{%s}", strings.Join(elts, ", "))
	case *dst.KeyValueExpr:
		return fmt.Sprintf("%s: %s", nodeToString(n.Key), nodeToString(n.Value))
	case *dst.IndexExpr:
		return fmt.Sprintf("%s[%s]", nodeToString(n.X), nodeToString(n.Index))
	case *dst.SliceExpr:
		return fmt.Sprintf("%s[%s:%s]", nodeToString(n.X), nodeToString(n.Low), nodeToString(n.High))
	case *dst.ParenExpr:
		return fmt.Sprintf("(%s)", nodeToString(n.X))
	case *dst.FuncLit:
		return "func(){...}"
	case *dst.ArrayType:
		return fmt.Sprintf("[]%s", nodeToString(n.Elt))
	case *dst.MapType:
		return fmt.Sprintf("map[%s]%s", nodeToString(n.Key), nodeToString(n.Value))
	case *dst.Ellipsis:
		if n.Elt != nil {
			return fmt.Sprintf("...%s", nodeToString(n.Elt))
		}
		return "..."
	default:
		_ = bytes.Buffer{}
		return "<unknown>"
	}
}

// argsToString converts an argument list to a comma-separated string.
func argsToString(args []dst.Expr) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = nodeToString(arg)
	}
	return strings.Join(parts, ", ")
}
