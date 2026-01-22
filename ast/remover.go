package ast

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"go-api-inst/ast/common"
	"go-api-inst/report"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// Remover removes monitoring code from source files
type Remover struct {
	RemoveAll bool     // --all mode: also remove manually inserted patterns
	Warnings  []string // warnings for unremovable patterns
}

// NewRemover creates a new remover
func NewRemover(removeAll bool) *Remover {
	return &Remover{
		RemoveAll: removeAll,
		Warnings:  []string{},
	}
}

// RemoveFile removes monitoring code from a single file
func (r *Remover) RemoveFile(srcPath, dstPath string) error {
	src, err := os.ReadFile(srcPath)
	if err != nil {
		report.Get().AddFile(report.FileReport{
			Path:   srcPath,
			Status: report.StatusError,
			Error:  fmt.Sprintf("read file: %v", err),
		})
		return err
	}

	// Copy empty files as-is
	if len(src) == 0 {
		report.Get().AddFile(report.FileReport{
			Path:   srcPath,
			Status: report.StatusSkipped,
			Reason: "empty file",
		})
		return r.copyFile(srcPath, dstPath)
	}

	// Parse with dst (auto-preserves comments)
	// CGO files (import "C") are parsed normally - dst handles them
	file, err := decorator.Parse(src)
	if err != nil {
		// Copy as-is on parse failure (syntax error files, etc.)
		report.Get().AddFile(report.FileReport{
			Path:   srcPath,
			Status: report.StatusError,
			Error:  fmt.Sprintf("parse error: %v", err),
			Reason: "copied as-is",
		})
		return r.copyFile(srcPath, dstPath)
	}

	// Copy as-is if no whatap import
	if !common.HasWhatapImport(file) {
		report.Get().AddFile(report.FileReport{
			Path:   srcPath,
			Status: report.StatusSkipped,
			Reason: "not instrumented",
		})
		return r.copyFile(srcPath, dstPath)
	}

	// For recording changes
	var changes []string

	// Remove whatap-related imports
	r.removeWhatapImports(file)
	changes = append(changes, "removed: whatap imports")

	// Remove trace.Init/Shutdown from main() function
	r.removeMainInit(file)
	changes = append(changes, "removed: trace.Init/Shutdown")

	// Phase 6: Use transformer registry
	// Call transformer.Remove() for detected packages
	transformers := common.GetDetectedTransformers(file)
	for _, t := range transformers {
		if err := t.Remove(file); err != nil {
			report.Get().AddFile(report.FileReport{
				Path:   srcPath,
				Status: report.StatusError,
				Error:  fmt.Sprintf("remove %s: %v", t.Name(), err),
			})
			return fmt.Errorf("remove %s: %w", t.Name(), err)
		}
		changes = append(changes, fmt.Sprintf("removed: %s instrumentation", t.Name()))
	}

	// Remove error tracing code (trace.Error(err))
	r.removeErrorTracing(file)

	// --all mode: remove manually inserted patterns
	if r.RemoveAll {
		r.removeManualPatterns(file, srcPath)
		changes = append(changes, "removed: manual instrumentation patterns")
	}

	// Remove unused context import
	r.removeUnusedContextImport(file)

	// Record in report
	report.Get().AddFile(report.FileReport{
		Path:    srcPath,
		Status:  report.StatusRemoved,
		Changes: changes,
	})

	// Generate result file
	return r.writeFile(file, dstPath)
}

// RemoveDir removes monitoring code from all Go files in a directory
func (r *Remover) RemoveDir(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Directories to skip: vendor, .git, node_modules, whatap-instrumented
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == ".git" || base == "node_modules" || base == "whatap-instrumented" {
				return filepath.SkipDir
			}
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// Process only .go files
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			return r.RemoveFile(path, dstPath)
		}

		// Copy other files
		report.Get().AddFile(report.FileReport{
			Path:   path,
			Status: report.StatusCopied,
			Reason: "non-go file",
		})
		return r.copyFile(path, dstPath)
	})
}

