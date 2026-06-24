package ast

import (
	"bytes"
	"path"
	"strings"
	"text/template"

	"github.com/dave/dst"
	"github.com/whatap/go-api-inst/ast/common"
)

// Advice defines the transformation strategy for a matched target.
type Advice interface {
	// Apply executes the transformation.
	// ctx.Mode determines inject vs remove behavior.
	Apply(ctx *MatchContext)

	// WhatapImportPath returns the whatap import path to add (empty if none).
	WhatapImportPath() string

	// WhatapImportAlias returns the import alias (empty for default).
	WhatapImportAlias() string
}

// ReplaceFunction swaps ident.Name in-place (same function signature).
// Example: sql.Open -> whatapsql.Open
type ReplaceFunction struct {
	WhatapPkg   string // full import path, e.g. "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"
	WhatapAlias string // alias to use in code, e.g. "whatapsql"
	WhatapFunc  string // replacement function name, e.g. "Open"
}

func (a *ReplaceFunction) Apply(ctx *MatchContext) {
	if ctx.Ident == nil {
		ctx.Applied = false
		return
	}
	// §272 Phase 3 Step 3 — ModeRemove else branch removed (engine no longer
	// invokes Apply in remove mode).
	ctx.Ident.Name = a.WhatapAlias
	if ctx.Sel != nil && a.WhatapFunc != "" {
		ctx.Sel.Sel.Name = a.WhatapFunc
	}
}

func (a *ReplaceFunction) WhatapImportPath() string  { return a.WhatapPkg }
func (a *ReplaceFunction) WhatapImportAlias() string { return a.WhatapAlias }

// WrapCall wraps a call expression.
// Example: gin.Default() -> whatapgin.WrapEngine(gin.Default())
type WrapCall struct {
	WhatapPkg   string // full import path
	WhatapAlias string // alias in code, e.g. "whatapgin"
	WhatapFunc  string // wrapper function name, e.g. "WrapEngine"
}

func (a *WrapCall) Apply(ctx *MatchContext) {
	if ctx.Call == nil {
		ctx.Applied = false
		return
	}
	// §272 Phase 3 Step 3 — ModeRemove else branch removed.
	common.WrapCallExpr(ctx.Call, a.WhatapAlias, a.WhatapFunc)
}

func (a *WrapCall) WhatapImportPath() string  { return a.WhatapPkg }
func (a *WrapCall) WhatapImportAlias() string { return a.WhatapAlias }

// OnMatchFunc is a callback for complex patterns that don't fit declarative Advice.
// LEGACY: Phase 2~3 adds declarative Advice types to replace OnMatchFunc usage.
type OnMatchFunc struct {
	WhatapPkg string
	Handler   func(ctx *MatchContext)
}

func (a *OnMatchFunc) Apply(ctx *MatchContext) {
	if a.Handler != nil {
		a.Handler(ctx)
	}
}

func (a *OnMatchFunc) WhatapImportPath() string  { return a.WhatapPkg }
func (a *OnMatchFunc) WhatapImportAlias() string { return "" }

// ── Phase 2 declarative Advice types ──────────────────────────────────

// InsertedArg describes an expression: WrapPkg.WrapFunc(WhatapAlias.InnerFunc()).
// Used by ArgInsert to build interceptor-style arguments.
type InsertedArg struct {
	WrapFunc  string // outer function on original package, e.g. "UnaryInterceptor"
	InnerFunc string // inner function on whatap package, e.g. "UnaryServerInterceptor"
}

// ArgInsert adds new arguments to a function call.
// Handles ellipsis (variadic spread) by wrapping with append().
//
// Example (grpc):
//
//	grpc.NewServer(opts...) →
//	grpc.NewServer(append(opts, grpc.UnaryInterceptor(whatapgrpc.UnaryServerInterceptor()), ...)...)
type ArgInsert struct {
	WhatapPkg   string       // whatap import path
	WhatapAlias string       // whatap alias in code
	InsertArgs  []InsertedArg // arguments to insert
	Ellipsis    bool         // true: handle variadic spread with append()
}

func (a *ArgInsert) Apply(ctx *MatchContext) {
	if ctx.Call == nil {
		ctx.Applied = false
		return
	}
	// §272 Phase 3 Step 3 — ModeRemove else branch removed.
	newArgs := a.buildArgs(ctx.PkgName)
	if a.Ellipsis && ctx.Call.Ellipsis && len(ctx.Call.Args) > 0 {
		// Has spread: wrap last arg with append()
		lastArg := ctx.Call.Args[len(ctx.Call.Args)-1]
		appendCall := &dst.CallExpr{
			Fun:  dst.NewIdent("append"),
			Args: append([]dst.Expr{lastArg}, newArgs...),
		}
		ctx.Call.Args[len(ctx.Call.Args)-1] = appendCall
	} else {
		ctx.Call.Args = append(ctx.Call.Args, newArgs...)
	}
}

