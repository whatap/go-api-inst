// Package fiber provides the Fiber framework transformer.
package fiber

import (
	"strings"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

const whatapImportPath = "github.com/whatap/go-api/instrumentation/github.com/gofiber/fiber/v2/whatapfiber"

// Transformer implements ast.Transformer for Fiber framework.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "fiber"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/gofiber/fiber"
}

// WhatapImport returns empty string because fiber transformer adds import during Inject().
func (t *Transformer) WhatapImport() string {
	return "" // Import is added during Inject() if transformation occurs
}

// SupportedVersions returns the supported major versions for Fiber.
// "v2" = fiber/v2.
func (t *Transformer) SupportedVersions() []string {
	return []string{"v2"}
}

// Detect checks if the file uses Fiber framework (supported versions only).
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasSupportedImport(file, t.ImportPath(), t.SupportedVersions())
}

// Inject adds Fiber middleware instrumentation via in-place CallExpr wrapping.
// Transforms: fiber.New() → whatapfiber.WrapApp(fiber.New())
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false

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

			pkg, funcName := getCallPkgAndFunc(call)
			if pkg == "whatapfiber" && funcName == "WrapApp" {
				return false
			}

			if pkg == pkgName && funcName == "New" {
				wrapCallExpr(call, "whatapfiber", "WrapApp")
				t.transformed = true
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
				t.transformed = true
				continue
			}
			newList = append(newList, stmt)
		}
		block.List = newList

		return true
	})

	if t.transformed {
		common.AddImport(file, whatapImportPath)
	}

	return t.transformed, nil
}

// Remove removes Fiber middleware instrumentation.
func (t *Transformer) Remove(file *dst.File) error {
	// Phase 1: Unwrap whatapfiber.WrapApp(expr) → expr
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
			if pkg == "whatapfiber" && funcName == "WrapApp" && len(call.Args) == 1 {
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

// wrapCallExpr wraps a CallExpr in-place.
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

// unwrapCallExpr unwraps a CallExpr in-place.
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

// isWhatapMiddlewareCall checks if the statement is a whatapfiber middleware call.
func isWhatapMiddlewareCall(stmt dst.Stmt) bool {
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

	if argCall, ok := call.Args[0].(*dst.CallExpr); ok {
		if argSel, ok := argCall.Fun.(*dst.SelectorExpr); ok {
			if ident, ok := argSel.X.(*dst.Ident); ok {
				return strings.HasPrefix(ident.Name, "whatap") && argSel.Sel.Name == "Middleware"
			}
		}
	}

	return false
}
