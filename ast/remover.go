package ast

import (
	"fmt"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/whatap/go-api-inst/ast/common"
	"github.com/whatap/go-api-inst/report"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// Remover removes monitoring code from source files using the v2 engine.
type Remover struct {
	RemoveAll bool     // --all mode: also remove manually inserted patterns
	Warnings  []string // warnings for unremovable patterns
	registry  *Registry

	// §246 / §272 — ReplaceFunction reverse map (lazy-built).
	// Key: "whatapAlias.WhatapFunc" (e.g. "whatapsql.Open").
	// Value: list of candidates — multiple Rules may share the same alias
	// (e.g. go-redis v8 and v9 both use `whatapgoredis`). The file's actual
	// whatap import path disambiguates at apply time.
	replaceFnReverseMap map[string][]replaceFnReverse
}

// replaceFnReverse describes how to revert a single whatap*.X(...) call
// back to its original pkg.X(...) form. Built once from LoadBuiltinRules()
// for every ReplaceFunction Rule. §246 / §272 Phase 2 follow-up.
type replaceFnReverse struct {
	whatapImportPath string // e.g. "github.com/whatap/.../whatapgoredis" — disambiguates v8/v9
	origImportPath   string // e.g. "database/sql"
	origAlias        string // e.g. "sql" (path.Base + version-suffix handling)
	origFunc         string // e.g. "Open"
}

// NewRemover creates a new v2 remover.
func NewRemover(removeAll bool) *Remover {
	rem := &Remover{
		RemoveAll: removeAll,
		Warnings:  []string{},
	}
	rem.registry = NewRegistry()
	for _, r := range LoadBuiltinRules() {
		if r != nil {
			rem.registry.Register(r)
		}
	}
	return rem
}

// RemoveFile removes monitoring code from a single file using the v2 engine.
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

	file, err := decorator.Parse(src)
	if err != nil {
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

	var changes []string

	// Remove trace.Init/Shutdown from main()
	r.removeMainInit(file)
	changes = append(changes, "removed: trace.Init/Shutdown")

	// §272 Phase 3 — 자동 inject 역연산 엔진 호출 비활성화.
	// remove 의 use case 는 사용자 수동 코드 청소 (manual → auto 마이그레이션)
	// 이고 자동 inject 결과는 $WORK 만 영향 → 역연산할 대상이 처음부터 없음.
	// advice.go 의 ModeRemove 분기 / registry 의 whatapRules 등록 / Transform
	// ReverseTarget 매칭 등은 모두 폐기 (Phase 3 Step 1~4).

	// Remove error tracing code
	r.removeErrorTracing(file)

	// §272 — Manual pattern removal is the default. Handles standalone
	// statements, defer pairs, wrapper unwrap (whatap*.WrapXxx(x) → x),
	// closure FuncLit bodies, etc.
	r.removeManualPatterns(file, srcPath)
	changes = append(changes, "removed: manual instrumentation patterns")

	// §246 / §272 — ReplaceFunction reverse 패스 (단순 함수명 교체).
	// removeManualPatterns 가 wrapper 형태 (whatapsql.Open(sql.Open(...))
	// 처럼 인자 1개) 를 먼저 unwrap → 남은 `whatap*.X(arg1, arg2, ...)`
	// 호출 (인자 2개+ 또는 wrap 화이트리스트 외) 만 여기서 함수명 교체 +
	// 원본 import 추가. whatap import 가 살아있는 시점에 동작해야 alias
	// 매칭이 명확함. Rule 메타데이터 1회 읽기, §272 Phase 3 에서 제거된
	// Registry-level reverse 인프라와는 분리.
	r.reverseReplaceFunctionCalls(file)

	// Remove whatap-related imports
	common.RemoveWhatapImports(file)
	changes = append(changes, "removed: whatap imports")

	// Remove unused context import
	r.removeUnusedContextImport(file)

	report.Get().AddFile(report.FileReport{
		Path:    srcPath,
		Status:  report.StatusRemoved,
		Changes: changes,
	})

	return r.writeFile(file, dstPath)
}

// RemoveDir removes monitoring code from all Go files in a directory
func (r *Remover) RemoveDir(srcDir, dstDir string) error {
	srcDirAbs, _ := filepath.Abs(srcDir)

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			base := filepath.Base(path)
			pathAbs, _ := filepath.Abs(path)
			isSourceDir := pathAbs == srcDirAbs

			if !isSourceDir && (base == "vendor" || base == ".git" || base == "node_modules" || base == "whatap-instrumented") {
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

		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			return r.RemoveFile(path, dstPath)
		}

		report.Get().AddFile(report.FileReport{
			Path:   path,
			Status: report.StatusCopied,
			Reason: "non-go file",
		})
		return r.copyFile(path, dstPath)
	})
}

