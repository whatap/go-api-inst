// Package echo provides the Echo framework transformer.
package echo

import (
	"strings"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements ast.Transformer for Echo framework.
type Transformer struct{}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "echo"
}

// ImportPath returns the original package import path.
// Note: Handles both v4 (github.com/labstack/echo/v4) and old (github.com/labstack/echo)
func (t *Transformer) ImportPath() string {
	return "github.com/labstack/echo"
}

// WhatapImport returns empty because echo handles import in Inject() based on version.
func (t *Transformer) WhatapImport() string {
	// Different imports needed for v3/v4, so handled in Inject()
	return ""
}

// WhatapImportForFile returns the correct whatap import path based on the file's imports.
func (t *Transformer) WhatapImportForFile(file *dst.File) string {
	if t.isV4(file) {
		return "github.com/whatap/go-api/instrumentation/github.com/labstack/echo/v4/whatapecho"
	}
	return "github.com/whatap/go-api/instrumentation/github.com/labstack/echo/whatapecho"
}

// isV4 checks if the file uses Echo v4.
func (t *Transformer) isV4(file *dst.File) bool {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		// github.com/labstack/echo/v4 (v4 path)
		if strings.HasPrefix(path, "github.com/labstack/echo/v4") {
			return true
		}
	}
	return false
}

// SupportedVersions returns the supported major versions for Echo.
// "" = echo without version suffix (v1-v3), "v4" = echo/v4.
func (t *Transformer) SupportedVersions() []string {
	return []string{"", "v4"}
}

// Detect checks if the file uses Echo framework (supported versions only).
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasSupportedImport(file, t.ImportPath(), t.SupportedVersions())
}

// Inject adds Echo middleware instrumentation via in-place CallExpr wrapping.
// Transforms: echo.New() → whatapecho.WrapEcho(echo.New())
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	transformed := false

	// Get the actual package name used in code (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

	// Phase 1: Wrap constructor calls in-place
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
			pkg, funcName := getCallPkgAndFunc(call)
			if pkg == "whatapecho" && funcName == "WrapEcho" {
				return false
			}

			// Check for echo.New()
			if pkg == pkgName && funcName == "New" {
				wrapCallExpr(call, "whatapecho", "WrapEcho")
				transformed = true
				return false
			}

			return true
		})
	}

	// Phase 2: Clean up old-style middleware statements
	dst.Inspect(file, func(n dst.Node) bool {
		block, ok := n.(*dst.BlockStmt)
		if !ok {
			return true
		}

		var newList []dst.Stmt
		for _, stmt := range block.List {
			if isWhatapMiddlewareCall(stmt) {
				transformed = true
				continue
			}
			newList = append(newList, stmt)
		}
		block.List = newList

		return true
	})

	// Add correct whatap import based on v3/v4
	if transformed {
		common.AddImport(file, t.WhatapImportForFile(file))
	}

	return transformed, nil
}

// Remove removes Echo middleware instrumentation.
func (t *Transformer) Remove(file *dst.File) error {
	// Phase 1: Unwrap whatapecho.WrapEcho(expr) → expr
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
			if pkg == "whatapecho" && funcName == "WrapEcho" && len(call.Args) == 1 {
				unwrapCallExpr(call)
				return false
			}

			return true
		})
	}

	// Phase 2: Remove old-style middleware statements (backward compatibility)
	dst.Inspect(file, func(n dst.Node) bool {
		block, ok := n.(*dst.BlockStmt)
		if !ok {
			return true
		}

		var newList []dst.Stmt
		for _, stmt := range block.List {
			if isWhatapMiddlewareCall(stmt) {
				continue
			}
			newList = append(newList, stmt)
		}
		block.List = newList

		return true
	})
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

// isWhatapMiddlewareCall checks if the statement is a whatapecho middleware call.
func isWhatapMiddlewareCall(stmt dst.Stmt) bool {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return false
	}

	// Check if it's a .Use(...) call
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok || sel.Sel.Name != "Use" {
		return false
	}

	// Check arguments
	if len(call.Args) != 1 {
		return false
	}

	// Check if argument is whatapecho.Middleware()
	if argCall, ok := call.Args[0].(*dst.CallExpr); ok {
		if argSel, ok := argCall.Fun.(*dst.SelectorExpr); ok {
			if ident, ok := argSel.X.(*dst.Ident); ok {
				return strings.HasPrefix(ident.Name, "whatap") && argSel.Sel.Name == "Middleware"
			}
		}
	}

	return false
}