func (a *ArgInsert) buildArgs(origPkgName string) []dst.Expr {
	var args []dst.Expr
	for _, ia := range a.InsertArgs {
		// Build: origPkg.WrapFunc(whatapAlias.InnerFunc())
		arg := &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(origPkgName),
				Sel: dst.NewIdent(ia.WrapFunc),
			},
			Args: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent(a.WhatapAlias),
						Sel: dst.NewIdent(ia.InnerFunc),
					},
				},
			},
		}
		args = append(args, arg)
	}
	return args
}

// §272 Phase 3 Step 3 — removed filterArgs / isInsertedArg (ModeRemove-only).

func (a *ArgInsert) WhatapImportPath() string  { return a.WhatapPkg }
func (a *ArgInsert) WhatapImportAlias() string { return a.WhatapAlias }

// ArgWrap wraps an existing argument with a whatap function call.
//
// Example (log.New):
//
//	log.New(writer, prefix, flag) → log.New(whataplogsink.GetTraceLogWriter(writer), prefix, flag)
type ArgWrap struct {
	WhatapPkg   string // whatap import path
	WhatapAlias string // whatap alias in code
	WhatapFunc  string // wrapper function name, e.g. "GetTraceLogWriter"
	ArgIndex    int    // which arg to wrap (0-based, -1 = last)
}

func (a *ArgWrap) Apply(ctx *MatchContext) {
	if ctx.Call == nil || len(ctx.Call.Args) == 0 {
		ctx.Applied = false
		return
	}
	idx := a.ArgIndex
	if idx < 0 {
		idx = len(ctx.Call.Args) + idx
	}
	if idx < 0 || idx >= len(ctx.Call.Args) {
		ctx.Applied = false
		return
	}
	// §272 Phase 3 Step 3 — ModeRemove else branch removed.
	// Check not already wrapped
	if isWrappedBy(ctx.Call.Args[idx], a.WhatapAlias, a.WhatapFunc) {
		ctx.Applied = false
		return
	}
	ctx.Call.Args[idx] = &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   dst.NewIdent(a.WhatapAlias),
			Sel: dst.NewIdent(a.WhatapFunc),
		},
		Args: []dst.Expr{ctx.Call.Args[idx]},
	}
}

func (a *ArgWrap) WhatapImportPath() string  { return a.WhatapPkg }
func (a *ArgWrap) WhatapImportAlias() string { return a.WhatapAlias }

// isWrappedBy checks if expr is alias.funcName(...) call.
func isWrappedBy(expr dst.Expr, alias, funcName string) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}
	return ident.Name == alias && sel.Sel.Name == funcName
}

// CodeInsert inserts a statement before or after the matched call.
// All fields are declarative settings — no callbacks.
//
// Pattern: {call.Args[ArgSource]}.MethodName(whatapAlias.WhatapFunc())
//
// Example (k8s):
//
//	[inserted] config.Wrap(whatapkubernetes.WrapRoundTripper())
//	clientset, err := kubernetes.NewForConfig(config)
type CodeInsert struct {
	WhatapPkg   string // whatap import path
	WhatapAlias string // whatap alias in code
	Position    string // "before" or "after"
	ArgSource   int    // which call arg to extract variable name from (0-based)
	MethodName  string // method to call on the extracted variable, e.g. "Wrap"
	WhatapFunc  string // whatap function to call as argument, e.g. "WrapRoundTripper"
}