// GetWarnings returns the list of warnings
func (r *Remover) GetWarnings() []string {
	return r.Warnings
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
	return (common.MatchIdentPkg(ident, "trace", "github.com/whatap/go-api/trace") ||
		common.MatchIdentPkg(ident, "whataptrace", "github.com/whatap/go-api/trace")) && sel.Sel.Name == "Init"
}

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
	return (common.MatchIdentPkg(ident, "trace", "github.com/whatap/go-api/trace") ||
		common.MatchIdentPkg(ident, "whataptrace", "github.com/whatap/go-api/trace")) && sel.Sel.Name == "Shutdown"
}

func (r *Remover) writeFile(file *dst.File, dstPath string) error {
	return common.WriteDstFile(file, dstPath)
}

func (r *Remover) copyFile(src, dstPath string) error {
	return common.CopyFile(src, dstPath)
}

// removeErrorTracing removes error tracing code from file
func (r *Remover) removeErrorTracing(file *dst.File) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		fn.Body.List = r.removeErrorTracingFromStmts(fn.Body.List)
	}
}

func (r *Remover) removeErrorTracingFromStmts(stmts []dst.Stmt) []dst.Stmt {
	var newStmts []dst.Stmt
	for i, stmt := range stmts {
		if i > 0 {
			if ifStmt, ok := stmts[i-1].(*dst.IfStmt); ok {
				if r.isErrEqualNilWithReturn(ifStmt) && r.isTraceErrorCallStmt(stmt) {
					continue
				}
			}
		}
		newStmts = append(newStmts, stmt)
		r.removeErrorTracingFromStmt(stmt)
	}
	return newStmts
}

func (r *Remover) removeErrorTracingFromStmt(stmt dst.Stmt) {
	switch s := stmt.(type) {
	case *dst.IfStmt:
		r.removeTraceErrorFromBlock(s.Body)
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

func (r *Remover) removeTraceErrorFromBlock(block *dst.BlockStmt) {
	var newList []dst.Stmt
	for _, stmt := range block.List {
		if r.isTraceErrorCallStmt(stmt) {
			continue
		}
		newList = append(newList, stmt)
	}
	block.List = newList
}

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
	return (common.MatchIdentPkg(ident, "trace", "github.com/whatap/go-api/trace") ||
		common.MatchIdentPkg(ident, "whataptrace", "github.com/whatap/go-api/trace")) && sel.Sel.Name == "Error"
}

func (r *Remover) isErrEqualNilCond(cond dst.Expr) bool {
	binExpr, ok := cond.(*dst.BinaryExpr)
	if !ok || binExpr.Op != token.EQL {
		return false
	}
	yIdent, ok := binExpr.Y.(*dst.Ident)
	if !ok || yIdent.Name != "nil" {
		return false
	}
	xIdent, ok := binExpr.X.(*dst.Ident)
	if !ok {
		return false
	}
	return xIdent.Name == "err" || xIdent.Name == "e" || xIdent.Name == "error"
}

func (r *Remover) isErrEqualNilWithReturn(ifStmt *dst.IfStmt) bool {
	if !r.isErrEqualNilCond(ifStmt.Cond) {
		return false
	}
	for _, stmt := range ifStmt.Body.List {
		if _, ok := stmt.(*dst.ReturnStmt); ok {
			return true
		}
	}
	return false
}

func (r *Remover) removeUnusedContextImport(file *dst.File) {
	contextUsed := false
	dst.Inspect(file, func(n dst.Node) bool {
		sel, ok := n.(*dst.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			return true
		}
		if common.MatchIdentPkg(ident, "context", "context") {
			contextUsed = true
			return false
		}
		return true
	})

	if contextUsed {
		return
	}

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
		fn.Body.List = r.removeManualPatternsFromStmts(fn.Body.List, srcPath)
	}
	r.checkStructFieldPatterns(file, srcPath)
}

func (r *Remover) removeManualPatternsFromStmts(stmts []dst.Stmt, srcPath string) []dst.Stmt {
	var newStmts []dst.Stmt
	for _, stmt := range stmts {
		if r.isRemovableManualStatement(stmt) {
			continue
		}
		// §272 Phase 2 Step 2 — wrapper unwrap (변수 대입 RHS 의 화이트리스트 wrapper).
		// `db, _ := whatapsql.Open(sql.Open(...))` → `db, _ := sql.Open(...)` 같이
		// 안쪽 인자만 남김. unwrap 한 경우 warning 안 띄움.
		unwrapped := r.tryUnwrapAssignStmt(stmt)
		if !unwrapped {
			if warning := r.checkUnremovablePattern(stmt); warning != "" {
				r.addWarning(srcPath, warning)
			}
		}
		r.processNestedStmts(stmt, srcPath)
		newStmts = append(newStmts, stmt)
	}
	return newStmts
}

