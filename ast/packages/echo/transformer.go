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

// Detect checks if the file uses Echo framework.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImportPrefix(file, t.ImportPath())
}

// Inject adds Echo middleware instrumentation.
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	transformed := false

	// Get the actual package name used in code (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var newList []dst.Stmt
		for _, stmt := range fn.Body.List {
			newList = append(newList, stmt)

			// Check if this is an assignment statement
			assign, ok := stmt.(*dst.AssignStmt)
			if !ok || len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
				continue
			}

			// Check if RHS is a function call
			call, ok := assign.Rhs[0].(*dst.CallExpr)
			if !ok {
				continue
			}

			// Get the variable name and function name
			varName := getVarName(assign.Lhs[0])
			callPkg, callFunc := getCallPkgAndFunc(call)

			// Check if it's pkgName.New()
			if callPkg == pkgName && callFunc == "New" {
				middlewareStmt := createMiddlewareStmt(varName)
				newList = append(newList, middlewareStmt)
				transformed = true
			}
		}
		fn.Body.List = newList
	}

	// Add correct whatap import based on v3/v4
	if transformed {
		common.AddImport(file, t.WhatapImportForFile(file))
	}

	return transformed, nil
}

// Remove removes Echo middleware instrumentation.
func (t *Transformer) Remove(file *dst.File) error {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var newList []dst.Stmt
		for _, stmt := range fn.Body.List {
			if isWhatapMiddlewareCall(stmt) {
				continue // Remove whatap middleware call
			}
			newList = append(newList, stmt)
		}
		fn.Body.List = newList
	}
	return nil
}

// getVarName extracts variable name from expression.
func getVarName(expr dst.Expr) string {
	if ident, ok := expr.(*dst.Ident); ok {
		return ident.Name
	}
	return ""
}

// getCallFuncName extracts the full function name from a call expression.
func getCallFuncName(call *dst.CallExpr) string {
	if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
		if ident, ok := sel.X.(*dst.Ident); ok {
			return ident.Name + "." + sel.Sel.Name
		}
	}
	return ""
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

// createMiddlewareStmt creates the middleware injection statement.
// Example: e.Use(whatapecho.Middleware())
func createMiddlewareStmt(varName string) dst.Stmt {
	stmt := &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(varName),
				Sel: dst.NewIdent("Use"),
			},
			Args: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent("whatapecho"),
						Sel: dst.NewIdent("Middleware"),
					},
				},
			},
		},
	}
	stmt.Decs.After = dst.NewLine
	return stmt
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