func (a *CodeInsert) Apply(ctx *MatchContext) {
	if ctx.ParentBlock == nil || ctx.StmtIndex < 0 {
		ctx.Applied = false
		return
	}
	// §272 Phase 3 Step 3 — ModeRemove else branch removed.
	// Extract variable name from call argument
	if ctx.Call == nil || a.ArgSource >= len(ctx.Call.Args) {
		ctx.Applied = false
		return
	}
	argIdent, ok := ctx.Call.Args[a.ArgSource].(*dst.Ident)
	if !ok {
		ctx.Applied = false
		return
	}
	varName := argIdent.Name

	// §287 — Engine 1차 패스(processStmt + dst.Inspect)는 중첩 블록(IfStmt/ForStmt body 등)
	// 안의 호출도 바깥 블록 컨텍스트(EnclosingStmt=바깥 stmt, ParentBlock=바깥 블록)로 1차
	// 매칭한다. CodeInsert 는 호출 노드를 변형하지 않고 형제 statement 만 삽입하므로, 이 1차
	// 매칭을 그대로 처리하면 호출을 감싼 바깥 statement 앞에 코드가 삽입되어 인자(varName)가
	// 스코프 밖이 된다 (teleport: cfg 가 중첩 if 블록 안에서 선언 → undefined: cfg). 중첩 블록
	// 안 호출은 processNestedBlocks 2차 패스(올바른 안쪽 블록 컨텍스트)에서 정확히 1회 삽입되므로,
	// 매칭된 호출이 EnclosingStmt 안에 중첩 블록을 건너뛰지 않고 직접 존재할 때만 삽입한다.
	if ctx.EnclosingStmt != nil && !stmtOwnsCallDirectly(ctx.EnclosingStmt, ctx.Call) {
		ctx.Applied = false
		return
	}

	// Build: varName.MethodName(whatapAlias.WhatapFunc())
	stmt := &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(varName),
				Sel: dst.NewIdent(a.MethodName),
			},
			Args: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent(a.WhatapAlias),
						Sel: dst.NewIdent(a.WhatapFunc),
					},
				},
			},
		},
	}
	stmt.Decs.After = dst.NewLine

	block := *ctx.ParentBlock
	if a.Position == "before" {
		newBlock := make([]dst.Stmt, 0, len(block)+1)
		newBlock = append(newBlock, block[:ctx.StmtIndex]...)
		newBlock = append(newBlock, stmt)
		newBlock = append(newBlock, block[ctx.StmtIndex:]...)
		*ctx.ParentBlock = newBlock
	} else {
		newBlock := make([]dst.Stmt, 0, len(block)+1)
		newBlock = append(newBlock, block[:ctx.StmtIndex+1]...)
		newBlock = append(newBlock, stmt)
		newBlock = append(newBlock, block[ctx.StmtIndex+1:]...)
		*ctx.ParentBlock = newBlock
	}
}

// §272 Phase 3 Step 3 — removed CodeInsert.isInsertedStmt (ModeRemove-only).

func (a *CodeInsert) WhatapImportPath() string  { return a.WhatapPkg }
func (a *CodeInsert) WhatapImportAlias() string { return a.WhatapAlias }

// stmtOwnsCallDirectly reports whether call appears inside stmt without crossing
// into a nested statement block (IfStmt/ForStmt/RangeStmt body, etc.). It is the
// guard for sibling-statement-inserting Advice (CodeInsert, Hook): the engine's
// 1st traversal pass (processStmt → dst.Inspect over the whole subtree) matches
// nested-block calls under the *outer* block context, which would insert sibling
// code at the wrong scope. Such calls are re-matched in the correct inner-block
// context by the processNestedBlocks 2nd pass, so the outer 1st-pass match must
// be skipped. Returns true only when the call is a direct descendant of stmt
// (call expressions in an IfStmt.Init/Cond count as direct — those are not
// statement blocks and the inner-block pass never revisits them). §287.
func stmtOwnsCallDirectly(stmt dst.Stmt, call *dst.CallExpr) bool {
	if stmt == nil || call == nil {
		return false
	}
	found := false
	root := true
	dst.Inspect(stmt, func(n dst.Node) bool {
		if found || n == nil {
			return false
		}
		if n == call {
			found = true
			return false
		}
		// Do not descend into nested statement blocks — a call there belongs to
		// the inner block, handled by the processNestedBlocks pass. The root stmt
		// itself (when it is a BlockStmt) must still be entered.
		if _, ok := n.(*dst.BlockStmt); ok && !root {
			return false
		}
		root = false
		return true
	})
	return found
}

// MainInsert inserts a statement in main() after defer trace.Shutdown().
// Used for zap (HookStderr) and log (SetOutput) — Engine 내부에서 직접 처리.
//
// Example (zap):
//
//	func main() {
//	    trace.Init(nil)
//	    defer trace.Shutdown()
//	    whataplogsink.HookStderr()  ← 삽입
//	    ...
//	    zap.NewProduction()         ← 매칭 지점
//	}
type MainInsert struct {
	WhatapPkg   string // whatap import path
	WhatapAlias string // whatap alias in code
	WhatapFunc  string // function to call, e.g. "HookStderr", "SetOutput"
	// ExtraImport is an additional import needed (e.g., "os" for log.SetOutput)
	ExtraImport string
	// WrapExpr: if non-empty, wraps the call: WhatapAlias.WhatapFunc(WrapExpr)
	// e.g., "os.Stderr" → whataplogsink.GetTraceLogWriter(os.Stderr)
	// If empty: WhatapAlias.WhatapFunc() — no args
	WrapExpr string
	// OrigPkgAlias: if non-empty, call is OrigPkgAlias.OrigFunc(WhatapAlias.WhatapFunc(...))
	// e.g., log.SetOutput(whataplogsink.GetTraceLogWriter(os.Stderr))
	OrigPkgAlias string
	OrigFunc     string

	inserted bool // prevent duplicate insertion
}

