// Package log provides the standard log package transformer.
package log

import (
	"strings"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements ast.Transformer for standard log package.
type Transformer struct {
	transformed bool // tracks if any transformation was made
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
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false

	// Get the actual package name used in code (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

	mainFn := common.FindMainFunc(file)
	if mainFn == nil {
		return false, nil // no main function, skip
	}

	// Find defer trace.Shutdown() position
	shutdownIdx := common.FindDeferShutdownIndex(mainFn)
	if shutdownIdx < 0 {
		return false, nil // defer trace.Shutdown() not found, skip
	}

	// Add "os" import for os.Stderr
	common.AddImport(file, `"os"`)

	// Add logsink import
	common.AddImport(file, `"github.com/whatap/go-api/logsink"`)

	// Create: log.SetOutput(logsink.GetTraceLogWriter(os.Stderr))
	// Use the actual package name (alias if present)
	setOutputStmt := createSetOutputStmtWithPkg(pkgName)

	// Insert after defer trace.Shutdown()
	common.InsertStmtAfterIndex(mainFn, shutdownIdx, setOutputStmt)
	t.transformed = true

	return t.transformed, nil
}

// Remove removes log.SetOutput(logsink.GetTraceLogWriter(...)) statement.
func (t *Transformer) Remove(file *dst.File) error {
	mainFn := common.FindMainFunc(file)
	if mainFn == nil {
		return nil
	}

	// Remove log.SetOutput(logsink.GetTraceLogWriter(...)) statement
	common.RemoveStmt(mainFn, isLogSetOutputWithLogsink)

	// Remove os import if no longer used
	common.RemoveUnusedImport(file, "os")

	return nil
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
				// logsink.GetTraceLogWriter(os.Stderr)
				&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent("logsink"),
						Sel: dst.NewIdent("GetTraceLogWriter"),
					},
					Args: []dst.Expr{
						&dst.SelectorExpr{
							X:   dst.NewIdent("os"),
							Sel: dst.NewIdent("Stderr"),
						},
					},
				},
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

	argCall, ok := call.Args[0].(*dst.CallExpr)
	if !ok {
		return false
	}

	argSel, ok := argCall.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	argIdent, ok := argSel.X.(*dst.Ident)
	if !ok {
		return false
	}

	return argIdent.Name == "logsink" && strings.HasPrefix(argSel.Sel.Name, "GetTraceLogWriter")
}
