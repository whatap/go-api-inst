// Package logrus provides the logrus package transformer.
package logrus

import (
	"strings"

	"go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements ast.Transformer for logrus package.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "logrus"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/sirupsen/logrus"
}

// WhatapImport returns empty string because logrus transformer adds import during Inject().
func (t *Transformer) WhatapImport() string {
	return "" // Import is added during Inject() if transformation occurs
}

// Detect checks if the file uses logrus package.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// Inject adds logrus.SetOutput(logsink.GetTraceLogWriter(os.Stderr)) after defer trace.Shutdown().
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false

	mainFn := common.FindMainFunc(file)
	if mainFn == nil {
		return false, nil // no main function, skip
	}

	// Find defer trace.Shutdown() position
	shutdownIdx := common.FindDeferShutdownIndex(mainFn)
	if shutdownIdx < 0 {
		return false, nil // defer trace.Shutdown() not found, skip
	}

	// Get the actual package name (could be alias like "log")
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

	// Add "os" import for os.Stderr
	common.AddImport(file, `"os"`)

	// Add logsink import
	common.AddImport(file, `"github.com/whatap/go-api/logsink"`)

	// Create: logrus.SetOutput(logsink.GetTraceLogWriter(os.Stderr))
	// or: log.SetOutput(...) if aliased as "log"
	setOutputStmt := createSetOutputStmt(pkgName)

	// Insert after defer trace.Shutdown()
	common.InsertStmtAfterIndex(mainFn, shutdownIdx, setOutputStmt)
	t.transformed = true

	return t.transformed, nil
}

// Remove removes logrus.SetOutput(logsink.GetTraceLogWriter(...)) statement.
func (t *Transformer) Remove(file *dst.File) error {
	mainFn := common.FindMainFunc(file)
	if mainFn == nil {
		return nil
	}

	// Get the actual package name (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		pkgName = "logrus" // fallback for remove case
	}

	// Remove SetOutput(logsink.GetTraceLogWriter(...)) statement
	common.RemoveStmt(mainFn, func(stmt dst.Stmt) bool {
		return isSetOutputWithLogsink(stmt, pkgName)
	})

	// Remove os import if no longer used
	common.RemoveUnusedImport(file, "os")

	return nil
}

// createSetOutputStmt creates: pkgName.SetOutput(logsink.GetTraceLogWriter(os.Stderr))
func createSetOutputStmt(pkgName string) *dst.ExprStmt {
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

// isSetOutputWithLogsink checks if stmt is pkgName.SetOutput(logsink.GetTraceLogWriter(...))
func isSetOutputWithLogsink(stmt dst.Stmt, pkgName string) bool {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return false
	}

	// Check if it's pkgName.SetOutput(...)
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok || sel.Sel.Name != "SetOutput" {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok || ident.Name != pkgName {
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