func (a *MainInsert) Apply(ctx *MatchContext) {
	if a.inserted {
		ctx.Applied = false
		return // already inserted in this file
	}
	// Must be inside main()
	if ctx.EnclosingFunc == nil || ctx.EnclosingFunc.Name.Name != "main" {
		ctx.Applied = false
		return
	}

	// §272 Phase 3 Step 3 — ModeRemove else branch removed.
	shutdownIdx := common.FindDeferShutdownIndex(ctx.EnclosingFunc)
	if shutdownIdx < 0 {
		ctx.Applied = false
		return
	}

	stmt := a.buildStmt(ctx)
	common.InsertStmtAfterIndex(ctx.EnclosingFunc, shutdownIdx, stmt)

	// Add extra import if needed (e.g., "os")
	if a.ExtraImport != "" {
		common.AddImport(ctx.File, `"`+a.ExtraImport+`"`)
	}

	a.inserted = true
}

func (a *MainInsert) buildStmt(ctx *MatchContext) *dst.ExprStmt {
	// Build the whatap call expression
	var whatapCall dst.Expr
	if a.WrapExpr != "" {
		// WhatapAlias.WhatapFunc(wrapExpr)
		// Parse WrapExpr as pkg.field (e.g., "os.Stderr")
		parts := splitDot(a.WrapExpr)
		var innerExpr dst.Expr
		if len(parts) == 2 {
			innerExpr = &dst.SelectorExpr{
				X:   dst.NewIdent(parts[0]),
				Sel: dst.NewIdent(parts[1]),
			}
		} else {
			innerExpr = dst.NewIdent(a.WrapExpr)
		}
		whatapCall = &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(a.WhatapAlias),
				Sel: dst.NewIdent(a.WhatapFunc),
			},
			Args: []dst.Expr{innerExpr},
		}
	} else {
		// WhatapAlias.WhatapFunc()
		whatapCall = &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(a.WhatapAlias),
				Sel: dst.NewIdent(a.WhatapFunc),
			},
		}
	}

	var finalExpr dst.Expr
	if a.OrigPkgAlias != "" {
		// OrigPkgAlias.OrigFunc(whatapCall)
		finalExpr = &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(a.OrigPkgAlias),
				Sel: dst.NewIdent(a.OrigFunc),
			},
			Args: []dst.Expr{whatapCall},
		}
	} else {
		finalExpr = whatapCall
	}

	stmt := &dst.ExprStmt{X: finalExpr}
	stmt.Decs.After = dst.NewLine
	return stmt
}

// §272 Phase 3 Step 3 — removed MainInsert.isInsertedStmt (ModeRemove-only).

func (a *MainInsert) WhatapImportPath() string  { return a.WhatapPkg }
func (a *MainInsert) WhatapImportAlias() string { return a.WhatapAlias }

// splitDot splits "a.b" into ["a", "b"]. Returns original as single element if no dot.
func splitDot(s string) []string {
	for i, c := range s {
		if c == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// ── Phase 3 declarative Advice types ──────────────────────────────────

// FieldWrap wraps an existing struct field value with a whatap function call.
// Used for composite literals like http.Server{Handler: h} or http.Client{Transport: t}.
//
// Example (http.Server):
//
//	http.Server{Handler: mux} → http.Server{Handler: whataphttp.WrapHandler(mux)}
//
// Example (http.Client with CtxAware):
//
//	http.Client{Transport: t} → http.Client{Transport: whataphttp.NewRoundTrip(ctx, t)}
type FieldWrap struct {
	WhatapPkg   string // whatap import path
	WhatapAlias string // alias in code
	WhatapFunc  string // wrapper function name, e.g. "WrapHandler", "NewRoundTrip"
	FieldName   string // struct field to wrap, e.g. "Handler", "Transport"
	CtxAware    bool   // true: WhatapFunc(ctx, fieldValue), false: WhatapFunc(fieldValue)
}

func (a *FieldWrap) Apply(ctx *MatchContext) {
	if ctx.Lit == nil {
		ctx.Applied = false
		return
	}
	for _, elt := range ctx.Lit.Elts {
		kv, ok := elt.(*dst.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*dst.Ident)
		if !ok || keyIdent.Name != a.FieldName {
			continue
		}

		// §272 Phase 3 Step 3 — ModeRemove else branch removed.
		// Skip nil values
		if valIdent, ok := kv.Value.(*dst.Ident); ok && valIdent.Name == "nil" {
			ctx.Applied = false
			return
		}
		// Skip if already wrapped
		if isWrappedBy(kv.Value, a.WhatapAlias, a.WhatapFunc) {
			ctx.Applied = false
			return
		}
		if a.CtxAware {
			// WhatapFunc(ctx, fieldValue)
			ctxExpr := detectCtxExpr(ctx)
			kv.Value = &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent(a.WhatapAlias),
					Sel: dst.NewIdent(a.WhatapFunc),
				},
				Args: []dst.Expr{ctxExpr, kv.Value},
			}
		} else {
			// WhatapFunc(fieldValue)
			kv.Value = &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent(a.WhatapAlias),
					Sel: dst.NewIdent(a.WhatapFunc),
				},
				Args: []dst.Expr{kv.Value},
			}
		}
		return
	}
}

