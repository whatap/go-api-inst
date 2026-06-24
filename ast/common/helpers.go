// Package common provides shared AST utilities.
// helpers.go: Common CallExpr helper functions shared across transformers (§159).
package common

import (
	"github.com/dave/dst"
)

// GetCallPkgAndFunc extracts package name and function name from a call expression.
// Returns ("", "") if the call is not a pkg.Func() pattern.
func GetCallPkgAndFunc(call *dst.CallExpr) (string, string) {
	if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
		if ident, ok := sel.X.(*dst.Ident); ok {
			return ident.Name, sel.Sel.Name
		}
	}
	return "", ""
}

// WrapCallExpr wraps a CallExpr in-place: pkg.Func(args) -> wrapPkg.wrapFunc(pkg.Func(args))
func WrapCallExpr(call *dst.CallExpr, wrapPkg, wrapFunc string) {
	originalFun := call.Fun
	originalArgs := make([]dst.Expr, len(call.Args))
	copy(originalArgs, call.Args)
	originalEllipsis := call.Ellipsis

	innerCall := &dst.CallExpr{
		Fun:      originalFun,
		Args:     originalArgs,
		Ellipsis: originalEllipsis,
	}

	call.Fun = &dst.SelectorExpr{
		X:   dst.NewIdent(wrapPkg),
		Sel: dst.NewIdent(wrapFunc),
	}
	call.Args = []dst.Expr{innerCall}
	call.Ellipsis = false
}

// UnwrapCallExpr unwraps a CallExpr in-place: wrapPkg.wrapFunc(inner) -> inner
func UnwrapCallExpr(call *dst.CallExpr) {
	if len(call.Args) != 1 {
		return
	}
	innerCall, ok := call.Args[0].(*dst.CallExpr)
	if !ok {
		return
	}
	call.Fun = innerCall.Fun
	call.Args = innerCall.Args
	call.Ellipsis = innerCall.Ellipsis
}

// IsWhatapMiddlewareCall checks if the statement is a whatap middleware call.
// Handles two patterns:
//   - Call pattern: r.Use(whatapXxx.Middleware()) — used by gin, echo, fiber, gorilla
//   - Value pattern: r.Use(whatapXxx.Middleware) — used by chi
//
// whatapPkgName is the default package name (e.g., "whatapgin").
// whatapImportPath is the full import path for go/types precise matching.
func IsWhatapMiddlewareCall(stmt dst.Stmt, whatapPkgName, whatapImportPath string) bool {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok || sel.Sel.Name != "Use" {
		return false
	}

	if len(call.Args) != 1 {
		return false
	}

	// Pattern 1: Function call — whatapXxx.Middleware()
	if argCall, ok := call.Args[0].(*dst.CallExpr); ok {
		if argSel, ok := argCall.Fun.(*dst.SelectorExpr); ok {
			if ident, ok := argSel.X.(*dst.Ident); ok {
				if MatchIdentPkg(ident, whatapPkgName, whatapImportPath) && argSel.Sel.Name == "Middleware" {
					return true
				}
			}
		}
	}

	// Pattern 2: Function value — whatapXxx.Middleware
	if argSel, ok := call.Args[0].(*dst.SelectorExpr); ok {
		if ident, ok := argSel.X.(*dst.Ident); ok {
			return MatchIdentPkg(ident, whatapPkgName, whatapImportPath) && argSel.Sel.Name == "Middleware"
		}
	}

	return false
}