// removeWhatapImports removes whatap-related imports
func (r *Remover) removeWhatapImports(file *dst.File) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}

		var newSpecs []dst.Spec
		for _, spec := range genDecl.Specs {
			imp, ok := spec.(*dst.ImportSpec)
			if !ok {
				newSpecs = append(newSpecs, spec)
				continue
			}

			path := strings.Trim(imp.Path.Value, `"`)
			if !strings.Contains(path, "whatap/go-api") {
				newSpecs = append(newSpecs, spec)
			}
		}
		genDecl.Specs = newSpecs
	}

	// Also update file.Imports
	var newImports []*dst.ImportSpec
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if !strings.Contains(path, "whatap/go-api") {
			newImports = append(newImports, imp)
		}
	}
	file.Imports = newImports
}

// removeMainInit removes trace.Init/Shutdown from main()
func (r *Remover) removeMainInit(file *dst.File) {
	dst.Inspect(file, func(n dst.Node) bool {
		fn, ok := n.(*dst.FuncDecl)
		if !ok || fn.Name.Name != "main" || fn.Recv != nil {
			return true
		}

		if fn.Body == nil {
			return true
		}

		var newList []dst.Stmt
		for _, stmt := range fn.Body.List {
			if r.isTraceInitCall(stmt) || r.isTraceShutdownDefer(stmt) {
				continue
			}
			newList = append(newList, stmt)
		}
		fn.Body.List = newList

		return false
	})
}

// isTraceInitCall checks if statement is trace.Init() call
func (r *Remover) isTraceInitCall(stmt dst.Stmt) bool {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	return ident.Name == "trace" && sel.Sel.Name == "Init"
}

// isTraceShutdownDefer checks if statement is defer trace.Shutdown()
func (r *Remover) isTraceShutdownDefer(stmt dst.Stmt) bool {
	deferStmt, ok := stmt.(*dst.DeferStmt)
	if !ok {
		return false
	}

	sel, ok := deferStmt.Call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	return ident.Name == "trace" && sel.Sel.Name == "Shutdown"
}

// writeFile writes the transformed file to disk
func (r *Remover) writeFile(file *dst.File, dstPath string) error {
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return decorator.Fprint(f, file)
}

// copyFile copies a file
func (r *Remover) copyFile(src, dstPath string) error {
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, 0644)
}

// removeErrorTracing removes error tracing code from file
func (r *Remover) removeErrorTracing(file *dst.File) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		// Traverse all statements in function body and remove trace.Error
		fn.Body.List = r.removeErrorTracingFromStmts(fn.Body.List)
	}
}

// removeErrorTracingFromStmts removes trace.Error from statement list
func (r *Remover) removeErrorTracingFromStmts(stmts []dst.Stmt) []dst.Stmt {
	var newStmts []dst.Stmt

	for i, stmt := range stmts {
		// Remove trace.Error after if err == nil { return }
		if i > 0 {
			if ifStmt, ok := stmts[i-1].(*dst.IfStmt); ok {
				if r.isErrEqualNilWithReturn(ifStmt) && r.isTraceErrorCallStmt(stmt) {
					continue // Remove
				}
			}
		}

		newStmts = append(newStmts, stmt)
		r.removeErrorTracingFromStmt(stmt)
	}

	return newStmts
}

