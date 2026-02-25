package nethttp

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/dave/dst/decorator"
)

func TestHasHttpServerLiterals(t *testing.T) {
	src := `package main

import (
	"net/http"
	"time"
)

func main() {
	handler := http.NotFoundHandler()

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	server.ListenAndServe()
}
`
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}

	result := hasHttpServerLiterals(f, "http")
	if !result {
		t.Error("hasHttpServerLiterals should return true for http.Server{Handler: handler}")
	}
}

func TestHasHttpServerLiteralsNoHandler(t *testing.T) {
	src := `package main

import "net/http"

func main() {
	server := &http.Server{
		Addr: ":8080",
	}
	server.ListenAndServe()
}
`
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}

	result := hasHttpServerLiterals(f, "http")
	if result {
		t.Error("hasHttpServerLiterals should return false when no Handler field")
	}
}

func TestHasHttpServerLiteralsNilHandler(t *testing.T) {
	src := `package main

import "net/http"

func main() {
	server := &http.Server{
		Addr:    ":8080",
		Handler: nil,
	}
	server.ListenAndServe()
}
`
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}

	result := hasHttpServerLiterals(f, "http")
	if result {
		t.Error("hasHttpServerLiterals should return false when Handler is nil")
	}
}

func TestInjectServerHandlers(t *testing.T) {
	src := `package main

import (
	"net/http"
	"time"
)

func main() {
	handler := http.NotFoundHandler()

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	server.ListenAndServe()
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

	if !strings.Contains(output, "whataphttp.WrapHandler") {
		t.Error("Output should contain whataphttp.WrapHandler")
	}
}

func TestInjectServerHandlersWithFramework(t *testing.T) {
	src := `package main

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	_ = r

	server := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}
	server.ListenAndServe()
}
`
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}

	tr := &Transformer{}
	// Detect should still return true (for server literals)
	detected := tr.Detect(f)
	if !detected {
		t.Log("Detect returned false (framework import skips server detection)")
	}

	// Even if detected, inject should skip due to framework import
	tr.transformed = false
	tr.injectServerHandlers(f, "http")
	if tr.transformed {
		t.Error("injectServerHandlers should skip when gin import exists")
	}
}

func TestInjectHandle(t *testing.T) {
	src := `package main

import "net/http"

func main() {
	http.Handle("/api/", http.StripPrefix("/api", http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	http.ListenAndServe(":8080", nil)
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
		t.Error("Inject should return true")
	}

	restorer := decorator.NewRestorer()
	var buf bytes.Buffer
	if err := restorer.Fprint(&buf, f); err != nil {
		t.Fatal("Fprint error:", err)
	}
	output := buf.String()
	t.Log("Output:\n" + output)

	if !strings.Contains(output, "whataphttp.WrapHandler") {
		t.Error("http.Handle() should be wrapped with whataphttp.WrapHandler")
	}
	if !strings.Contains(output, "whataphttp.Func") {
		t.Error("http.HandleFunc() should be wrapped with whataphttp.Func")
	}
}

func TestRemoveHandle(t *testing.T) {
	src := `package main

import (
	"net/http"
	whataphttp "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
)

func main() {
	http.Handle("/api/", whataphttp.WrapHandler(http.StripPrefix("/api", http.FileServer(http.Dir("./static")))))
	http.HandleFunc("/health", whataphttp.Func(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	http.ListenAndServe(":8080", nil)
}
`
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}

	tr := &Transformer{}
	tr.Remove(f)

	restorer := decorator.NewRestorer()
	var buf bytes.Buffer
	if err := restorer.Fprint(&buf, f); err != nil {
		t.Fatal("Fprint error:", err)
	}
	output := buf.String()
	t.Log("Output:\n" + output)

	if strings.Contains(output, "whataphttp.WrapHandler") {
		t.Error("whataphttp.WrapHandler should be removed")
	}
	if strings.Contains(output, "whataphttp.Func") {
		t.Error("whataphttp.Func should be removed")
	}
	if !strings.Contains(output, "http.StripPrefix") {
		t.Error("Original http.StripPrefix should be restored")
	}
}

func TestRemoveServerHandlerWrappers(t *testing.T) {
	src := `package main

import (
	"net/http"
	"time"
	whataphttp "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
)

func main() {
	handler := http.NotFoundHandler()

	server := &http.Server{
		Addr:         ":8080",
		Handler:      whataphttp.WrapHandler(handler),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	server.ListenAndServe()
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

	if strings.Contains(output, "whataphttp.WrapHandler") {
		t.Error("Output should NOT contain whataphttp.WrapHandler after Remove")
	}
	if !strings.Contains(output, "Handler:      handler,") && !strings.Contains(output, "Handler: handler") {
		t.Error("Output should have Handler: handler (restored)")
	}
}
