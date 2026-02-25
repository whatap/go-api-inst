package fasthttp

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/dave/dst/decorator"
)

func TestHasFasthttpServerLiterals(t *testing.T) {
	src := `package main

import (
	"github.com/valyala/fasthttp"
	"time"
)

func main() {
	handler := func(ctx *fasthttp.RequestCtx) {}

	server := &fasthttp.Server{
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	server.ListenAndServe(":8080")
}
`
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}

	result := hasFasthttpServerLiterals(f, "fasthttp")
	if !result {
		t.Error("hasFasthttpServerLiterals should return true for fasthttp.Server{Handler: handler}")
	}
}

func TestHasFasthttpServerLiteralsNoHandler(t *testing.T) {
	src := `package main

import "github.com/valyala/fasthttp"

func main() {
	server := &fasthttp.Server{
		Name: "test",
	}
	server.ListenAndServe(":8080")
}
`
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}

	result := hasFasthttpServerLiterals(f, "fasthttp")
	if result {
		t.Error("hasFasthttpServerLiterals should return false when no Handler field")
	}
}

func TestHasFasthttpServerLiteralsNilHandler(t *testing.T) {
	src := `package main

import "github.com/valyala/fasthttp"

func main() {
	server := &fasthttp.Server{
		Handler: nil,
	}
	server.ListenAndServe(":8080")
}
`
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}

	result := hasFasthttpServerLiterals(f, "fasthttp")
	if result {
		t.Error("hasFasthttpServerLiterals should return false when Handler is nil")
	}
}

func TestInjectFasthttpServerHandlers(t *testing.T) {
	src := `package main

import (
	"github.com/valyala/fasthttp"
	"time"
)

func main() {
	handler := func(ctx *fasthttp.RequestCtx) {}

	server := &fasthttp.Server{
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	server.ListenAndServe(":8080")
}
`
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}

	tr := &Transformer{}
	detected := tr.Detect(f)
	if !detected {
		t.Fatal("Detect should return true")
	}

	transformed, err := tr.Inject(f)
	if err != nil {
		t.Fatal("Inject error:", err)
	}
	if !transformed {
		t.Error("Inject should return true (transformation occurred)")
	}

	// Print the output to verify WrapHandler is present
	restorer := decorator.NewRestorer()
	var buf bytes.Buffer
	if err := restorer.Fprint(&buf, f); err != nil {
		t.Fatal("Fprint error:", err)
	}
	output := buf.String()
	t.Log("Output:\n" + output)

	if !strings.Contains(output, "whatapfasthttp.WrapHandler") {
		t.Error("Output should contain whatapfasthttp.WrapHandler")
	}
}

func TestRemoveFasthttpServerHandlerWrappers(t *testing.T) {
	src := `package main

import (
	"github.com/valyala/fasthttp"
	"time"
	whatapfasthttp "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp"
)

func main() {
	handler := func(ctx *fasthttp.RequestCtx) {}

	server := &fasthttp.Server{
		Handler:      whatapfasthttp.WrapHandler(handler),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	server.ListenAndServe(":8080")
}
`
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}

	tr := &Transformer{}
	tr.Remove(f)

	// Print the output to verify WrapHandler is removed
	restorer := decorator.NewRestorer()
	var buf bytes.Buffer
	if err := restorer.Fprint(&buf, f); err != nil {
		t.Fatal("Fprint error:", err)
	}
	output := buf.String()
	t.Log("Output:\n" + output)

	if strings.Contains(output, "whatapfasthttp.WrapHandler") {
		t.Error("Output should NOT contain whatapfasthttp.WrapHandler after Remove")
	}
	if !strings.Contains(output, "Handler:      handler,") && !strings.Contains(output, "Handler: handler") {
		t.Error("Output should have Handler: handler (restored)")
	}
}
