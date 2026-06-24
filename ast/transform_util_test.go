package ast

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// §254 회귀 — nested composite literal 의 element (Type 없는) 가
// `{...}` placeholder 가 아닌 실제 source 로 render 되어야. 이전엔
// `[]Msg{{Role: "x"}, {Role: "y"}}` 같은 nested struct 가
// `[]Msg{{...}, {...}}` 로 출력 → parser 가 `...` 토큰으로 오인식 →
// Transform 의 parseCodeBlock 실패 → sashabaranov CreateChatCompletion
// 등 자동 inject 변환 누락 (embed 만 매칭됐던 원인).
func TestNodeToString_NestedCompositeLit(t *testing.T) {
	src := `package main
type Msg struct {
	Role    string
	Content string
}
type Req struct {
	Model    string
	Messages []Msg
}
var _ = Req{Model: "gpt-4o", Messages: []Msg{{Role: "system", Content: "x"}, {Role: "user", Content: "y"}}}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	df, err := decorator.DecorateFile(fset, f)
	if err != nil {
		t.Fatalf("decorate: %v", err)
	}

	var lit *dst.CompositeLit
	dst.Inspect(df, func(n dst.Node) bool {
		if cl, ok := n.(*dst.CompositeLit); ok && lit == nil {
			lit = cl
			return false
		}
		return true
	})
	if lit == nil {
		t.Fatalf("composite literal not found")
	}

	got := nodeToString(lit)
	if strings.Contains(got, "{...}") {
		t.Errorf("nodeToString must NOT emit placeholder \"{...}\" for nested literals — got %q", got)
	}
	// Re-parse the result to ensure it is valid Go source.
	wrapped := "package x\nvar _ = " + got + "\n"
	if _, err := parser.ParseFile(token.NewFileSet(), "test.go", wrapped, 0); err != nil {
		t.Errorf("nodeToString output is not parseable Go source: %v\ngot: %s", err, got)
	}
}