// removeErrorTracingFromStmt removes trace.Error from a single statement
func (r *Remover) removeErrorTracingFromStmt(stmt dst.Stmt) {
	switch s := stmt.(type) {
	case *dst.IfStmt:
		// Remove trace.Error from if err != nil block
		r.removeTraceErrorFromBlock(s.Body)

		// Also remove from else block in if err == nil { } else { return } pattern
		if s.Else != nil {
			if elseBlock, ok := s.Else.(*dst.BlockStmt); ok {
				if r.isErrEqualNilCond(s.Cond) {
					r.removeTraceErrorFromBlock(elseBlock)
				}
				elseBlock.List = r.removeErrorTracingFromStmts(elseBlock.List)
			} else if elseIf, ok := s.Else.(*dst.IfStmt); ok {
				r.removeErrorTracingFromStmt(elseIf)
			}
		}

		// Also process nested structure inside if block
		s.Body.List = r.removeErrorTracingFromStmts(s.Body.List)

	case *dst.BlockStmt:
		s.List = r.removeErrorTracingFromStmts(s.List)

	case *dst.ForStmt:
		if s.Body != nil {
			s.Body.List = r.removeErrorTracingFromStmts(s.Body.List)
		}

	case *dst.RangeStmt:
		if s.Body != nil {
			s.Body.List = r.removeErrorTracingFromStmts(s.Body.List)
		}

	case *dst.SwitchStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CaseClause); ok {
					cc.Body = r.removeErrorTracingFromStmts(cc.Body)
				}
			}
		}

	case *dst.SelectStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CommClause); ok {
					cc.Body = r.removeErrorTracingFromStmts(cc.Body)
				}
			}
		}
	}
}

// removeTraceErrorFromBlock removes trace.Error(err) calls from block
func (r *Remover) removeTraceErrorFromBlock(block *dst.BlockStmt) {
	var newList []dst.Stmt

	for _, stmt := range block.List {
		// Check if it's trace.Error(err) call
		if r.isTraceErrorCallStmt(stmt) {
			continue // Remove
		}
		newList = append(newList, stmt)
	}

	block.List = newList
}

// isTraceErrorCallStmt checks if statement is trace.Error(...) call
func (r *Remover) isTraceErrorCallStmt(stmt dst.Stmt) bool {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	return ident.Name == "trace" && sel.Sel.Name == "Error"
}

// isErrEqualNilCond checks if condition is err == nil
func (r *Remover) isErrEqualNilCond(cond dst.Expr) bool {
	binExpr, ok := cond.(*dst.BinaryExpr)
	if !ok || binExpr.Op != token.EQL {
		return false
	}

	// Check if right side is nil
	yIdent, ok := binExpr.Y.(*dst.Ident)
	if !ok || yIdent.Name != "nil" {
		return false
	}

	// Check if left side is err variable
	xIdent, ok := binExpr.X.(*dst.Ident)
	if !ok {
		return false
	}

	return xIdent.Name == "err" || xIdent.Name == "e" || xIdent.Name == "error"
}

// isErrEqualNilWithReturn checks if it's if err == nil { return } pattern
func (r *Remover) isErrEqualNilWithReturn(ifStmt *dst.IfStmt) bool {
	if !r.isErrEqualNilCond(ifStmt.Cond) {
		return false
	}

	// Check if there's a return in if block
	for _, stmt := range ifStmt.Body.List {
		if _, ok := stmt.(*dst.ReturnStmt); ok {
			return true
		}
	}
	return false
}

// removeUnusedContextImport removes context import if not used in code
func (r *Remover) removeUnusedContextImport(file *dst.File) {
	// Check if context package is used in code
	contextUsed := false

	// Find context.XXX selectors in entire file
	dst.Inspect(file, func(n dst.Node) bool {
		sel, ok := n.(*dst.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			return true
		}

		if ident.Name == "context" {
			contextUsed = true
			return false
		}
		return true
	})

	if contextUsed {
		return
	}

	// Remove import if context is not used
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}

		var newSpecs []dst.Spec
		for _, spec := range genDecl.Specs {
			imp, ok := spec.(*dst.ImportSpec)
			if !ok {
				newSpecs = append(newSpecs, spec)
				continue
			}

			path := strings.Trim(imp.Path.Value, `"`)
			if path != "context" {
				newSpecs = append(newSpecs, spec)
			}
		}
		genDecl.Specs = newSpecs
	}

	// Also update file.Imports
	var newImports []*dst.ImportSpec
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path != "context" {
			newImports = append(newImports, imp)
		}
	}
	file.Imports = newImports
}

