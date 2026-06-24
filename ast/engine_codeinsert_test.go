package ast

import (
	"strings"
	"testing"

	"github.com/dave/dst"
)

// §287 — CodeInsert "before" must insert exactly once, in the block that
// directly owns the matched call. The engine traverses each statement subtree
// (processStmt + dst.Inspect) under the outer block context, then recurses into
// nested blocks (processNestedBlocks). Without a guard, a call inside a nested
// if/for block gets matched twice — once at the outer (function-body) scope and
// once at the correct inner scope — producing a duplicate, out-of-scope
// insertion (`undefined: cfg` in teleport). These tests lock in the single,
// correctly-scoped insertion for nested, non-nested, and if-init shapes.

// k8sCodeInsertEngine builds an engine wired with the k8s NewForConfig
// CodeInsert rule and a resolver that recognises kubernetes.NewForConfig calls.
func k8sCodeInsertEngine() *Engine {
	reg := NewRegistry()
	reg.Register(&Rule{
		Target: "k8s.io/client-go/kubernetes.NewForConfig",
		Advice: &CodeInsert{
			WhatapPkg:   "github.com/whatap/go-api/instrumentation/k8s.io/client-go/kubernetes/whatapkubernetes",
			WhatapAlias: "whatapkubernetes",
			Position:    "before",
			ArgSource:   0,
			MethodName:  "Wrap",
			WhatapFunc:  "WrapRoundTripper",
		},
	})
	resolve := func(n dst.Node) string {
		call, ok := n.(*dst.CallExpr)
		if !ok {
			return ""
		}
		sel, ok := call.Fun.(*dst.SelectorExpr)
		if !ok {
			return ""
		}
		pkg, ok := sel.X.(*dst.Ident)
		if !ok {
			return ""
		}
		if pkg.Name == "kubernetes" && sel.Sel.Name == "NewForConfig" {
			return "k8s.io/client-go/kubernetes.NewForConfig"
		}
		return ""
	}
	return NewEngine(reg, ModeInject, resolve)
}

const wrapStmt = "cfg.Wrap(whatapkubernetes.WrapRoundTripper())"

// TestCodeInsert_NestedBlock — teleport-shape: NewForConfig(cfg) inside a nested
// if block where cfg is declared inside that same block. The insert must land
// inside the block (before the call), exactly once. A second, function-body
// level insert would reference cfg out of scope.
func TestCodeInsert_NestedBlock(t *testing.T) {
	src := `package p

func handler(needsClient bool) {
	if needsClient {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return
		}
		client, err := kubernetes.NewForConfig(cfg)
		_ = client
		_ = err
	}
}
`
	file := parseTestFile(t, src)
	e := k8sCodeInsertEngine()
	if !e.Process(file) {
		t.Fatal("expected a transformation")
	}
	got := fileToString(t, file)

	if n := strings.Count(got, wrapStmt); n != 1 {
		t.Fatalf("expected exactly 1 insertion, got %d:\n%s", n, got)
	}
	// The inserted Wrap must come AFTER cfg's declaration (in scope), and
	// immediately before the NewForConfig call.
	declIdx := strings.Index(got, "cfg, err := rest.InClusterConfig()")
	wrapIdx := strings.Index(got, wrapStmt)
	callIdx := strings.Index(got, "kubernetes.NewForConfig(cfg)")
	if !(declIdx < wrapIdx && wrapIdx < callIdx) {
		t.Errorf("insertion not between cfg decl and NewForConfig call:\n%s", got)
	}
}

// TestCodeInsert_NonNested — function-body level call (k8s-app k8sInClusterPattern
// shape). Already worked before §287; this guards against the fix regressing it.
func TestCodeInsert_NonNested(t *testing.T) {
	src := `package p

func handler() {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return
	}
	client, err := kubernetes.NewForConfig(cfg)
	_ = client
	_ = err
}
`
	file := parseTestFile(t, src)
	e := k8sCodeInsertEngine()
	if !e.Process(file) {
		t.Fatal("expected a transformation")
	}
	got := fileToString(t, file)

	if n := strings.Count(got, wrapStmt); n != 1 {
		t.Fatalf("expected exactly 1 insertion, got %d:\n%s", n, got)
	}
	wrapIdx := strings.Index(got, wrapStmt)
	callIdx := strings.Index(got, "kubernetes.NewForConfig(cfg)")
	if !(wrapIdx < callIdx && wrapIdx >= 0) {
		t.Errorf("insertion not before NewForConfig call:\n%s", got)
	}
}

// TestCodeInsert_IfInit — call inside an if-statement init clause:
//
//	if client, err := kubernetes.NewForConfig(cfg); err != nil { ... }
//
// The arg cfg is declared before the if, so inserting cfg.Wrap(...) before the
// IfStmt is valid and must happen exactly once. The Init is not a statement
// block, so the processNestedBlocks pass never revisits it.
func TestCodeInsert_IfInit(t *testing.T) {
	src := `package p

func handler(cfg *Config) {
	if client, err := kubernetes.NewForConfig(cfg); err != nil {
		_ = client
		return
	}
}
`
	file := parseTestFile(t, src)
	e := k8sCodeInsertEngine()
	if !e.Process(file) {
		t.Fatal("expected a transformation")
	}
	got := fileToString(t, file)

	if n := strings.Count(got, wrapStmt); n != 1 {
		t.Fatalf("expected exactly 1 insertion, got %d:\n%s", n, got)
	}
	wrapIdx := strings.Index(got, wrapStmt)
	ifIdx := strings.Index(got, "if client, err := kubernetes.NewForConfig(cfg)")
	if !(wrapIdx >= 0 && wrapIdx < ifIdx) {
		t.Errorf("insertion not before the if-init statement:\n%s", got)
	}
}

// TestCodeInsert_DeeplyNested — two levels of nesting (if inside for). Still a
// single insertion, in the innermost block.
func TestCodeInsert_DeeplyNested(t *testing.T) {
	src := `package p

func handler(items []int) {
	for range items {
		if true {
			cfg, err := rest.InClusterConfig()
			if err != nil {
				return
			}
			client, err := kubernetes.NewForConfig(cfg)
			_ = client
			_ = err
		}
	}
}
`
	file := parseTestFile(t, src)
	e := k8sCodeInsertEngine()
	if !e.Process(file) {
		t.Fatal("expected a transformation")
	}
	got := fileToString(t, file)

	if n := strings.Count(got, wrapStmt); n != 1 {
		t.Fatalf("expected exactly 1 insertion, got %d:\n%s", n, got)
	}
	declIdx := strings.Index(got, "cfg, err := rest.InClusterConfig()")
	wrapIdx := strings.Index(got, wrapStmt)
	callIdx := strings.Index(got, "kubernetes.NewForConfig(cfg)")
	if !(declIdx < wrapIdx && wrapIdx < callIdx) {
		t.Errorf("insertion not correctly scoped in innermost block:\n%s", got)
	}
}
