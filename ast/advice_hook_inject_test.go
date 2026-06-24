package ast

import (
	"strings"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// fileToString re-renders a dst.File for assertions.
func fileToString(t *testing.T, file *dst.File) string {
	t.Helper()
	var b strings.Builder
	if err := decorator.Fprint(&b, file); err != nil {
		t.Fatalf("fprint: %v", err)
	}
	return b.String()
}

// TestHookAdvice_BeforeAfter verifies Before/After statements are inserted
// around the matched call statement.
func TestHookAdvice_BeforeAfter(t *testing.T) {
	src := `package p

func main() {
	doWork()
}
`
	file := parseTestFile(t, src)
	fn := findFuncDecl(file, "main")
	if fn == nil || fn.Body == nil {
		t.Fatal("no main body")
	}
	stmt := fn.Body.List[0]
	call := findFirstCall(file)

	ctx := &MatchContext{
		File:          file,
		Mode:          ModeInject,
		Call:          call,
		FuncName:      "doWork",
		EnclosingFunc: fn,
		EnclosingStmt: stmt,
		ParentBlock:   &fn.Body.List,
		StmtIndex:     0,
	}

	adv := &Hook{
		Before: `println("before")`,
		After:  `println("after")`,
	}
	adv.Apply(ctx)

	if !ctx.Applied {
		t.Fatal("Hook should have applied")
	}
	got := fileToString(t, file)
	if !strings.Contains(got, `println("before")`) {
		t.Errorf("missing before code:\n%s", got)
	}
	if !strings.Contains(got, `println("after")`) {
		t.Errorf("missing after code:\n%s", got)
	}
	// Before must precede the original call
	bIdx := strings.Index(got, `println("before")`)
	cIdx := strings.Index(got, `doWork()`)
	aIdx := strings.Index(got, `println("after")`)
	if !(bIdx < cIdx && cIdx < aIdx) {
		t.Errorf("statement order wrong:\n%s", got)
	}
}

// TestHookAdvice_Template verifies template variables resolve inside Before.
func TestHookAdvice_Template(t *testing.T) {
	src := `package p
func main() { doWork(ctx, "a") }
`
	file := parseTestFile(t, src)
	fn := findFuncDecl(file, "main")
	stmt := fn.Body.List[0]
	call := findFirstCall(file)

	ctx := &MatchContext{
		File: file, Mode: ModeInject, Call: call, FuncName: "doWork",
		EnclosingFunc: fn, EnclosingStmt: stmt,
		ParentBlock: &fn.Body.List, StmtIndex: 0,
	}
	adv := &Hook{
		Before: `println("{{.FuncName}} hasCtx={{.HasCtx}}")`,
	}
	adv.Apply(ctx)
	if !ctx.Applied {
		t.Fatal("apply failed")
	}
	got := fileToString(t, file)
	if !strings.Contains(got, `"doWork hasCtx=true"`) {
		t.Errorf("template not rendered:\n%s", got)
	}
}

// TestInjectAdvice_StartEnd verifies start code is inserted at function start
// and end code is wrapped in a defer closure.
func TestInjectAdvice_StartEnd(t *testing.T) {
	src := `package p
func ProcessOrder() error {
	return nil
}
`
	file := parseTestFile(t, src)
	fn := findFuncDecl(file, "ProcessOrder")

	ctx := &MatchContext{
		File:          file,
		Mode:          ModeInject,
		Decl:          fn,
		FuncName:      "ProcessOrder",
		EnclosingFunc: fn,
	}
	adv := &Inject{
		Start: `println("enter {{.FuncName}}")`,
		End:   `println("exit")`,
	}
	adv.Apply(ctx)

	if !ctx.Applied {
		t.Fatal("Inject should have applied")
	}
	got := fileToString(t, file)
	if !strings.Contains(got, `println("enter ProcessOrder")`) {
		t.Errorf("missing start code:\n%s", got)
	}
	if !strings.Contains(got, `defer func()`) {
		t.Errorf("end code should be wrapped in defer:\n%s", got)
	}
	if !strings.Contains(got, `println("exit")`) {
		t.Errorf("missing end code:\n%s", got)
	}
}

// TestInjectAdvice_HasCtxConditional verifies that {{.HasCtx}} works for inject.
func TestInjectAdvice_HasCtxConditional(t *testing.T) {
	src := `package p
import "context"
func Handle(ctx context.Context, id int) error { return nil }
`
	file := parseTestFile(t, src)
	fn := findFuncDecl(file, "Handle")

	ctx := &MatchContext{
		File: file, Mode: ModeInject, Decl: fn, FuncName: "Handle",
		EnclosingFunc: fn,
	}
	adv := &Inject{
		Start: `{{if .HasCtx}}println("has context"){{end}}`,
	}
	adv.Apply(ctx)

	if !ctx.Applied {
		t.Fatal("apply failed")
	}
	got := fileToString(t, file)
	if !strings.Contains(got, `println("has context")`) {
		t.Errorf("HasCtx branch should have fired:\n%s", got)
	}
}

// TestHookAdvice_RemoveIsNoop was removed by §272 Phase 3 Step 3 (2026-05-19).
// Apply is no longer dispatched from ModeRemove paths, so a "Remove is no-op"
// guard is dead code. remove now cleans up manually written code via a
// separate engine (removeManualPatterns); see issue 272.