// removeManualPatterns removes manually inserted patterns (--all mode)
func (r *Remover) removeManualPatterns(file *dst.File, srcPath string) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		// Remove manual insertion patterns from function body
		fn.Body.List = r.removeManualPatternsFromStmts(fn.Body.List, srcPath)
	}

	// Warn about struct field assignment patterns in entire file
	r.checkStructFieldPatterns(file, srcPath)
}

// checkStructFieldPatterns checks if struct fields contain whatap calls
func (r *Remover) checkStructFieldPatterns(file *dst.File, srcPath string) {
	dst.Inspect(file, func(n dst.Node) bool {
		// Check composite literal (struct literal)
		compLit, ok := n.(*dst.CompositeLit)
		if !ok {
			return true
		}

		for _, elt := range compLit.Elts {
			// key: value form
			kv, ok := elt.(*dst.KeyValueExpr)
			if !ok {
				continue
			}

			// Check if value is whatap call
			if r.isWhatapRelatedExpr(kv.Value) {
				keyName := "unknown"
				if ident, ok := kv.Key.(*dst.Ident); ok {
					keyName = ident.Name
				}
				r.addWarning(srcPath, fmt.Sprintf("struct field assignment: %s: %s (manual removal required)", keyName, r.exprToString(kv.Value)))
			}
		}

		return true
	})
}

// removeManualPatternsFromStmts removes manual insertion patterns from statement list
func (r *Remover) removeManualPatternsFromStmts(stmts []dst.Stmt, srcPath string) []dst.Stmt {
	var newStmts []dst.Stmt

	for _, stmt := range stmts {
		// Check if it's a removable standalone statement
		if r.isRemovableManualStatement(stmt) {
			continue // Remove
		}

		// Warn about unremovable patterns
		if warning := r.checkUnremovablePattern(stmt); warning != "" {
			r.addWarning(srcPath, warning)
		}

		// Process nested structures
		r.processNestedStmts(stmt, srcPath)

		newStmts = append(newStmts, stmt)
	}

	return newStmts
}

// isRemovableManualStatement checks if it's a removable manual insertion statement
func (r *Remover) isRemovableManualStatement(stmt dst.Stmt) bool {
	// 1. Standalone expression statement (ExprStmt)
	if exprStmt, ok := stmt.(*dst.ExprStmt); ok {
		return r.isRemovableExpr(exprStmt.X)
	}

	// 2. defer statement
	if deferStmt, ok := stmt.(*dst.DeferStmt); ok {
		return r.isRemovableDeferCall(deferStmt.Call)
	}

	return false
}

// isRemovableExpr checks if expression is removable
func (r *Remover) isRemovableExpr(expr dst.Expr) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}

	// Check package.function() or receiver.Method() form
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	pkg := ident.Name
	fn := sel.Sel.Name

	// If not a known whatap package, treat as method call (e.g., rdb.AddHook)
	if !r.isKnownWhatapPackage(pkg) {
		return r.isRemovableMethodCall(call)
	}

	// Removable standalone statement patterns (side-effect free calls only)
	switch pkg {
	case "trace":
		switch fn {
		case "Step", "Println", "SetMTrace", "Error":
			return true
		}
	case "logsink":
		// logsink related calls
		return true
	}

	// Cannot remove: Wrap/Trace patterns containing closures
	// Business logic is inside the closure, so don't delete recklessly
	if r.isClosurePattern(pkg, fn) {
		return false
	}

	// Cannot remove: Patterns that use return values (NewXXX, Open, etc.)
	if r.isFactoryPattern(pkg, fn) {
		return false
	}

	return false
}

// isKnownWhatapPackage checks if it's a known whatap package
func (r *Remover) isKnownWhatapPackage(name string) bool {
	switch name {
	case "trace", "logsink", "httpc", "method":
		return true
	}
	// whatap* prefix
	if strings.HasPrefix(name, "whatap") {
		return true
	}
	return false
}

