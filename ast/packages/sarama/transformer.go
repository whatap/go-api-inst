// Package sarama provides the IBM/sarama (Kafka) transformer.
package sarama

import (
	"go/token"
	"strings"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for sarama.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "sarama"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/IBM/sarama"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/github.com/IBM/sarama/whatapsarama"
}

// Detect checks if the file uses sarama.
func (t *Transformer) Detect(file *dst.File) bool {
	// Check for both IBM/sarama and Shopify/sarama
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == "github.com/IBM/sarama" || path == "github.com/Shopify/sarama" {
			return true
		}
	}
	return false
}

// Inject adds whatapsarama.WrapConfig() wrapping around sarama.NewConfig() calls.
// Uses dst.Inspect on CallExpr to detect calls in any context (struct fields, etc).
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false

	// Get the actual package name used in code (could be alias)
	// Check both IBM/sarama and Shopify/sarama (deprecated)
	pkgName := common.GetPackageNameForImportPrefix(file, "github.com/IBM/sarama")
	if pkgName == "" {
		pkgName = common.GetPackageNameForImportPrefix(file, "github.com/Shopify/sarama")
	}
	if pkgName == "" {
		return false, nil
	}

	// Phase 1: Wrap sarama.NewConfig() calls in-place
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		dst.Inspect(fn.Body, func(n dst.Node) bool {
			call, ok := n.(*dst.CallExpr)
			if !ok {
				return true
			}

			// If this is already a wrap call, skip its children
			callPkg, callFunc := getCallPkgAndFunc(call)
			if callPkg == "whatapsarama" && callFunc == "WrapConfig" {
				return false
			}

			// Check for sarama.NewConfig()
			if callPkg == pkgName && callFunc == "NewConfig" {
				wrapCallExpr(call, "whatapsarama", "WrapConfig")
				t.transformed = true
				return false
			}

			return true
		})
	}

	// Phase 2: Clean up old-style interceptor statements (backward compatibility upgrade)
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var newList []dst.Stmt
		for _, stmt := range fn.Body.List {
			// Skip: whatapInterceptor := &whatapsarama.Interceptor{}
			if t.isWhatapsaramaInterceptorDecl(stmt) {
				t.transformed = true
				continue
			}

			// Skip: config.Producer.Interceptors = ... or config.Consumer.Interceptors = ...
			if t.isSaramaInterceptorAssign(stmt) {
				t.transformed = true
				continue
			}

			newList = append(newList, stmt)
		}
		fn.Body.List = newList
	}

	return t.transformed, nil
}

// Remove removes whatapsarama instrumentation.
func (t *Transformer) Remove(file *dst.File) error {
	// Phase 1: Unwrap whatapsarama.WrapConfig(expr) → expr
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		dst.Inspect(fn.Body, func(n dst.Node) bool {
			call, ok := n.(*dst.CallExpr)
			if !ok {
				return true
			}

			pkg, funcName := getCallPkgAndFunc(call)
			if pkg == "whatapsarama" && funcName == "WrapConfig" && len(call.Args) == 1 {
				unwrapCallExpr(call)
				return false
			}

			return true
		})
	}

	// Phase 2: Remove old-style interceptor statements (backward compatibility)
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var newList []dst.Stmt
		for _, stmt := range fn.Body.List {
			// Skip: whatapInterceptor := &whatapsarama.Interceptor{}
			if t.isWhatapsaramaInterceptorDecl(stmt) {
				continue
			}

			// Skip: config.Producer.Interceptors = ... or config.Consumer.Interceptors = ...
			if t.isSaramaInterceptorAssign(stmt) {
				continue
			}

			newList = append(newList, stmt)
		}
		fn.Body.List = newList
	}
	return nil
}

// getCallPkgAndFunc extracts package name and function name from a call expression.
func getCallPkgAndFunc(call *dst.CallExpr) (string, string) {
	if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
		if ident, ok := sel.X.(*dst.Ident); ok {
			return ident.Name, sel.Sel.Name
		}
	}
	return "", ""
}

// wrapCallExpr wraps a CallExpr in-place: pkg.Func(args) → wrapPkg.wrapFunc(pkg.Func(args))
func wrapCallExpr(call *dst.CallExpr, wrapPkg, wrapFunc string) {
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

// unwrapCallExpr unwraps a CallExpr in-place: wrapPkg.wrapFunc(inner) → inner
func unwrapCallExpr(call *dst.CallExpr) {
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

// isWhatapsaramaInterceptorDecl checks if stmt is whatapInterceptor := &whatapsarama.Interceptor{}
func (t *Transformer) isWhatapsaramaInterceptorDecl(stmt dst.Stmt) bool {
	assign, ok := stmt.(*dst.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return false
	}

	if len(assign.Rhs) == 0 {
		return false
	}

	unary, ok := assign.Rhs[0].(*dst.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return false
	}

	composite, ok := unary.X.(*dst.CompositeLit)
	if !ok {
		return false
	}

	sel, ok := composite.Type.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	return ident.Name == "whatapsarama" && sel.Sel.Name == "Interceptor"
}

// isSaramaInterceptorAssign checks if stmt is config.*.Interceptors = [...]{whatapInterceptor}
func (t *Transformer) isSaramaInterceptorAssign(stmt dst.Stmt) bool {
	assign, ok := stmt.(*dst.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return false
	}

	if len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
		return false
	}

	// Check LHS: *.Interceptors
	sel, ok := assign.Lhs[0].(*dst.SelectorExpr)
	if !ok || sel.Sel.Name != "Interceptors" {
		return false
	}

	// Check RHS contains whatapInterceptor
	composite, ok := assign.Rhs[0].(*dst.CompositeLit)
	if !ok {
		return false
	}

	for _, elt := range composite.Elts {
		if ident, ok := elt.(*dst.Ident); ok {
			if ident.Name == "whatapInterceptor" {
				return true
			}
		}
	}

	return false
}
