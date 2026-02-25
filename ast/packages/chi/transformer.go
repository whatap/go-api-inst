// Package chi provides the Chi router transformer.
package chi

import (
	"strings"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements ast.Transformer for Chi router.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "chi"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/go-chi/chi"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi"
}

// SupportedVersions returns the supported major versions for Chi.
// "" = chi without version suffix (v1-v4), "v5" = chi/v5.
func (t *Transformer) SupportedVersions() []string {
	return []string{"", "v5"}
}

// Detect checks if the file uses Chi router (supported versions only).
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasSupportedImport(file, t.ImportPath(), t.SupportedVersions())
}

// Inject adds Chi middleware instrumentation via in-place CallExpr wrapping.
// Transforms: chi.NewRouter() → whatapchi.WrapRouter(chi.NewRouter())
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
			if pkg == "whatapchi" && funcName == "WrapRouter" {
				return false
			}

			if pkg == pkgName && funcName == "NewRouter" {
				wrapCallExpr(call, "whatapchi", "WrapRouter")
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

	return t.transformed, nil
}

// Remove removes Chi middleware instrumentation.
func (t *Transformer) Remove(file *dst.File) error {
	// Phase 1: Unwrap whatapchi.WrapRouter(expr) → expr
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
			if pkg == "whatapchi" && funcName == "WrapRouter" && len(call.Args) == 1 {
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

// isWhatapMiddlewareCall checks if the statement is a whatapchi middleware call.
// Chi uses function value: r.Use(whatapchi.Middleware)
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

	// Chi uses whatapchi.Middleware (function value) not Middleware()
	if argSel, ok := call.Args[0].(*dst.SelectorExpr); ok {
		if ident, ok := argSel.X.(*dst.Ident); ok {
			return strings.HasPrefix(ident.Name, "whatap") && argSel.Sel.Name == "Middleware"
		}
	}

	return false
}
