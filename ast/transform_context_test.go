package ast

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// parseTestFile parses Go source into a *dst.File.
func parseTestFile(t *testing.T, src string) *dst.File {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	df, err := decorator.DecorateFile(fset, f)
	if err != nil {
		t.Fatalf("decorate: %v", err)
	}
	return df
}

func findFirstCall(file *dst.File) *dst.CallExpr {
	var found *dst.CallExpr
	dst.Inspect(file, func(n dst.Node) bool {
		if found != nil {
			return false
		}
		if call, ok := n.(*dst.CallExpr); ok {
			found = call
			return false
		}
		return true
	})
	return found
}

func findFuncDecl(file *dst.File, name string) *dst.FuncDecl {
	for _, d := range file.Decls {
		if fn, ok := d.(*dst.FuncDecl); ok && fn.Name != nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// TestArgAt verifies the ArgAt template method handles in-range and out-of-range indices.
func TestArgAt(t *testing.T) {
	tc := TransformContext{ArgsList: []string{"a", "b", "c"}}
	if got := tc.ArgAt(0); got != "a" {
		t.Errorf("ArgAt(0) = %q, want %q", got, "a")
	}
	if got := tc.ArgAt(2); got != "c" {
		t.Errorf("ArgAt(2) = %q, want %q", got, "c")
	}
	if got := tc.ArgAt(3); got != "" {
		t.Errorf("ArgAt(3) = %q, want empty", got)
	}
	if got := tc.ArgAt(-1); got != "" {
		t.Errorf("ArgAt(-1) = %q, want empty", got)
	}
}

// TestBuildCallTransformContext_FiveArgs ensures 5+ argument calls populate ArgsList/ArgCount
// even though Arg0..Arg3 only cover the first four.
func TestBuildCallTransformContext_FiveArgs(t *testing.T) {
	src := `package p
func f() { callSomething(a, b, c, d, e) }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	if call == nil {
		t.Fatal("no call expr")
	}

	ctx := &MatchContext{File: file, Call: call, FuncName: "callSomething"}
	tc := buildCallTransformContext(ctx)

	if tc.ArgCount != 5 {
		t.Errorf("ArgCount = %d, want 5", tc.ArgCount)
	}
	if len(tc.ArgsList) != 5 {
		t.Errorf("len(ArgsList) = %d, want 5", len(tc.ArgsList))
	}
	want := []string{"a", "b", "c", "d", "e"}
	for i, w := range want {
		if tc.ArgAt(i) != w {
			t.Errorf("ArgAt(%d) = %q, want %q", i, tc.ArgAt(i), w)
		}
	}
	if tc.Arg3 != "d" {
		t.Errorf("Arg3 = %q, want d", tc.Arg3)
	}
}

// TestBuildCallTransformContext_NoArgs handles the zero-arg case.
func TestBuildCallTransformContext_NoArgs(t *testing.T) {
	src := `package p
func f() { foo() }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	ctx := &MatchContext{File: file, Call: call, FuncName: "foo"}
	tc := buildCallTransformContext(ctx)
	if tc.ArgCount != 0 {
		t.Errorf("ArgCount = %d, want 0", tc.ArgCount)
	}
	if tc.HasCtx {
		t.Error("HasCtx should be false for no-arg call")
	}
	if tc.ArgAt(0) != "" {
		t.Errorf("ArgAt(0) on empty = %q, want empty", tc.ArgAt(0))
	}
}

// TestBuildCallTransformContext_HasCtxFallback verifies the syntactic fallback:
// without go/types info, an arg named "ctx" is treated as context.
func TestBuildCallTransformContext_HasCtxFallback(t *testing.T) {
	src := `package p
func f() { doWork(ctx, key) }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	ctx := &MatchContext{File: file, Call: call, FuncName: "doWork"}
	tc := buildCallTransformContext(ctx)
	if !tc.HasCtx {
		t.Error("HasCtx should be true when arg is named ctx (syntactic fallback)")
	}
}

// TestBuildCallTransformContext_Variadic verifies a variadic call populates ArgsList correctly.
func TestBuildCallTransformContext_Variadic(t *testing.T) {
	src := `package p
func f() { grpc.NewServer(opts...) }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	ctx := &MatchContext{File: file, Call: call, FuncName: "NewServer"}
	tc := buildCallTransformContext(ctx)
	if tc.ArgCount != 1 {
		t.Errorf("ArgCount = %d, want 1", tc.ArgCount)
	}
	if tc.ArgAt(0) != "opts" {
		t.Errorf("ArgAt(0) = %q, want opts", tc.ArgAt(0))
	}
}

// TestBuildDeclTransformContext verifies parameter-name extraction for inject-site Advice.
func TestBuildDeclTransformContext(t *testing.T) {
	src := `package p
import "context"
func ProcessOrder(ctx context.Context, orderID string, qty int) error { return nil }
`
	file := parseTestFile(t, src)
	fn := findFuncDecl(file, "ProcessOrder")
	if fn == nil {
		t.Fatal("no FuncDecl")
	}
	ctx := &MatchContext{File: file, Decl: fn, FuncName: "ProcessOrder"}
	tc := buildDeclTransformContext(ctx)

	if tc.ArgCount != 3 {
		t.Errorf("ArgCount = %d, want 3", tc.ArgCount)
	}
	if tc.ArgAt(0) != "ctx" {
		t.Errorf("ArgAt(0) = %q, want ctx", tc.ArgAt(0))
	}
	if tc.ArgAt(1) != "orderID" {
		t.Errorf("ArgAt(1) = %q, want orderID", tc.ArgAt(1))
	}
	if !tc.HasCtx {
		t.Error("HasCtx should be true when first param is context.Context")
	}
}

// TestBuildDeclTransformContext_NoCtx verifies HasCtx=false when no context param.
func TestBuildDeclTransformContext_NoCtx(t *testing.T) {
	src := `package p
func Helper(n int) {}
`
	file := parseTestFile(t, src)
	fn := findFuncDecl(file, "Helper")
	ctx := &MatchContext{File: file, Decl: fn, FuncName: "Helper"}
	tc := buildDeclTransformContext(ctx)
	if tc.HasCtx {
		t.Error("HasCtx should be false for non-ctx func")
	}
	if tc.ArgCount != 1 {
		t.Errorf("ArgCount = %d, want 1", tc.ArgCount)
	}
}

// §236 — Args1Plus / IsSpread placeholders for variadic-aware Template substitution.
// Two source call shapes must both produce a compilable substitution string:
//   - spread:    f(policy, hosts...) → "hosts..."
//   - individual: f(policy, h1, h2, h3) → "h1, h2, h3"
func TestBuildCallTransformContext_Args1Plus_Spread(t *testing.T) {
	src := `package p
func f() { NewClientWithPolicyAndHost(policy, hosts...) }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	ctx := &MatchContext{File: file, Call: call, FuncName: "NewClientWithPolicyAndHost"}
	tc := buildCallTransformContext(ctx)

	if !tc.IsSpread {
		t.Error("IsSpread should be true for spread call")
	}
	if tc.Args1Plus != "hosts..." {
		t.Errorf("Args1Plus = %q, want %q", tc.Args1Plus, "hosts...")
	}
}

func TestBuildCallTransformContext_Args1Plus_Individual(t *testing.T) {
	src := `package p
func f() { NewClientWithPolicyAndHost(policy, h1, h2, h3) }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	ctx := &MatchContext{File: file, Call: call, FuncName: "NewClientWithPolicyAndHost"}
	tc := buildCallTransformContext(ctx)

	if tc.IsSpread {
		t.Error("IsSpread should be false for individual-arg call")
	}
	if tc.Args1Plus != "h1, h2, h3" {
		t.Errorf("Args1Plus = %q, want %q", tc.Args1Plus, "h1, h2, h3")
	}
}

func TestBuildCallTransformContext_Args1Plus_SingleExtra(t *testing.T) {
	src := `package p
func f() { g(a, b) }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	ctx := &MatchContext{File: file, Call: call, FuncName: "g"}
	tc := buildCallTransformContext(ctx)

	if tc.Args1Plus != "b" {
		t.Errorf("Args1Plus = %q, want %q", tc.Args1Plus, "b")
	}
	if tc.IsSpread {
		t.Error("IsSpread should be false")
	}
}

func TestBuildCallTransformContext_Args1Plus_OnlyArg0(t *testing.T) {
	src := `package p
func f() { g(only) }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	ctx := &MatchContext{File: file, Call: call, FuncName: "g"}
	tc := buildCallTransformContext(ctx)

	if tc.Args1Plus != "" {
		t.Errorf("Args1Plus = %q, want empty (only one arg)", tc.Args1Plus)
	}
}

// §236 — spread call where the slice variable holds multiple elements.
// The AST sees only the variable name (no go/types inspection of contents),
// so Args1Plus still reduces to the single arg + "..." regardless of slice length.
func TestBuildCallTransformContext_Args1Plus_SpreadMultiElement(t *testing.T) {
	src := `package p
func f() {
	hosts := []*Host{a, b, c, d}
	_ = hosts
	NewClientWithPolicyAndHost(policy, hosts...)
}
`
	file := parseTestFile(t, src)
	var call *dst.CallExpr
	dst.Inspect(file, func(n dst.Node) bool {
		if c, ok := n.(*dst.CallExpr); ok {
			if sel, ok := c.Fun.(*dst.Ident); ok && sel.Name == "NewClientWithPolicyAndHost" {
				call = c
				return false
			}
		}
		return true
	})
	if call == nil {
		t.Fatal("no NewClientWithPolicyAndHost call")
	}
	ctx := &MatchContext{File: file, Call: call, FuncName: "NewClientWithPolicyAndHost"}
	tc := buildCallTransformContext(ctx)

	if !tc.IsSpread {
		t.Error("IsSpread should be true for spread call")
	}
	if tc.Args1Plus != "hosts..." {
		t.Errorf("Args1Plus = %q, want %q (slice content is opaque to AST)", tc.Args1Plus, "hosts...")
	}
}

// §236 — individual args with 3+ hosts. Args1Plus must comma-join all
// non-Arg0 arguments without dropping any (covers >Arg3 limit).
func TestBuildCallTransformContext_Args1Plus_MultiIndividualArgs(t *testing.T) {
	src := `package p
func f() { NewClientWithPolicyAndHost(policy, h1, h2, h3, h4, h5) }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	ctx := &MatchContext{File: file, Call: call, FuncName: "NewClientWithPolicyAndHost"}
	tc := buildCallTransformContext(ctx)

	if tc.IsSpread {
		t.Error("IsSpread should be false for individual-arg call")
	}
	want := "h1, h2, h3, h4, h5"
	if tc.Args1Plus != want {
		t.Errorf("Args1Plus = %q, want %q (all non-Arg0 args, no truncation past Arg3)", tc.Args1Plus, want)
	}
	if tc.ArgCount != 6 {
		t.Errorf("ArgCount = %d, want 6", tc.ArgCount)
	}
}

// §236 — same multi-arg shape but with method receiver. Verifies the
// receiver path also threads Args1Plus correctly.
func TestBuildCallTransformContext_Args1Plus_MethodMultiArgs(t *testing.T) {
	src := `package p
func f() { client.Send(ctx, m1, m2, m3) }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	ctx := &MatchContext{File: file, Call: call, FuncName: "Send"}
	tc := buildCallTransformContext(ctx)

	if tc.Args1Plus != "m1, m2, m3" {
		t.Errorf("Args1Plus = %q, want %q", tc.Args1Plus, "m1, m2, m3")
	}
	if tc.IsSpread {
		t.Error("IsSpread should be false")
	}
}