// tryUnwrapAssignStmt — AssignStmt 의 RHS 에 화이트리스트 wrapper 호출이 있으면
// 안쪽 인자로 교체. unwrap 한 경우 true 반환. 그 외 false.
// §272 Phase 2 Step 2.
func (r *Remover) tryUnwrapAssignStmt(stmt dst.Stmt) bool {
	assign, ok := stmt.(*dst.AssignStmt)
	if !ok {
		return false
	}
	changed := false
	for i, rhs := range assign.Rhs {
		if newExpr, ok := r.tryUnwrapWhatapWrapper(rhs); ok {
			assign.Rhs[i] = newExpr
			changed = true
		}
	}
	return changed
}

// tryUnwrapWhatapWrapper — 화이트리스트 wrapper 호출(`whatap*.Wrap*(arg)` 등)이면
// 안쪽 첫 인자를 반환. 인자가 정확히 1개인 경우만 처리.
// §272 Phase 2 Step 2.
func (r *Remover) tryUnwrapWhatapWrapper(expr dst.Expr) (dst.Expr, bool) {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return nil, false
	}
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return nil, false
	}
	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return nil, false
	}
	if !r.isKnownWhatapPackage(ident.Name, ident) {
		return nil, false
	}
	if !r.isUnwrapWhitelist(ident.Name, sel.Sel.Name) {
		return nil, false
	}
	if len(call.Args) != 1 {
		return nil, false
	}
	return call.Args[0], true
}

// isUnwrapWhitelist — 안쪽 인자를 그대로 노출해도 안전한 whatap wrapper 함수.
// 사용자가 수동으로 적용한 패턴 — wrapper 만 벗기면 원본 코드 복원.
// §272 Phase 2 Step 2.
func (r *Remover) isUnwrapWhitelist(pkg, fn string) bool {
	// Wrap* 시작 함수 (whatapgin.WrapEngine, whatapecho.WrapEngine,
	// whataphttp.WrapHandler 등) — 일반 패턴
	if strings.HasPrefix(fn, "Wrap") {
		return true
	}
	// 명시적 화이트리스트
	switch pkg {
	case "whataphttp":
		return fn == "Func" || fn == "WrapHandler" || fn == "WrapHandlerFunc"
	case "whatapsql", "whatapdb":
		return fn == "Open" || fn == "OpenDB"
	case "whataplogsink":
		return fn == "GetTraceLogWriter"
	}
	return false
}

func (r *Remover) isRemovableManualStatement(stmt dst.Stmt) bool {
	if exprStmt, ok := stmt.(*dst.ExprStmt); ok {
		return r.isRemovableExpr(exprStmt.X)
	}
	if deferStmt, ok := stmt.(*dst.DeferStmt); ok {
		return r.isRemovableDeferCall(deferStmt.Call)
	}
	return false
}

func (r *Remover) isRemovableExpr(expr dst.Expr) bool {
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

	if !r.isKnownWhatapPackage(pkg, ident) {
		return r.isRemovableMethodCall(call)
	}

	switch pkg {
	case "trace", "whataptrace":
		switch fn {
		case "Step", "Println", "SetMTrace", "Error":
			return true
		}
	case "logsink", "whataplogsink":
		return true
	}

	if r.isClosurePattern(pkg, fn) {
		return false
	}
	if r.isFactoryPattern(pkg, fn) {
		return false
	}
	return false
}

func (r *Remover) isKnownWhatapPackage(name string, ident *dst.Ident) bool {
	if ident != nil && common.HasTypeInfo() {
		identPath := common.GetIdentPath(ident)
		if identPath != "" {
			return strings.HasPrefix(identPath, "github.com/whatap/")
		}
	}
	switch name {
	case "trace", "whataptrace", "logsink", "whataplogsink", "httpc", "method":
		return true
	}
	return strings.HasPrefix(name, "whatap")
}

func (r *Remover) isClosurePattern(pkg, fn string) bool {
	switch pkg {
	case "whatapsql", "whatapdb":
		return fn == "Wrap" || fn == "WrapWithParam"
	case "httpc":
		return fn == "Trace" || fn == "Wrap"
	case "method":
		return fn == "Trace" || fn == "Wrap"
	}
	return false
}

