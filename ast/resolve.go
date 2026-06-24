package ast

import (
	"github.com/dave/dst"
	"github.com/whatap/go-api-inst/ast/common"
)

// resolveTarget converts a dst.Node to a Target string using go/types.
// Returns "" if the node is not resolvable or not a target we care about.
//
// Handles four patterns:
//   - CallExpr with SelectorExpr: pkg.Func() or receiver.Method()
//   - CompositeLit with SelectorExpr: pkg.Type{}
//   - FuncDecl: function/method declaration → "decl:name" or "decl:pkg.Type.Method"
func resolveTarget(node dst.Node) string {
	switch n := node.(type) {
	case *dst.CallExpr:
		return resolveCallTarget(n)
	case *dst.CompositeLit:
		return resolveLitTarget(n)
	case *dst.FuncDecl:
		return resolveFuncDeclTarget(n)
	}
	return ""
}

// resolveCallTarget resolves a call expression to a Target string.
// Handles three patterns:
//
//	A. pkg.Func(args)              — selector with package ident
//	B. receiver.Method(args)        — selector with value/pointer receiver
//	C. funcName(args)               — bare identifier referring to a (local
//	                                  or dot-imported) function (§227 Step 5)
func resolveCallTarget(call *dst.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *dst.SelectorExpr:
		// Try Pattern A first: pkg.Func(args) — ident is a package reference
		if ident, ok := fun.X.(*dst.Ident); ok {
			target := resolveIdentTarget(ident, fun.Sel.Name)
			if target != "" {
				return target
			}
			// ident is a local variable (not a package) → fall through to method call
		}
		// Pattern B: receiver.Method(args) — sel.X is a variable/expression
		// Use go/types to resolve receiver type
		return resolveMethodTarget(fun)
	case *dst.Ident:
		// Pattern C: bare-identifier function call. Resolve via go/types Uses
		// to obtain the function's full importpath.FuncName form. Returns ""
		// for builtins (len, make, …) and unresolved/free idents — safe
		// direction: the engine simply skips unresolved calls.
		return common.GetIdentFuncTarget(fun)
	}
	return ""
}

// resolveIdentTarget resolves pkg.Name where ident is a package identifier.
//
// Inject mode (type info available): uses go/types to obtain full import path.
// Remove mode or no type info: §237 옵션 E — falls back to "<ident>.<name>" so
// that whatap-injected calls like "whatapgin.WrapEngine(...)" can be matched
// against alias-based reverse keys registered in registry.go (Rule.Advice 의
// WhatapAlias + WhatapFunc 그대로). Inject 경로에서 type info 가 살아있을 때는
// 이 fallback 이 도달하지 않으므로 기존 정밀 매칭이 유지된다.
func resolveIdentTarget(ident *dst.Ident, name string) string {
	if common.HasTypeInfo() {
		importPath := common.GetIdentPath(ident)
		if importPath != "" {
			return importPath + "." + name
		}
		// type info 가 있으나 ident 가 패키지 참조가 아님 (local variable 등)
		// → fallback 으로 떨어지지 않고 매칭 안 함이 안전 (오탐 방지)
		return ""
	}
	// No type info (typically remove mode): use ident name directly.
	// 매칭은 registry.whatapRules 에 등록된 alias key (e.g. "whatapgin.WrapEngine") 기준.
	return ident.Name + "." + name
}

// resolveMethodTarget resolves receiver.Method() using go/types.
// Returns "pkg.Type.Method" format, e.g. "github.com/gorilla/mux.Route.Subrouter".
//
// Uses common.NamedTypeOf (not types.Type.String()) so that generic receivers
// resolve to the origin type without type arguments — e.g.
// compose.Chain[string, *schema.Message].AppendChatModel →
// "github.com/cloudwego/eino/compose.Chain.AppendChatModel" — which a static
// Rule target can match (§282). Non-generic named types are unaffected: their
// path/name equal the t.String() form (e.g. github.com/gorilla/mux.Route).
func resolveMethodTarget(sel *dst.SelectorExpr) string {
	pkgPath, typeName, ok := common.NamedTypeOf(sel.X)
	if !ok {
		return ""
	}
	return pkgPath + "." + typeName + "." + sel.Sel.Name
}

// resolveLitTarget resolves a composite literal to a Target string.
// Example: &http.Client{} → "net/http.Client{}"
func resolveLitTarget(lit *dst.CompositeLit) string {
	// Handle &pkg.Type{} — the & is in parent UnaryExpr, CompositeLit has Type
	sel, ok := lit.Type.(*dst.SelectorExpr)
	if !ok {
		return ""
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return ""
	}

	target := resolveIdentTarget(ident, sel.Sel.Name)
	if target == "" {
		return ""
	}
	return target + "{}"
}

// resolveFuncDeclTarget resolves a function/method declaration to a Target string.
// Format:
//   - Function: "decl:pkgpath.funcName" when import path known; "decl:funcName" as fallback
//   - Method:   "decl:pkg.Type.Method"  (e.g. "decl:net/http.Server.ListenAndServe")
func resolveFuncDeclTarget(fn *dst.FuncDecl) string {
	if fn.Name == nil {
		return ""
	}

	// Method declaration: has receiver
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		return resolveFuncDeclMethodTarget(fn)
	}

	// Function declaration: no receiver
	// Prefer fully-qualified form "decl:pkgpath.funcName" when go/types + package path known.
	if pkgPath := common.GetCurrentImportPath(); pkgPath != "" {
		return "decl:" + pkgPath + "." + fn.Name.Name
	}
	return "decl:" + fn.Name.Name
}

// resolveFuncDeclMethodTarget resolves a method declaration using receiver type.
// Example: func (s *http.Server) ListenAndServe() → "decl:net/http.Server.ListenAndServe"
func resolveFuncDeclMethodTarget(fn *dst.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}

	recvType := fn.Recv.List[0].Type

	// Dereference pointer: *Type → Type (also *Type[T] → Type[T])
	if star, ok := recvType.(*dst.StarExpr); ok {
		recvType = star.X
	}

	// Resolve the receiver type via go/types. NamedTypeOf drops generic type
	// parameters (func (c *Chain[I, O]) M → "...compose.Chain.M"), §282.
	if pkgPath, typeName, ok := common.NamedTypeOf(recvType); ok {
		return "decl:" + pkgPath + "." + typeName + "." + fn.Name.Name
	}

	// Fallback: use ident name from receiver
	switch rt := recvType.(type) {
	case *dst.Ident:
		return "decl:" + rt.Name + "." + fn.Name.Name
	case *dst.SelectorExpr:
		if ident, ok := rt.X.(*dst.Ident); ok {
			return "decl:" + ident.Name + "." + rt.Sel.Name + "." + fn.Name.Name
		}
	}

	return ""
}

// newResolveFunc returns the ResolveFunc for v2types method.
func newResolveFunc() ResolveFunc {
	return resolveTarget
}