// isClosurePattern checks if it's a pattern containing closures
func (r *Remover) isClosurePattern(pkg, fn string) bool {
	switch pkg {
	case "whatapsql":
		return fn == "Wrap" || fn == "WrapWithParam"
	case "httpc":
		return fn == "Trace" || fn == "Wrap"
	case "method":
		return fn == "Trace" || fn == "Wrap"
	}
	return false
}

// isFactoryPattern checks if it's a factory pattern (uses return value)
func (r *Remover) isFactoryPattern(pkg, fn string) bool {
	// NewXXX, OpenXXX, etc. use return values so don't appear as standalone statements
	// But don't remove if called standalone
	if strings.HasPrefix(fn, "New") || strings.HasPrefix(fn, "Open") {
		return true
	}
	return false
}

// isRemovableDeferCall checks if defer call is removable
func (r *Remover) isRemovableDeferCall(call *dst.CallExpr) bool {
	// defer trace.End(...), defer trace.Shutdown(), etc.
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		// defer func() { ... }() form - need to check contents
		if funcLit, ok := call.Fun.(*dst.FuncLit); ok {
			return r.isRemovableFuncLit(funcLit)
		}
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	pkg := ident.Name
	fn := sel.Sel.Name

	switch pkg {
	case "trace":
		switch fn {
		case "End", "Shutdown":
			return true
		}
	case "whatapsql", "httpc", "method":
		if fn == "End" {
			return true
		}
	}

	// whatap* package calls
	if strings.HasPrefix(pkg, "whatap") {
		return true
	}

	return false
}

// isRemovableFuncLit checks defer func() { trace.End(...) }() form
func (r *Remover) isRemovableFuncLit(fn *dst.FuncLit) bool {
	if fn.Body == nil || len(fn.Body.List) == 0 {
		return false
	}

	// Removable if function only contains whatap-related calls
	for _, stmt := range fn.Body.List {
		if !r.isRemovableManualStatement(stmt) {
			return false
		}
	}
	return true
}

// isRemovableMethodCall checks receiver.Method() form (e.g., rdb.AddHook(...))
func (r *Remover) isRemovableMethodCall(call *dst.CallExpr) bool {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	methodName := sel.Sel.Name

	// AddHook(whatapgoredis.NewHook(...)) pattern
	if methodName == "AddHook" && len(call.Args) > 0 {
		if r.isWhatapCall(call.Args[0]) {
			return true
		}
	}

	return false
}

// isWhatapCall checks if it's a whatap* package call
func (r *Remover) isWhatapCall(expr dst.Expr) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	return strings.HasPrefix(ident.Name, "whatap")
}

// checkUnremovablePattern checks unremovable patterns and returns warning message
func (r *Remover) checkUnremovablePattern(stmt dst.Stmt) string {
	// 1. Variable assignment (=) + whatap call
	if assignStmt, ok := stmt.(*dst.AssignStmt); ok {
		for _, rhs := range assignStmt.Rhs {
			if r.isWhatapRelatedExpr(rhs) {
				if assignStmt.Tok.String() == ":=" {
					return fmt.Sprintf("variable declaration: %s (manual removal required)", r.exprToString(rhs))
				}
				return fmt.Sprintf("variable assignment: %s (manual removal required)", r.exprToString(rhs))
			}
		}
	}

	// 2. Standalone expression statement - check whatap-related calls
	if exprStmt, ok := stmt.(*dst.ExprStmt); ok {
		if call, ok := exprStmt.X.(*dst.CallExpr); ok {
			// Closure patterns (Wrap, Trace)
			if warning := r.checkClosurePattern(call); warning != "" {
				return warning
			}
			// Other whatap-related calls (if not removed)
			if warning := r.checkWhatapCall(call); warning != "" {
				return warning
			}
		}
	}

	return ""
}