func (a *FieldWrap) WhatapImportPath() string  { return a.WhatapPkg }
func (a *FieldWrap) WhatapImportAlias() string { return a.WhatapAlias }

// FieldInsert adds a new struct field when it doesn't exist.
// Used for composite literals like http.Client{} (no Transport field).
//
// Example:
//
//	http.Client{} → http.Client{Transport: whataphttp.NewRoundTripWithEmptyTransport(ctx)}
type FieldInsert struct {
	WhatapPkg   string // whatap import path
	WhatapAlias string // alias in code
	WhatapFunc  string // function to call, e.g. "NewRoundTripWithEmptyTransport"
	FieldName   string // field to insert, e.g. "Transport"
	CtxAware    bool   // true: WhatapFunc(ctx), false: WhatapFunc()
}

func (a *FieldInsert) Apply(ctx *MatchContext) {
	if ctx.Lit == nil {
		ctx.Applied = false
		return
	}
	// §272 Phase 3 Step 3 — ModeRemove else branch removed.
	// Build value expression
	var valueExpr dst.Expr
	if a.CtxAware {
		ctxExpr := detectCtxExpr(ctx)
		valueExpr = &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(a.WhatapAlias),
				Sel: dst.NewIdent(a.WhatapFunc),
			},
			Args: []dst.Expr{ctxExpr},
		}
	} else {
		valueExpr = &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(a.WhatapAlias),
				Sel: dst.NewIdent(a.WhatapFunc),
			},
		}
	}
	newKV := &dst.KeyValueExpr{
		Key:   dst.NewIdent(a.FieldName),
		Value: valueExpr,
	}
	ctx.Lit.Elts = append(ctx.Lit.Elts, newKV)
}

func (a *FieldInsert) WhatapImportPath() string  { return a.WhatapPkg }
func (a *FieldInsert) WhatapImportAlias() string { return a.WhatapAlias }

// FieldWrapOrInsert wraps an existing field or inserts a new one if missing.
// Combines FieldWrap + FieldInsert for cases where the same Target needs both behaviors.
//
// Example (http.Client with Transport):
//
//	http.Client{Transport: t} → http.Client{Transport: whataphttp.NewRoundTrip(ctx, t)}
//
// Example (http.Client without Transport):
//
//	http.Client{} → http.Client{Transport: whataphttp.NewRoundTripWithEmptyTransport(ctx)}
type FieldWrapOrInsert struct {
	WhatapPkg   string // whatap import path
	WhatapAlias string // alias in code
	WrapFunc    string // function when field exists, e.g. "NewRoundTrip"
	InsertFunc  string // function when field missing, e.g. "NewRoundTripWithEmptyTransport"
	FieldName   string // struct field name, e.g. "Transport"
	CtxAware    bool   // true: functions take ctx as first arg
}

func (a *FieldWrapOrInsert) Apply(ctx *MatchContext) {
	if ctx.Lit == nil {
		ctx.Applied = false
		return
	}
	// §272 Phase 3 Step 3 — ModeRemove else branch removed.
	// Find existing field
	for _, elt := range ctx.Lit.Elts {
		kv, ok := elt.(*dst.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*dst.Ident)
		if !ok || keyIdent.Name != a.FieldName {
			continue
		}
		// Field exists — wrap it
		if isWrappedBy(kv.Value, a.WhatapAlias, a.WrapFunc) {
			ctx.Applied = false
			return
		}
		if a.CtxAware {
			ctxExpr := detectCtxExpr(ctx)
			kv.Value = &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent(a.WhatapAlias),
					Sel: dst.NewIdent(a.WrapFunc),
				},
				Args: []dst.Expr{ctxExpr, kv.Value},
			}
		} else {
			kv.Value = &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent(a.WhatapAlias),
					Sel: dst.NewIdent(a.WrapFunc),
				},
				Args: []dst.Expr{kv.Value},
			}
		}
		return
	}
	// Field missing — insert
	var valueExpr dst.Expr
	if a.CtxAware {
		ctxExpr := detectCtxExpr(ctx)
		valueExpr = &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(a.WhatapAlias),
				Sel: dst.NewIdent(a.InsertFunc),
			},
			Args: []dst.Expr{ctxExpr},
		}
	} else {
		valueExpr = &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(a.WhatapAlias),
				Sel: dst.NewIdent(a.InsertFunc),
			},
		}
	}
	ctx.Lit.Elts = append(ctx.Lit.Elts, &dst.KeyValueExpr{
		Key:   dst.NewIdent(a.FieldName),
		Value: valueExpr,
	})
}

func (a *FieldWrapOrInsert) WhatapImportPath() string  { return a.WhatapPkg }
func (a *FieldWrapOrInsert) WhatapImportAlias() string { return a.WhatapAlias }