func (r *Remover) isFactoryPattern(pkg, fn string) bool {
	return strings.HasPrefix(fn, "New") || strings.HasPrefix(fn, "Open")
}

func (r *Remover) isRemovableDeferCall(call *dst.CallExpr) bool {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
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
	case "trace", "whataptrace":
		if fn == "End" || fn == "Shutdown" {
			return true
		}
	case "whatapsql", "whatapdb", "httpc", "method":
		if fn == "End" {
			return true
		}
	}

	if common.HasTypeInfo() {
		identPath := common.GetIdentPath(ident)
		if strings.HasPrefix(identPath, "github.com/whatap/") {
			return true
		}
	} else if strings.HasPrefix(pkg, "whatap") {
		return true
	}
	return false
}

func (r *Remover) isRemovableFuncLit(fn *dst.FuncLit) bool {
	if fn.Body == nil || len(fn.Body.List) == 0 {
		return false
	}
	for _, stmt := range fn.Body.List {
		if !r.isRemovableManualStatement(stmt) {
			return false
		}
	}
	return true
}

func (r *Remover) isRemovableMethodCall(call *dst.CallExpr) bool {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name == "AddHook" && len(call.Args) > 0 {
		if r.isWhatapCall(call.Args[0]) {
			return true
		}
	}
	return false
}

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
	if common.HasTypeInfo() {
		identPath := common.GetIdentPath(ident)
		return strings.HasPrefix(identPath, "github.com/whatap/")
	}
	return strings.HasPrefix(ident.Name, "whatap")
}

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
	if common.HasTypeInfo() {
		identPath := common.GetIdentPath(ident)
		return strings.HasPrefix(identPath, "github.com/whatap/")
	}
	return strings.HasPrefix(ident.Name, "whatap")
}

func (r *Remover) checkUnremovablePattern(stmt dst.Stmt) string {
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
	return ""
}