// checkWhatapCall checks if it's a whatap-related call and returns warning message
func (r *Remover) checkWhatapCall(call *dst.CallExpr) string {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return ""
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return ""
	}

	pkg := ident.Name
	fn := sel.Sel.Name

	// Check if it's a known whatap package call
	if r.isKnownWhatapPackage(pkg) {
		return fmt.Sprintf("whatap call: %s.%s(...) (manual removal required)", pkg, fn)
	}

	return ""
}

// checkClosurePattern checks closure patterns
func (r *Remover) checkClosurePattern(call *dst.CallExpr) string {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return ""
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return ""
	}

	pkg := ident.Name
	fn := sel.Sel.Name

	// Patterns containing closures
	if r.isClosurePattern(pkg, fn) {
		return fmt.Sprintf("closure pattern: %s.%s(...) (contains business logic, manual removal required)", pkg, fn)
	}

	return ""
}

// isWhatapRelatedExpr checks if expression is whatap-related
func (r *Remover) isWhatapRelatedExpr(expr dst.Expr) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	pkg := ident.Name
	fn := sel.Sel.Name

	// trace package
	if pkg == "trace" {
		switch fn {
		case "Start", "StartMethod", "GetMTrace":
			return true
		}
	}

	// whatapsql package
	if pkg == "whatapsql" {
		switch fn {
		case "Start", "StartWithParam", "StartOpen", "Open", "OpenContext", "OpenDB",
			"Wrap", "WrapWithParam":
			return true
		}
	}

	// httpc package
	if pkg == "httpc" {
		switch fn {
		case "Start", "Trace", "Wrap":
			return true
		}
	}

	// method package
	if pkg == "method" {
		switch fn {
		case "Start", "Trace", "Wrap":
			return true
		}
	}

	// whatap* prefix packages (whataphttp, whatapgoredis, whatapredigo, whatapmongo, etc.)
	if strings.HasPrefix(pkg, "whatap") {
		return true
	}

	return false
}

// exprToString converts expression to string (for warning messages)
func (r *Remover) exprToString(expr dst.Expr) string {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return "unknown"
	}

	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return "unknown"
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return "unknown"
	}

	return fmt.Sprintf("%s.%s(...)", ident.Name, sel.Sel.Name)
}

// processNestedStmts processes nested structures
func (r *Remover) processNestedStmts(stmt dst.Stmt, srcPath string) {
	switch s := stmt.(type) {
	case *dst.IfStmt:
		s.Body.List = r.removeManualPatternsFromStmts(s.Body.List, srcPath)
		if s.Else != nil {
			if elseBlock, ok := s.Else.(*dst.BlockStmt); ok {
				elseBlock.List = r.removeManualPatternsFromStmts(elseBlock.List, srcPath)
			} else if elseIf, ok := s.Else.(*dst.IfStmt); ok {
				r.processNestedStmts(elseIf, srcPath)
			}
		}
	case *dst.BlockStmt:
		s.List = r.removeManualPatternsFromStmts(s.List, srcPath)
	case *dst.ForStmt:
		if s.Body != nil {
			s.Body.List = r.removeManualPatternsFromStmts(s.Body.List, srcPath)
		}
	case *dst.RangeStmt:
		if s.Body != nil {
			s.Body.List = r.removeManualPatternsFromStmts(s.Body.List, srcPath)
		}
	case *dst.SwitchStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CaseClause); ok {
					cc.Body = r.removeManualPatternsFromStmts(cc.Body, srcPath)
				}
			}
		}
	case *dst.SelectStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CommClause); ok {
					cc.Body = r.removeManualPatternsFromStmts(cc.Body, srcPath)
				}
			}
		}
	}
}

// addWarning adds a warning
func (r *Remover) addWarning(srcPath, warning string) {
	msg := fmt.Sprintf("%s: %s", srcPath, warning)
	r.Warnings = append(r.Warnings, msg)
	fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
}

// GetWarnings returns the list of warnings
func (r *Remover) GetWarnings() []string {
	return r.Warnings
}