// ReplaceWithCtx replaces a function call and prepends a context argument.
// Used for net/http client functions that need context injection.
//
// Example (simple):
//
//	http.Get(url) → whataphttp.HttpGet(ctx, url)
//
// Example (DefaultClient):
//
//	http.DefaultClient.Get(url) → whataphttp.DefaultClientGet(ctx, url)
type ReplaceWithCtx struct {
	WhatapPkg   string // whatap import path
	WhatapAlias string // alias in code
	WhatapFunc  string // replacement function, e.g. "HttpGet", "DefaultClientGet"
	OrigVar     string // "DefaultClient" for pkg var access, "" for package-level
	OrigFunc    string // original function name for removal, e.g. "Get", "Post"
}

func (a *ReplaceWithCtx) Apply(ctx *MatchContext) {
	if ctx.Call == nil {
		ctx.Applied = false
		return
	}
	// §272 Phase 3 Step 3 — ModeRemove else branch removed.
	// Replace call.Fun entirely
	ctx.Call.Fun = &dst.SelectorExpr{
		X:   dst.NewIdent(a.WhatapAlias),
		Sel: dst.NewIdent(a.WhatapFunc),
	}
	// Prepend ctx arg
	ctxExpr := detectCtxExpr(ctx)
	ctx.Call.Args = append([]dst.Expr{ctxExpr}, ctx.Call.Args...)
}

func (a *ReplaceWithCtx) WhatapImportPath() string  { return a.WhatapPkg }
func (a *ReplaceWithCtx) WhatapImportAlias() string { return a.WhatapAlias }

// detectCtxExpr returns a context expression for CtxAware Advice.
// Uses enclosing function's handler context if available, otherwise nil.
func detectCtxExpr(ctx *MatchContext) dst.Expr {
	if ctx.EnclosingFunc != nil {
		if ctxExpr := common.DetectHandlerContext(ctx.EnclosingFunc); ctxExpr != nil {
			return ctxExpr
		}
	}
	return dst.NewIdent("nil")
}

// ── Transform — template-based transformation (10th Advice type) ──

// TransformContext provides template variables for Transform/Hook/Inject Advice.
type TransformContext struct {
	Original  string // full original call: "client.Put(policy, key, bins)"
	Var       string // first LHS variable of assignment
	Var1      string // second LHS variable (e.g., "err")
	Args      string // all arguments as text
	Arg0      string
	Arg1      string
	Arg2      string
	Arg3      string
	FuncName  string // function/method name: "Put"
	PkgName   string // local package alias: "aerospike" (for pkg calls) or receiver (for method calls)
	Receiver  string // method receiver variable: "client"
	Ctx       string // detected context expression or "context.Background()"
	TargetPkg string // package alias resolved from Target import path: "aerospike"

	// §227 Q6 additions
	ArgsList []string // all arguments (or parameter names for inject) as slice — for {{range}}
	ArgCount int      // len(ArgsList)
	HasCtx   bool     // call/decl has a context.Context argument/parameter
	File     string   // matched file identifier (inject-only; empty otherwise)

	// §236 — variadic-aware "rest of args after Arg0" placeholder.
	// Built from ArgsList[1:]. Preserves the original call's spread form so
	// the template substitution compiles for both spread (`f(h, hosts...)`)
	// and individual-arg (`f(h, a, b, c)`) call sites. Empty when ArgCount <= 1.
	Args1Plus string // e.g. "hosts..." or "a, b, c"
	IsSpread  bool   // true when the source call passes a slice with `...` (Call.Ellipsis)
}

// Transform applies a Go text/template to transform a matched call.
// Used for closure-wrapping patterns (aerospike, custom yaml transform).
// §272 Phase 3 Step 4 (2026-05-19): ReverseTarget field removed (auto-inject
// reverse mapping retired). User yaml `reverseTarget:` still parses but is
// silently ignored — see custom_rules.go for the deprecation warning.
type Transform struct {
	Template      string            // Go text/template string
	Imports       []string          // import paths to add
	ImportAliases map[string]string // import path → alias (e.g., "whatapdb")
}

func (a *Transform) Apply(ctx *MatchContext) {
	if ctx.Call == nil {
		ctx.Applied = false
		return
	}
	// §272 Phase 3 Step 3 — ModeRemove path removed (engine no longer
	// invokes Apply in remove mode). applyRemove/applyRemove-only helpers
	// are dead and will be deleted by Step 4.
	a.applyInject(ctx)
}

