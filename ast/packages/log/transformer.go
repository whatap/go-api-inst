// Package log provides the standard log package transformer.
package log

import (
	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements ast.Transformer for standard log package.
// Handles both global log.SetOutput() and log.New() instance writer wrapping.
type Transformer struct {
	transformed    bool // tracks if any transformation was made
	hasNewInstance bool // tracks if log.New() wrapping occurred
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "log"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "log"
}

// WhatapImport returns empty string because log transformer adds import during Inject().
func (t *Transformer) WhatapImport() string {
	return "" // Import is added during Inject() if transformation occurs
}

// Detect checks if the file uses log package.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// Inject adds log.SetOutput(logsink.GetTraceLogWriter(os.Stderr)) after defer trace.Shutdown().
// Also wraps log.New() first argument (writer) with logsink.GetTraceLogWriter().
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false
	t.hasNewInstance = false

	// Get the actual package name used in code (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

	// 1. Global log.SetOutput() in main() (existing logic)
	mainFn := common.FindMainFunc(file)
	if mainFn != nil {
		shutdownIdx := common.FindDeferShutdownIndex(mainFn)
		if shutdownIdx >= 0 {
			// Check if SetOutput already exists
			if !hasSetOutputWithLogsink(mainFn, pkgName) {
				setOutputStmt := createSetOutputStmtWithPkg(pkgName)
				common.InsertStmtAfterIndex(mainFn, shutdownIdx, setOutputStmt)
				t.transformed = true
			}
		}
	}

	// 2. Wrap log.New() first argument with logsink.GetTraceLogWriter()
	// Use dst.Inspect on FuncDecl bodies to detect calls in any context
	// (struct fields, return statements, function arguments, etc.)
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

			callPkg, callFunc := getCallPkgAndFunc(call)
			if callPkg == pkgName && callFunc == "New" && len(call.Args) >= 1 {
				if !isLogsinkGetTraceLogWriter(call.Args[0]) {
					call.Args[0] = createLogsinkWrapExpr(call.Args[0])
					t.hasNewInstance = true
					t.transformed = true
				}
			}
			return true
		})
	}

	// Add required imports if any transformation occurred
	if t.transformed || t.hasNewInstance {
		common.AddImport(file, `"os"`)
		common.AddImport(file, `"github.com/whatap/go-api/logsink"`)
		t.transformed = true
	}

	return t.transformed, nil
}

// Remove removes log.SetOutput(logsink.GetTraceLogWriter(...)) and unwraps log.New() args.
func (t *Transformer) Remove(file *dst.File) error {
	// Get the actual package name used in code (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		pkgName = "log"
	}

	// 1. Remove log.SetOutput(logsink.GetTraceLogWriter(...)) in main()
	mainFn := common.FindMainFunc(file)
	if mainFn != nil {
		common.RemoveStmt(mainFn, isLogSetOutputWithLogsink)
	}

	// 2. Unwrap log.New() first argument via dst.Inspect on FuncDecl bodies
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

			callPkg, callFunc := getCallPkgAndFunc(call)
			if callPkg == pkgName && callFunc == "New" && len(call.Args) >= 1 {
				if innerCall, ok := call.Args[0].(*dst.CallExpr); ok {
					if isLogsinkGetTraceLogWriter(innerCall) && len(innerCall.Args) == 1 {
						call.Args[0] = innerCall.Args[0]
					}
				}
			}
			return true
		})
	}

	// 3. Also unwrap in package-level var declarations
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*dst.ValueSpec)
			if !ok {
				continue
			}
			for _, val := range valueSpec.Values {
				call, ok := val.(*dst.CallExpr)
				if !ok {
					continue
				}
				callPkg, callFunc := getCallPkgAndFunc(call)
				if callPkg == pkgName && callFunc == "New" && len(call.Args) >= 1 {
					if innerCall, ok := call.Args[0].(*dst.CallExpr); ok {
						if isLogsinkGetTraceLogWriter(innerCall) && len(innerCall.Args) == 1 {
							call.Args[0] = innerCall.Args[0]
						}
					}
				}
			}
		}
	}

	// Remove os import if no longer used
	common.RemoveUnusedImport(file, "os")

	// Remove logsink import if no longer used
	common.RemoveImportIfUnused(file, "github.com/whatap/go-api/logsink", "logsink")

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

// isLogsinkGetTraceLogWriter checks if expr is logsink.GetTraceLogWriter(...)
func isLogsinkGetTraceLogWriter(expr dst.Expr) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}
	pkg, fn := getCallPkgAndFunc(call)
	return pkg == "logsink" && fn == "GetTraceLogWriter"
}

// createLogsinkWrapExpr creates: logsink.GetTraceLogWriter(innerExpr)
func createLogsinkWrapExpr(innerExpr dst.Expr) *dst.CallExpr {
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   dst.NewIdent("logsink"),
			Sel: dst.NewIdent("GetTraceLogWriter"),
		},
		Args: []dst.Expr{innerExpr},
	}
}

// hasSetOutputWithLogsink checks if main function already has log.SetOutput(logsink...) call.
func hasSetOutputWithLogsink(fn *dst.FuncDecl, pkgName string) bool {
	for _, stmt := range fn.Body.List {
		if isSetOutputWithLogsinkPkg(stmt, pkgName) {
			return true
		}
	}
	return false
}

// isSetOutputWithLogsinkPkg checks if stmt is pkgName.SetOutput(logsink.GetTraceLogWriter(...))
func isSetOutputWithLogsinkPkg(stmt dst.Stmt, pkgName string) bool {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok || sel.Sel.Name != "SetOutput" {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok || ident.Name != pkgName {
		return false
	}

	if len(call.Args) != 1 {
		return false
	}

	return isLogsinkGetTraceLogWriter(call.Args[0])
}

// createSetOutputStmt creates: log.SetOutput(logsink.GetTraceLogWriter(os.Stderr))
func createSetOutputStmt() *dst.ExprStmt {
	return createSetOutputStmtWithPkg("log")
}

// createSetOutputStmtWithPkg creates: pkgName.SetOutput(logsink.GetTraceLogWriter(os.Stderr))
func createSetOutputStmtWithPkg(pkgName string) *dst.ExprStmt {
	stmt := &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(pkgName),
				Sel: dst.NewIdent("SetOutput"),
			},
			Args: []dst.Expr{
				createLogsinkWrapExpr(&dst.SelectorExpr{
					X:   dst.NewIdent("os"),
					Sel: dst.NewIdent("Stderr"),
				}),
			},
		},
	}
	stmt.Decs.After = dst.NewLine
	return stmt
}

// isLogSetOutputWithLogsink checks if stmt is log.SetOutput(logsink.GetTraceLogWriter(...))
func isLogSetOutputWithLogsink(stmt dst.Stmt) bool {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return false
	}

	// Check if it's log.SetOutput(...)
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok || sel.Sel.Name != "SetOutput" {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok || ident.Name != "log" {
		return false
	}

	// Check if argument is logsink.GetTraceLogWriter(...)
	if len(call.Args) != 1 {
		return false
	}

	return isLogsinkGetTraceLogWriter(call.Args[0])
}
