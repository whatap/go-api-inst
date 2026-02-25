// Package logrus provides the logrus package transformer.
package logrus

import (
	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

const whataplogrusPath = "github.com/whatap/go-api/instrumentation/github.com/sirupsen/logrus/whataplogrus"

// Transformer implements ast.Transformer for logrus package.
// Uses blank import to automatically register WhaTap Hook via init().
// Also wraps logrus.New() instances with whataplogrus.WrapLogger() to ensure
// Hook is registered on new logger instances (not just the global one).
type Transformer struct {
	transformed    bool // tracks if any transformation was made
	hasNewInstance bool // tracks if logrus.New() wrapping occurred
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "logrus"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/sirupsen/logrus"
}

// WhatapImport returns empty string because logrus uses blank import
// which is handled directly in Inject() via AddImportWithAlias.
func (t *Transformer) WhatapImport() string {
	return "" // Blank import is added directly in Inject()
}

// Detect checks if the file uses logrus package.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// Inject adds blank import for whataplogrus which auto-registers Hook via init().
// Also wraps logrus.New() calls with whataplogrus.WrapLogger() for new instances.
// Uses dst.Inspect on CallExpr to detect calls in any context (struct fields, etc).
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false
	t.hasNewInstance = false

	// Get the actual package name used in code (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

	// Wrap logrus.New() calls with whataplogrus.WrapLogger() via dst.Inspect on CallExpr
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
			if isWrapLoggerCall(call) {
				return false
			}

			// Check if it's pkgName.New()
			callPkg, callFunc := getCallPkgAndFunc(call)
			if callPkg == pkgName && callFunc == "New" {
				wrapCallExpr(call, "whataplogrus", "WrapLogger")
				t.hasNewInstance = true
				t.transformed = true
				return false
			}

			return true
		})
	}

	// Add whataplogrus import
	if !common.HasImport(file, whataplogrusPath) {
		if t.hasNewInstance {
			// Regular import for WrapLogger() usage
			common.AddImport(file, `"`+whataplogrusPath+`"`)
		} else {
			// Blank import for global Hook only
			common.AddImportWithAlias(file, whataplogrusPath, "_")
		}
		t.transformed = true
	} else if t.hasNewInstance {
		// Already imported - check if it's blank import and needs upgrade to regular
		if common.HasImportWithAlias(file, whataplogrusPath, "_") {
			common.RemoveImport(file, whataplogrusPath)
			common.AddImport(file, `"`+whataplogrusPath+`"`)
		}
		t.transformed = true
	}

	return t.transformed, nil
}

// Remove removes the whataplogrus import and unwraps WrapLogger() calls.
func (t *Transformer) Remove(file *dst.File) error {
	// Phase 1: Unwrap whataplogrus.WrapLogger(expr) → expr via dst.Inspect on CallExpr
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

			if isWrapLoggerCall(call) && len(call.Args) == 1 {
				unwrapCallExpr(call)
				return false
			}

			return true
		})
	}

	// Phase 2: Also unwrap in BlockStmt-based patterns (backward compatibility)
	dst.Inspect(file, func(n dst.Node) bool {
		block, ok := n.(*dst.BlockStmt)
		if !ok {
			return true
		}

		t.unwrapNewInstances(block)
		return true
	})

	// Remove whataplogrus import (both blank and regular)
	common.RemoveImport(file, whataplogrusPath)

	// Also remove legacy SetOutput pattern if exists (for backwards compatibility)
	t.removeLegacySetOutput(file)

	return nil
}

// unwrapNewInstances unwraps whataplogrus.WrapLogger(logrus.New()) → logrus.New()
// This handles old-style injection where wrapping was done at AssignStmt level.
func (t *Transformer) unwrapNewInstances(block *dst.BlockStmt) {
	for _, stmt := range block.List {
		assign, ok := stmt.(*dst.AssignStmt)
		if !ok || len(assign.Rhs) == 0 {
			continue
		}

		for i, rhs := range assign.Rhs {
			call, ok := rhs.(*dst.CallExpr)
			if !ok {
				continue
			}

			// Check if it's whataplogrus.WrapLogger(...)
			if isWrapLoggerCall(call) && len(call.Args) == 1 {
				// Unwrap: whataplogrus.WrapLogger(logrus.New()) → logrus.New()
				assign.Rhs[i] = call.Args[0]
			}
		}
	}
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

// isWrapLoggerCall checks if call is whataplogrus.WrapLogger(...)
func isWrapLoggerCall(call *dst.CallExpr) bool {
	pkg, fn := getCallPkgAndFunc(call)
	return pkg == "whataplogrus" && fn == "WrapLogger"
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

// removeLegacySetOutput removes old SetOutput(logsink.GetTraceLogWriter(...)) pattern
func (t *Transformer) removeLegacySetOutput(file *dst.File) {
	mainFn := common.FindMainFunc(file)
	if mainFn == nil {
		return
	}

	// Get the actual package name (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		pkgName = "logrus" // fallback
	}

	// Remove SetOutput(logsink.GetTraceLogWriter(...)) statement
	common.RemoveStmt(mainFn, func(stmt dst.Stmt) bool {
		return isSetOutputWithLogsink(stmt, pkgName)
	})

	// Remove os import if no longer used
	common.RemoveImportIfUnused(file, "os", "os")

	// Remove logsink import if no longer used
	common.RemoveImportIfUnused(file, "github.com/whatap/go-api/logsink", "logsink")
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

	return argIdent.Name == "logsink" && argSel.Sel.Name == "GetTraceLogWriter"
}
