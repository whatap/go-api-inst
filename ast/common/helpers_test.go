package common

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// parseSrcHelper parses Go source into a dst.File for testing
func parseSrcHelper(t *testing.T, src string) *dst.File {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	dstFile, err := decorator.DecorateFile(fset, f)
	if err != nil {
		t.Fatalf("decorate error: %v", err)
	}
	return dstFile
}

// getFirstStmt returns the first statement from the first function declaration
func getFirstStmt(t *testing.T, file *dst.File) dst.Stmt {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if len(fn.Body.List) > 0 {
			return fn.Body.List[0]
		}
	}
	t.Fatal("no statement found in file")
	return nil
}

// §213 HIGH 3: IsWhatapMiddlewareCall precision matching tests

func TestIsWhatapMiddlewareCall_Positive_CallPattern(t *testing.T) {
	// Pattern 1: r.Use(whatapgin.Middleware())
	src := `package main

import "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"

func setup() {
	r.Use(whatapgin.Middleware())
}
`
	file := parseSrcHelper(t, src)
	stmt := getFirstStmt(t, file)

	if !IsWhatapMiddlewareCall(stmt, "whatapgin", "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin") {
		t.Error("should match whatapgin.Middleware() call pattern")
	}
}

func TestIsWhatapMiddlewareCall_Positive_ValuePattern(t *testing.T) {
	// Pattern 2: r.Use(whatapchi.Middleware)
	src := `package main

import "github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi"

func setup() {
	r.Use(whatapchi.Middleware)
}
`
	file := parseSrcHelper(t, src)
	stmt := getFirstStmt(t, file)

	if !IsWhatapMiddlewareCall(stmt, "whatapchi", "github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi") {
		t.Error("should match whatapchi.Middleware value pattern")
	}
}

func TestIsWhatapMiddlewareCall_Negative_WrongWhatapPkg(t *testing.T) {
	// whatapgin.Middleware() should NOT match when checking for whatapecho
	src := `package main

import "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"

func setup() {
	r.Use(whatapgin.Middleware())
}
`
	file := parseSrcHelper(t, src)
	stmt := getFirstStmt(t, file)

	// Asking for whatapecho, but the code has whatapgin → should NOT match
	if IsWhatapMiddlewareCall(stmt, "whatapecho", "github.com/whatap/go-api/instrumentation/github.com/labstack/echo/v4/whatapecho") {
		t.Error("whatapgin.Middleware() should NOT match when checking for whatapecho")
	}
}

func TestIsWhatapMiddlewareCall_Negative_NotMiddleware(t *testing.T) {
	// r.Use(whatapgin.SomethingElse()) → not Middleware
	src := `package main

import "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"

func setup() {
	r.Use(whatapgin.SomethingElse())
}
`
	file := parseSrcHelper(t, src)
	stmt := getFirstStmt(t, file)

	if IsWhatapMiddlewareCall(stmt, "whatapgin", "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin") {
		t.Error("should NOT match non-Middleware function call")
	}
}

func TestIsWhatapMiddlewareCall_Negative_NotUse(t *testing.T) {
	// r.Get(whatapgin.Middleware()) → not Use method
	src := `package main

import "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"

func setup() {
	r.Get(whatapgin.Middleware())
}
`
	file := parseSrcHelper(t, src)
	stmt := getFirstStmt(t, file)

	if IsWhatapMiddlewareCall(stmt, "whatapgin", "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin") {
		t.Error("should NOT match when method is not Use")
	}
}

func TestIsWhatapMiddlewareCall_Negative_MultipleArgs(t *testing.T) {
	// r.Use(whatapgin.Middleware(), otherMiddleware) → 2 args
	src := `package main

import "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"

func setup() {
	r.Use(whatapgin.Middleware(), otherMiddleware)
}
`
	file := parseSrcHelper(t, src)
	stmt := getFirstStmt(t, file)

	if IsWhatapMiddlewareCall(stmt, "whatapgin", "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin") {
		t.Error("should NOT match when Use has multiple arguments")
	}
}

func TestIsWhatapMiddlewareCall_Negative_NonExprStmt(t *testing.T) {
	// x := r.Use(whatapgin.Middleware()) → AssignStmt, not ExprStmt
	src := `package main

import "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"

func setup() {
	x := whatapgin.Middleware()
	_ = x
}
`
	file := parseSrcHelper(t, src)
	stmt := getFirstStmt(t, file)

	if IsWhatapMiddlewareCall(stmt, "whatapgin", "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin") {
		t.Error("should NOT match AssignStmt")
	}
}