func (a *Transform) applyInject(ctx *MatchContext) {
	tc := buildCallTransformContext(ctx)

	// Execute template
	tmpl, err := template.New("transform").Parse(a.Template)
	if err != nil {
		ctx.Applied = false
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tc); err != nil {
		ctx.Applied = false
		return
	}

	// Parse template result into statements
	result := buf.String()

	// Handle assignment: if original was `x, err := call()`,
	// wrap result as `x, err := <template result>`
	if assign, ok := ctx.EnclosingStmt.(*dst.AssignStmt); ok {
		stmts, err := parseCodeBlock(result)
		if err != nil || len(stmts) == 0 {
			ctx.Applied = false
			return
		}
		// If parsed result is an expression statement, use its expression as RHS
		if exprStmt, ok := stmts[0].(*dst.ExprStmt); ok {
			for i, rhs := range assign.Rhs {
				if rhs == ctx.Call {
					assign.Rhs[i] = exprStmt.X
					break
				}
			}
		} else {
			// Multi-statement: replace in parent block
			a.replaceInBlock(ctx, stmts)
		}
	} else if _, ok := ctx.EnclosingStmt.(*dst.ExprStmt); ok {
		stmts, err := parseCodeBlock(result)
		if err != nil || len(stmts) == 0 {
			ctx.Applied = false
			return
		}
		a.replaceInBlock(ctx, stmts)
	}

	// Record extra imports
	if ctx.ExtraImports == nil {
		ctx.ExtraImports = make(map[string]string)
	}
	for _, imp := range a.Imports {
		alias := ""
		if a.ImportAliases != nil {
			alias = a.ImportAliases[imp]
		}
		ctx.ExtraImports[imp] = alias
	}
}

// §272 Phase 3 Step 3 — removed Transform.applyRemove (ModeRemove-only).

// replaceInBlock replaces the statement at StmtIndex with new statements.
func (a *Transform) replaceInBlock(ctx *MatchContext, newStmts []dst.Stmt) {
	if ctx.ParentBlock == nil || ctx.StmtIndex < 0 {
		return
	}
	block := *ctx.ParentBlock
	if ctx.StmtIndex >= len(block) {
		return
	}

	// Splice: remove original, insert new
	result := make([]dst.Stmt, 0, len(block)-1+len(newStmts))
	result = append(result, block[:ctx.StmtIndex]...)
	result = append(result, newStmts...)
	result = append(result, block[ctx.StmtIndex+1:]...)
	*ctx.ParentBlock = result
}

// §272 Phase 3 Step 3 — removed extractOriginalFromClosure /
// resolveOriginalPkgAlias / origFuncFromRuleTarget (ModeRemove-only helpers).

// resolveTargetPkgAlias finds the local package alias for the Target's import path.
// e.g., Target "github.com/aerospike/aerospike-client-go/v6.Client.Get" → finds import
// "github.com/aerospike/aerospike-client-go/v6" → local name "aerospike" (from alias or path.Base).
func resolveTargetPkgAlias(ctx *MatchContext) string {
	if ctx.File == nil || ctx.Target == "" {
		return ctx.PkgName
	}

	// Extract import path from target: remove .Type.Method or .Func suffix
	targetImport := ctx.Target
	// Find last segment that looks like an exported name (starts with uppercase)
	for {
		lastDot := strings.LastIndex(targetImport, ".")
		if lastDot < 0 {
			break
		}
		suffix := targetImport[lastDot+1:]
		if len(suffix) > 0 && suffix[0] >= 'A' && suffix[0] <= 'Z' {
			targetImport = targetImport[:lastDot]
		} else {
			break
		}
	}

	// Search file imports for this path
	for _, imp := range ctx.File.Imports {
		impPath := strings.Trim(imp.Path.Value, `"`)
		if impPath == targetImport {
			if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" {
				return imp.Name.Name // explicit alias
			}
			// No alias: use last path segment (strip version suffix)
			base := path.Base(impPath)
			if isVersionSuffix(base) {
				parent := path.Dir(impPath)
				base = path.Base(parent)
			}
			return base
		}
	}

	return ctx.PkgName // fallback
}