func (r *Remover) checkStructFieldPatterns(file *dst.File, srcPath string) {
	dst.Inspect(file, func(n dst.Node) bool {
		compLit, ok := n.(*dst.CompositeLit)
		if !ok {
			return true
		}
		for _, elt := range compLit.Elts {
			kv, ok := elt.(*dst.KeyValueExpr)
			if !ok {
				continue
			}
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
	// §272 Phase 2 follow-up — closures attached to common statement forms.
	// FuncLit bodies are otherwise invisible to the per-block recursion, so
	// manual whatap calls inside `engine.GET("/", func(c){...})`,
	// `go func(){...}()`, `defer func(){...}()`, or `h := func(){...}` would
	// survive removal without this branch.
	case *dst.ExprStmt:
		r.descendIntoFuncLits(s.X, srcPath)
	case *dst.DeferStmt:
		if s.Call != nil {
			r.descendIntoCallFuncLits(s.Call, srcPath)
		}
	case *dst.GoStmt:
		if s.Call != nil {
			r.descendIntoCallFuncLits(s.Call, srcPath)
		}
	case *dst.AssignStmt:
		for _, rhs := range s.Rhs {
			r.descendIntoFuncLits(rhs, srcPath)
		}
	}
}

// descendIntoFuncLits inspects an expression and, if it's a FuncLit (or a
// CallExpr containing FuncLit args), recurses into its Body for manual-pattern
// removal. §272 Phase 2 follow-up.
func (r *Remover) descendIntoFuncLits(expr dst.Expr, srcPath string) {
	switch e := expr.(type) {
	case *dst.FuncLit:
		if e.Body != nil {
			e.Body.List = r.removeManualPatternsFromStmts(e.Body.List, srcPath)
		}
	case *dst.CallExpr:
		r.descendIntoCallFuncLits(e, srcPath)
	}
}

// descendIntoCallFuncLits walks a CallExpr's Fun and Args, processing any
// FuncLit bodies in place. §272 Phase 2 follow-up.
func (r *Remover) descendIntoCallFuncLits(call *dst.CallExpr, srcPath string) {
	if fn, ok := call.Fun.(*dst.FuncLit); ok && fn.Body != nil {
		fn.Body.List = r.removeManualPatternsFromStmts(fn.Body.List, srcPath)
	}
	for _, arg := range call.Args {
		r.descendIntoFuncLits(arg, srcPath)
	}
}

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

func (r *Remover) addWarning(srcPath, warning string) {
	msg := fmt.Sprintf("%s: %s", srcPath, warning)
	r.Warnings = append(r.Warnings, msg)
	fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
}

// reverseReplaceFunctionCalls walks the file's AST and reverts any
// `whatapAlias.WhatapFunc(...)` calls that match a built-in ReplaceFunction
// Rule back to their original `origAlias.origFunc(...)` form. Required
// imports for the originals are added once at the end.
//
// This handles the **manual-application** scenario behind §246: a user who
// followed the SDK guide and hand-wrote `whatapsql.Open(...)`, `whatapfmt.
// Println(...)`, etc., now wants those gone. The §272 single-deletion engine
// alone would strip whatap imports but leave the call sites referencing a
// missing identifier.
//
// Scope: ReplaceFunction Rules only — 25 entries covering SQL/sqlx/gorm/
// redis(v8,v9)/redigo/mongo/fmt. Other Advice types (WrapCall/ArgWrap → §272
// unwrap whitelist; ReplaceWithCtx → arg-count change, manual; Transform/
// FieldWrap* → too structural, manual) intentionally do not participate.
//
// §246 / §272 Phase 2 follow-up.
func (r *Remover) reverseReplaceFunctionCalls(file *dst.File) {
	if r.replaceFnReverseMap == nil {
		r.replaceFnReverseMap = buildReplaceFnReverseMap()
	}
	if len(r.replaceFnReverseMap) == 0 {
		return
	}

	// Collect whatap import paths actually present in this file. Used to
	// disambiguate when multiple Rules share the same WhatapAlias (e.g.
	// go-redis v8 and v9 both register under `whatapgoredis`).
	whatapImports := map[string]bool{}
	for _, imp := range file.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		if strings.HasPrefix(p, "github.com/whatap/go-api/") {
			whatapImports[p] = true
		}
	}

	importsToAdd := map[string]bool{}
	dst.Inspect(file, func(n dst.Node) bool {
		call, ok := n.(*dst.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*dst.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			return true
		}
		candidates, exists := r.replaceFnReverseMap[ident.Name+"."+sel.Sel.Name]
		if !exists || len(candidates) == 0 {
			return true
		}

		// Choose the candidate whose whatap import is actually imported by
		// this file. If none match and only one candidate exists, accept it
		// (single-source alias). Otherwise skip — too ambiguous.
		var chosen *replaceFnReverse
		for i := range candidates {
			if whatapImports[candidates[i].whatapImportPath] {
				chosen = &candidates[i]
				break
			}
		}
		if chosen == nil {
			if len(candidates) == 1 {
				chosen = &candidates[0]
			} else {
				return true
			}
		}

		ident.Name = chosen.origAlias
		sel.Sel.Name = chosen.origFunc
		importsToAdd[chosen.origImportPath] = true
		return true
	})

	for imp := range importsToAdd {
		common.AddImport(file, `"`+imp+`"`)
	}
}

// buildReplaceFnReverseMap reads LoadBuiltinRules() once and constructs the
// whatap→original mapping for every ReplaceFunction Rule. Cached on first
// use via Remover.replaceFnReverseMap.
//
// §272 invariant: this map lives inside the remove engine; it does not
// reintroduce the Registry whatapRules / LookupWhatap reverse infrastructure
// that Phase 3 Step 2 deleted.
func buildReplaceFnReverseMap() map[string][]replaceFnReverse {
	m := map[string][]replaceFnReverse{}
	for _, rule := range LoadBuiltinRules() {
		if rule == nil {
			continue
		}
		rf, ok := rule.Advice.(*ReplaceFunction)
		if !ok {
			continue
		}
		if rf.WhatapAlias == "" || rf.WhatapFunc == "" {
			continue
		}
		origImportPath := extractImportPath(rule.Target)
		origFunc := lastDotSegment(rule.Target)
		if origImportPath == "" || origFunc == "" {
			continue
		}
		origAlias := computeBaseAlias(origImportPath)
		if origAlias == "" {
			continue
		}
		key := rf.WhatapAlias + "." + rf.WhatapFunc
		m[key] = append(m[key], replaceFnReverse{
			whatapImportPath: rf.WhatapPkg,
			origImportPath:   origImportPath,
			origAlias:        origAlias,
			origFunc:         origFunc,
		})
	}
	return m
}

// lastDotSegment returns the substring after the last "." in s.
func lastDotSegment(s string) string {
	i := strings.LastIndex(s, ".")
	if i < 0 {
		return ""
	}
	return s[i+1:]
}

// computeBaseAlias returns the default Go import alias for importPath:
// path.Base, with v-numeric suffixes (e.g. /v9) skipped one level up
// — matching Go's own default-alias convention.
func computeBaseAlias(importPath string) string {
	base := path.Base(importPath)
	if isVersionSuffix(base) {
		base = path.Base(path.Dir(importPath))
	}
	return base
}
