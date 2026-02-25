// Package gin provides the Gin framework transformer.
package gin

import (
	"strings"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

const whatapImportPath = "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"

// Transformer implements ast.Transformer for Gin framework.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "gin"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/gin-gonic/gin"
}

// WhatapImport returns empty string because gin transformer adds import during Inject().
func (t *Transformer) WhatapImport() string {
	return "" // Import is added during Inject() if transformation occurs
}

// Detect checks if the file uses Gin framework.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// Inject adds Gin middleware instrumentation via in-place CallExpr wrapping.
// Transforms: gin.Default() → whatapgin.WrapEngine(gin.Default())
// Transforms: gin.New() → whatapgin.WrapEngine(gin.New())
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false

	// Get the actual package name used in code (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

	// Phase 1: Wrap constructor calls in-place using dst.Inspect on CallExpr
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
			if pkg == "whatapgin" && funcName == "WrapEngine" {
				return false
			}

			// Check for gin.Default() or gin.New()
			if pkg == pkgName && (funcName == "Default" || funcName == "New") {
				wrapCallExpr(call, "whatapgin", "WrapEngine")
				t.transformed = true
				return false // don't inspect children of wrapped call
			}

			return true
		})
	}

	// Phase 2: Clean up old-style middleware statements (backward compatibility upgrade)
	dst.Inspect(file, func(n dst.Node) bool {
		block, ok := n.(*dst.BlockStmt)
		if !ok {
			return true
		}

		var newList []dst.Stmt
		for _, stmt := range block.List {
			if isWhatapMiddlewareCall(stmt) {
				t.transformed = true
				continue // Remove old-style middleware call
			}
			newList = append(newList, stmt)
		}
		block.List = newList

		return true
	})

	// Add import only if transformation occurred
	if t.transformed {
		common.AddImport(file, whatapImportPath)
	}

	return t.transformed, nil
}

// Remove removes Gin middleware instrumentation.
func (t *Transformer) Remove(file *dst.File) error {
	// Phase 1: Unwrap whatapgin.WrapEngine(expr) → expr
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
			if pkg == "whatapgin" && funcName == "WrapEngine" && len(call.Args) == 1 {
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
				continue // Remove whatap middleware call
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

// isWhatapMiddlewareCall checks if the statement is a whatapgin middleware call.
// Detects: varName.Use(whatapgin.Middleware())
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

	// Check if argument is whatapgin.Middleware()
	if argCall, ok := call.Args[0].(*dst.CallExpr); ok {
		if argSel, ok := argCall.Fun.(*dst.SelectorExpr); ok {
			if ident, ok := argSel.X.(*dst.Ident); ok {
				return strings.HasPrefix(ident.Name, "whatap") && argSel.Sel.Name == "Middleware"
			}
		}
	}

	return false
}
