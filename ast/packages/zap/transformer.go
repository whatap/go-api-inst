// Package zap provides the uber-go/zap package transformer.
package zap

import (
	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements ast.Transformer for zap package.
// Note: zap transformer uses logsink.HookStderr() to capture zap output.
// This is because zap uses os.Stderr directly and doesn't have SetOutput().
// Future enhancement: implement zapcore.WriteSyncer integration for TraceLogWriter.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "zap"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "go.uber.org/zap"
}

// WhatapImport returns empty string because zap transformer adds import during Inject().
func (t *Transformer) WhatapImport() string {
	return "" // Import is added during Inject() if transformation occurs
}

// Detect checks if the file uses zap package.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// Inject adds logsink.HookStderr() after defer trace.Shutdown() to capture zap output.
// Note: zap writes to os.Stderr by default (NewProduction, NewDevelopment).
// HookStderr() redirects os.Stderr to capture these logs.
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

	// Add logsink import
	common.AddImport(file, `"github.com/whatap/go-api/logsink"`)

	// Create: logsink.HookStderr()
	hookStmt := createHookStderrStmt()

	// Insert after defer trace.Shutdown()
	common.InsertStmtAfterIndex(mainFn, shutdownIdx, hookStmt)
	t.transformed = true

	return t.transformed, nil
}

// Remove removes logsink.HookStderr() statement.
func (t *Transformer) Remove(file *dst.File) error {
	mainFn := common.FindMainFunc(file)
	if mainFn == nil {
		return nil
	}

	// Remove logsink.HookStderr() statement
	common.RemoveStmt(mainFn, isLogsinkHookStderr)

	return nil
}

// createHookStderrStmt creates: logsink.HookStderr()
func createHookStderrStmt() *dst.ExprStmt {
	stmt := &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("logsink"),
				Sel: dst.NewIdent("HookStderr"),
			},
		},
	}
	stmt.Decs.After = dst.NewLine
	return stmt
}

// isLogsinkHookStderr checks if stmt is logsink.HookStderr()
func isLogsinkHookStderr(stmt dst.Stmt) bool {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return false
	}

	// Check if it's logsink.HookStderr()
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	return ident.Name == "logsink" && sel.Sel.Name == "HookStderr"
}
