package ast

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dave/dst"
	"github.com/whatap/go-api-inst/ast/common"
)

// TestResolveFuncDeclTarget_WithPkgPath verifies decl:pkgpath.funcName format
// when currentImportPath is set.
func TestResolveFuncDeclTarget_WithPkgPath(t *testing.T) {
	common.SetCurrentImportPath("myapp/service")
	defer common.SetCurrentImportPath("")

	fn := &dst.FuncDecl{
		Name: dst.NewIdent("ProcessOrder"),
		Type: &dst.FuncType{},
	}
	got := resolveFuncDeclTarget(fn)
	want := "decl:myapp/service.ProcessOrder"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveFuncDeclTarget_FallbackNoPkgPath verifies the legacy format when
// currentImportPath is empty (go/types unavailable or not set).
func TestResolveFuncDeclTarget_FallbackNoPkgPath(t *testing.T) {
	common.SetCurrentImportPath("")
	fn := &dst.FuncDecl{
		Name: dst.NewIdent("main"),
		Type: &dst.FuncType{},
	}
	got := resolveFuncDeclTarget(fn)
	if got != "decl:main" {
		t.Errorf("got %q, want %q", got, "decl:main")
	}
}

// TestRegistryLookup_DeclWildcardMatchAll verifies "decl:*" matches any decl target.
func TestRegistryLookup_DeclWildcardMatchAll(t *testing.T) {
	r := NewRegistry()
	rule := &Rule{Target: "decl:*", Advice: &OnMatchFunc{}}
	r.Register(rule)

	if r.Lookup("decl:myapp/service.Foo") != rule {
		t.Error("decl:* should match decl:myapp/service.Foo")
	}
	if r.Lookup("decl:main") != rule {
		t.Error("decl:* should match decl:main")
	}
}

// TestRegistryLookup_DeclWildcardPkgPrefix verifies "decl:myapp/service.*"
// matches functions in that package but not others.
func TestRegistryLookup_DeclWildcardPkgPrefix(t *testing.T) {
	r := NewRegistry()
	rule := &Rule{Target: "decl:myapp/service.*", Advice: &OnMatchFunc{}}
	r.Register(rule)

	if r.Lookup("decl:myapp/service.ProcessOrder") != rule {
		t.Error("should match ProcessOrder")
	}
	if r.Lookup("decl:myapp/service.Cancel") != rule {
		t.Error("should match Cancel")
	}
	if r.Lookup("decl:myapp/other.Foo") != nil {
		t.Error("should NOT match other package")
	}
	if r.Lookup("decl:main") != nil {
		t.Error("should NOT match unrelated decl")
	}
}

// TestRegistryLookup_ExactBeatsWildcard verifies exact matches take priority.
func TestRegistryLookup_ExactBeatsWildcard(t *testing.T) {
	r := NewRegistry()
	wild := &Rule{Target: "decl:myapp/service.*", Advice: &OnMatchFunc{WhatapPkg: "wild"}}
	exact := &Rule{Target: "decl:myapp/service.Foo", Advice: &OnMatchFunc{WhatapPkg: "exact"}}
	r.Register(wild)
	r.Register(exact)

	got := r.Lookup("decl:myapp/service.Foo")
	if got != exact {
		t.Errorf("exact rule should win over wildcard; got target=%q", got.Target)
	}
}

// TestMatchDeclWildcard_PrefixInFunc verifies prefix matching within function name.
func TestMatchDeclWildcard_PrefixInFunc(t *testing.T) {
	if !matchDeclWildcard("decl:myapp/svc.Process*", "decl:myapp/svc.ProcessOrder") {
		t.Error("Process* should match ProcessOrder")
	}
	if matchDeclWildcard("decl:myapp/svc.Process*", "decl:myapp/svc.Cancel") {
		t.Error("Process* should NOT match Cancel")
	}
}

// TestMatchDeclWildcard_MiddleAndSuffix verifies single middle wildcard
// (e.g. Get*DB) and pure suffix wildcard (e.g. *Handler) — §227 Step 5.
func TestMatchDeclWildcard_MiddleAndSuffix(t *testing.T) {
	cases := []struct {
		pattern, target string
		want            bool
	}{
		// middle wildcard
		{"decl:app.Get*DB", "decl:app.GetUserDB", true},
		{"decl:app.Get*DB", "decl:app.GetOrderDB", true},
		{"decl:app.Get*DB", "decl:app.GetLogDB", true},
		{"decl:app.Get*DB", "decl:app.GetUser", false},
		{"decl:app.Get*DB", "decl:app.UserDB", false},

		// suffix-only wildcard
		{"decl:app.*Handler", "decl:app.RequestHandler", true},
		{"decl:app.*Handler", "decl:app.HTTPHandler", true},
		{"decl:app.*Handler", "decl:app.HandleRequest", false},

		// prefix wildcard still works
		{"decl:app.Process*", "decl:app.ProcessOrder", true},
		{"decl:app.Process*", "decl:app.Cancel", false},

		// multiple wildcards rejected
		{"decl:app.A*B*C", "decl:app.AxByCz", false},
	}
	for _, tc := range cases {
		got := matchDeclWildcard(tc.pattern, tc.target)
		if got != tc.want {
			t.Errorf("matchDeclWildcard(%q, %q) = %v, want %v",
				tc.pattern, tc.target, got, tc.want)
		}
	}
}

// TestResolveCallTarget_LocalFunctionCall verifies that bare-identifier calls
// to a local function resolve to "<importpath>.<funcname>" (§227 Step 5).
//
// Builds a tiny Go module on disk, runs TrySetupTypeContext, walks the dst
// tree, finds the bare-ident call, and asserts the resolved target matches
// the expected importpath form.
func TestResolveCallTarget_LocalFunctionCall(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module localcalltest\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	src := `package main

func fetchData() string {
	return "hello"
}

func main() {
	_ = fetchData()
}
`
	mainPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	tc := common.NewTypeChecker()
	file := common.TrySetupTypeContext(tc, mainPath)
	if file == nil {
		t.Skip("TrySetupTypeContext returned nil — packages.Load may have failed in this env")
	}
	defer common.ClearTypeContext()

	if !common.HasTypeInfo() {
		t.Fatal("expected HasTypeInfo() == true after TrySetupTypeContext")
	}

	var found string
	dst.Inspect(file, func(n dst.Node) bool {
		call, ok := n.(*dst.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*dst.Ident)
		if !ok || ident.Name != "fetchData" {
			return true
		}
		found = resolveCallTarget(call)
		return false
	})

	want := "localcalltest.fetchData"
	if found != want {
		t.Errorf("resolveCallTarget for fetchData() = %q, want %q", found, want)
	}
}

// TestResolveMethodTarget_GenericReceiver verifies that a method call on an
// instantiated generic receiver resolves to a target WITHOUT the type arguments
// (§282). Before the fix, resolveMethodTarget used types.Type.String() which
// embeds the type args (e.g. "...Chain[string, int].AppendChatModel"), so no
// static Rule target could ever match a generic compose receiver. A non-generic
// receiver in the same file guards against regression.
func TestResolveMethodTarget_GenericReceiver(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gentgt\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	src := `package main

// Generic type with a chainable method (mirrors eino compose.Chain[I, O]).
type Chain[I any, O any] struct{}

func (c *Chain[I, O]) AppendChatModel(m any) *Chain[I, O] { return c }

func NewChain[I any, O any]() *Chain[I, O] { return &Chain[I, O]{} }

// Non-generic type — regression guard (target must be unchanged).
type Box struct{}

func (b *Box) Open() {}

func main() {
	ch := NewChain[string, int]()
	ch.AppendChatModel(nil)

	box := &Box{}
	box.Open()
}
`
	mainPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	tc := common.NewTypeChecker()
	file := common.TrySetupTypeContext(tc, mainPath)
	if file == nil {
		t.Skip("TrySetupTypeContext returned nil — packages.Load may have failed in this env")
	}
	defer common.ClearTypeContext()

	if !common.HasTypeInfo() {
		t.Fatal("expected HasTypeInfo() == true after TrySetupTypeContext")
	}

	got := map[string]string{} // method name → resolved target
	dst.Inspect(file, func(n dst.Node) bool {
		call, ok := n.(*dst.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*dst.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "AppendChatModel" || sel.Sel.Name == "Open" {
			got[sel.Sel.Name] = resolveCallTarget(call)
		}
		return true
	})

	// Generic receiver: type arguments must be dropped.
	if want := "gentgt.Chain.AppendChatModel"; got["AppendChatModel"] != want {
		t.Errorf("generic receiver target = %q, want %q (type args must be stripped)", got["AppendChatModel"], want)
	}
	// Non-generic receiver: unchanged behavior.
	if want := "gentgt.Box.Open"; got["Open"] != want {
		t.Errorf("non-generic receiver target = %q, want %q (regression)", got["Open"], want)
	}
}