// isVersionSuffix returns true if s looks like "v2", "v3", etc.
func isVersionSuffix(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (a *Transform) WhatapImportPath() string {
	if len(a.Imports) > 0 {
		return a.Imports[0]
	}
	return ""
}

func (a *Transform) WhatapImportAlias() string {
	if a.ImportAliases != nil && len(a.Imports) > 0 {
		return a.ImportAliases[a.Imports[0]]
	}
	return ""
}

// ── Hook — insert code before/after a matched call statement (§227) ──

// Hook wraps a matched call statement with user-provided before/after code.
// Before code is inserted as statements immediately prior to the matched statement.
// After code is inserted as statements immediately following the matched statement.
// Both Before and After support template variables from TransformContext.
type Hook struct {
	Before        string            // code inserted before the matched statement
	After         string            // code inserted after the matched statement
	Imports       []string          // extra imports needed by the code
	ImportAliases map[string]string // import path → alias
}

func (a *Hook) Apply(ctx *MatchContext) {
	// §272 Phase 3 Step 3 — Apply only runs from ModeInject paths; ModeRemove
	// guard removed (engine.Process(ModeRemove) no longer invoked).
	if ctx.ParentBlock == nil || ctx.StmtIndex < 0 || ctx.EnclosingStmt == nil {
		ctx.Applied = false
		return
	}
	// §287 — Hook inserts sibling statements without transforming the call node,
	// so the engine's outer-context 1st pass would double-insert (and may
	// reference out-of-scope variables) for calls in nested blocks. Mirror the
	// CodeInsert guard: only insert when the call is directly in EnclosingStmt;
	// nested-block calls are handled by the processNestedBlocks pass.
	if ctx.Call != nil && !stmtOwnsCallDirectly(ctx.EnclosingStmt, ctx.Call) {
		ctx.Applied = false
		return
	}

	tc := buildCallTransformContext(ctx)
	beforeStmts, err := evalTemplateStmts(tc, a.Before)
	if err != nil {
		ctx.Applied = false
		return
	}
	afterStmts, err := evalTemplateStmts(tc, a.After)
	if err != nil {
		ctx.Applied = false
		return
	}
	if len(beforeStmts) == 0 && len(afterStmts) == 0 {
		ctx.Applied = false
		return
	}

	block := *ctx.ParentBlock
	result := make([]dst.Stmt, 0, len(block)+len(beforeStmts)+len(afterStmts))
	result = append(result, block[:ctx.StmtIndex]...)
	result = append(result, beforeStmts...)
	result = append(result, block[ctx.StmtIndex])
	result = append(result, afterStmts...)
	result = append(result, block[ctx.StmtIndex+1:]...)
	*ctx.ParentBlock = result

	a.recordImports(ctx)
	ctx.Applied = true
}

func (a *Hook) recordImports(ctx *MatchContext) {
	if len(a.Imports) == 0 {
		return
	}
	if ctx.ExtraImports == nil {
		ctx.ExtraImports = make(map[string]string)
	}
	for _, imp := range a.Imports {
		alias := ""
		if a.ImportAliases != nil {
			alias = a.ImportAliases[imp]
		}
		ctx.ExtraImports[imp] = alias
	}
}

func (a *Hook) WhatapImportPath() string {
	if len(a.Imports) > 0 {
		return a.Imports[0]
	}
	return ""
}

func (a *Hook) WhatapImportAlias() string {
	if a.ImportAliases != nil && len(a.Imports) > 0 {
		return a.ImportAliases[a.Imports[0]]
	}
	return ""
}

// ── Inject — insert start/end code into a matched function declaration (§227) ──

// Inject inserts user-provided code at the beginning and end of a function body.
// Start code is prepended to fn.Body.List.
// End code is wrapped in a defer closure so it runs at function exit.
// Both support TransformContext template variables computed from the FuncDecl.
type Inject struct {
	Start         string            // code inserted at function start
	End           string            // code wrapped in defer func(){...}() at function start
	Imports       []string          // extra imports needed by the code
	ImportAliases map[string]string // import path → alias
}

func (a *Inject) Apply(ctx *MatchContext) {
	// §272 Phase 3 Step 3 — ModeRemove guard removed (see Hook above).
	if ctx.Decl == nil || ctx.Decl.Body == nil {
		ctx.Applied = false
		return
	}

	tc := buildDeclTransformContext(ctx)
	startStmts, err := evalTemplateStmts(tc, a.Start)
	if err != nil {
		ctx.Applied = false
		return
	}
	endStmts, err := evalTemplateStmts(tc, a.End)
	if err != nil {
		ctx.Applied = false
		return
	}
	if len(startStmts) == 0 && len(endStmts) == 0 {
		ctx.Applied = false
		return
	}

	body := ctx.Decl.Body
	// Build new body: [startStmts, deferEnd (if any), original stmts...]
	prefix := make([]dst.Stmt, 0, len(startStmts)+1)
	prefix = append(prefix, startStmts...)
	if len(endStmts) > 0 {
		deferStmt := &dst.DeferStmt{
			Call: &dst.CallExpr{
				Fun: &dst.FuncLit{
					Type: &dst.FuncType{Params: &dst.FieldList{}},
					Body: &dst.BlockStmt{List: endStmts},
				},
			},
		}
		prefix = append(prefix, deferStmt)
	}
	body.List = append(prefix, body.List...)

	if ctx.ExtraImports == nil {
		ctx.ExtraImports = make(map[string]string)
	}
	for _, imp := range a.Imports {
		alias := ""
		if a.ImportAliases != nil {
			alias = a.ImportAliases[imp]
		}
		ctx.ExtraImports[imp] = alias
	}
	ctx.Applied = true
}

func (a *Inject) WhatapImportPath() string {
	if len(a.Imports) > 0 {
		return a.Imports[0]
	}
	return ""
}

func (a *Inject) WhatapImportAlias() string {
	if a.ImportAliases != nil && len(a.Imports) > 0 {
		return a.ImportAliases[a.Imports[0]]
	}
	return ""
}
