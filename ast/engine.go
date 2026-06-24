package ast

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/dave/dst"
	"github.com/whatap/go-api-inst/ast/common"
)

var engineDebug = os.Getenv("GO_API_AST_DEBUG") != ""

// ResolveFunc converts a dst.Node to a Target string.
// Provided by v2types (go/types) or v2decorator (dst decoration).
// Returns "" if the node cannot be resolved.
type ResolveFunc func(node dst.Node) string

// Engine performs a single AST traversal, matching nodes against the Registry
// and applying Advice transformations.
type Engine struct {
	registry      *Registry
	mode          Mode
	resolve       ResolveFunc
	enclosingFunc *dst.FuncDecl
	transformed   bool
	whatapImports map[string]string // whatap import path → alias (to add)
	replacedPkgs  map[string]string // original import path → pkg alias (removal candidates)

	// §271 — replace directive skip-list. When skipReplacedModules is true
	// and a Rule's Target import path matches (prefix) any entry in
	// replacedModules, the rule is skipped at the entry point of
	// matchAndApply / matchFuncDecl. Mirrors v1 §205 behaviour
	// (b4131398 / 2026-03-24) lost during §227 Step 5 v1 retirement.
	replacedModules     []string
	skipReplacedModules bool
}

// NewEngine creates a new Engine.
func NewEngine(registry *Registry, mode Mode, resolve ResolveFunc) *Engine {
	return &Engine{
		registry:      registry,
		mode:          mode,
		resolve:       resolve,
		whatapImports: make(map[string]string),
		replacedPkgs:  make(map[string]string),
	}
}

// SetReplacedModules supplies the go.mod replace directive module paths.
// Combined with SetSkipReplacedModules(true) (default) this makes the engine
// skip Rules whose Target import path prefix-matches any entry. §205/§271.
func (e *Engine) SetReplacedModules(mods []string) {
	e.replacedModules = mods
}

// SetSkipReplacedModules toggles the §205/§271 replace skip check.
func (e *Engine) SetSkipReplacedModules(skip bool) {
	e.skipReplacedModules = skip
}

// isReplacedTarget reports whether the Rule target's package path
// matches any go.mod replace directive entry. §271 — mirrors v1
// Injector.isReplacedModule logic. Returns false when the skip is
// disabled or the replace list is empty so callers pay no cost in
// the common case.
//
// Uses ExtractRulePackage (not extractImportPath) so method-style targets
// like "net/http.DefaultClient.Get" and "github.com/gorilla/mux.Route.Subrouter"
// resolve to their package (`net/http`, `github.com/gorilla/mux`) for
// matching against `replace` left-hand modules.
func (e *Engine) isReplacedTarget(target string) bool {
	if !e.skipReplacedModules || len(e.replacedModules) == 0 {
		return false
	}
	pkg := ExtractRulePackage(target)
	if pkg == "" {
		return false
	}
	for _, mod := range e.replacedModules {
		if mod == "" {
			continue
		}
		if pkg == mod || strings.HasPrefix(pkg, mod+"/") {
			return true
		}
	}
	return false
}

// Process runs the engine on a file. Returns true if any transformation occurred.
func (e *Engine) Process(file *dst.File) bool {
	e.transformed = false
	e.whatapImports = make(map[string]string)
	e.replacedPkgs = make(map[string]string)

	// Traverse AST — match rules, apply transformations
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *dst.FuncDecl:
			e.enclosingFunc = d
			e.matchFuncDecl(file, d)
			if d.Body != nil {
				e.processBlock(file, &d.Body.List)
			}
		case *dst.GenDecl:
			e.enclosingFunc = nil // Package-level var/const — no enclosing function
			e.processGenDecl(file, d)
		}
	}

	// Step 3: Resolve imports based on ref counts (single pass)
	if e.mode == ModeInject {
		e.resolveImports(file)
	}

	return e.transformed
}

// resolveImports applies import changes after all transformations are done.
// §225: No pre-traversal. Tracks only what was added/replaced during transformation.
// For replaced packages, does a single post-transform scan to check if still used.
func (e *Engine) resolveImports(file *dst.File) {
	originalImports := common.GetImportPathSet(file)

	// Step 1: Remove replaced imports only if no longer referenced in code
	if len(e.replacedPkgs) > 0 {
		usedPkgs := collectUsedPackages(file)
		for importPath, alias := range e.replacedPkgs {
			if originalImports[importPath] && !usedPkgs[alias] {
				common.RemoveImport(file, importPath)
			}
		}
	}

	// Step 2: Add blank imports for packages with whatap transformations
	for origPath, whatapImport := range e.registry.BlankImports() {
		if originalImports[origPath] {
			if _, hasWhatap := e.whatapImports[whatapImport]; hasWhatap {
				common.AddImport(file, whatapImport)
			}
		}
	}

	// Step 3: Add whatap imports
	for importPath, alias := range e.whatapImports {
		if !originalImports[importPath] {
			if alias != "" && !isRedundantAlias(importPath, alias) {
				common.AddImportWithAlias(file, importPath, alias)
			} else {
				common.AddImport(file, importPath)
			}
		}
	}

	common.CleanupEmptyImports(file)
}

// collectUsedPackages scans the file for all package identifiers in SelectorExpr.
// Returns a set of package identifier names (e.g., "http", "fmt", "gin").
func collectUsedPackages(file *dst.File) map[string]bool {
	used := make(map[string]bool)
	dst.Inspect(file, func(n dst.Node) bool {
		sel, ok := n.(*dst.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			return true
		}
		used[ident.Name] = true
		return true
	})
	return used
}

// Transformed returns whether any transformation occurred.
func (e *Engine) Transformed() bool {
	return e.transformed
}


// processBlock traverses statements in reverse order for safe insertion.
func (e *Engine) processBlock(file *dst.File, stmts *[]dst.Stmt) {
	for i := len(*stmts) - 1; i >= 0; i-- {
		stmt := (*stmts)[i]
		e.processStmt(file, stmts, i, stmt)
		e.processNestedBlocks(file, stmt)
	}
}

// processStmt inspects a single statement for matchable AST nodes.
func (e *Engine) processStmt(file *dst.File, block *[]dst.Stmt, idx int, stmt dst.Stmt) {
	dst.Inspect(stmt, func(node dst.Node) bool {
		if node == nil {
			return false
		}
		matched := e.matchAndApply(file, node, block, idx, stmt)
		if matched {
			// Don't descend into children of transformed nodes.
			// WrapCall creates inner CallExpr that would re-match → infinite recursion.
			return false
		}
		return true
	})
}

// processGenDecl handles package-level declarations (e.g. var x = http.Client{}).
func (e *Engine) processGenDecl(file *dst.File, decl *dst.GenDecl) {
	dst.Inspect(decl, func(node dst.Node) bool {
		if node == nil {
			return false
		}
		matched := e.matchAndApply(file, node, nil, -1, nil)
		if matched {
			return false
		}
		return true
	})
}

// matchFuncDecl tries to match a FuncDecl against "decl:..." rules.
func (e *Engine) matchFuncDecl(file *dst.File, fn *dst.FuncDecl) {
	target := e.resolve(fn)
	if target == "" {
		return
	}

	if engineDebug {
		fmt.Fprintf(os.Stderr, "[v2-resolve] target=%q\n", target)
	}

	// §271 — skip Rules whose target module is replaced in go.mod
	if e.isReplacedTarget(target) {
		if engineDebug {
			fmt.Fprintf(os.Stderr, "[v2-resolve] skip target=%q (replaced in go.mod)\n", target)
		}
		return
	}

	// §272 Phase 3 Step 2 — ModeRemove 경로 미사용. forward map 만 조회.
	rule := e.registry.Lookup(target)
	if rule == nil {
		return
	}

	ctx := &MatchContext{
		File:          file,
		Mode:          e.mode,
		Target:        target,
		Rule:          rule,
		Decl:          fn,
		EnclosingFunc: fn,
		FuncName:      fn.Name.Name,
	}
	if fn.Body != nil {
		ctx.ParentBlock = &fn.Body.List
	}

	// Optional filter chain (Steps 3-4 through 3-10)
	if !validateRule(ctx, rule) {
		return
	}

	ctx.Applied = true // default: assume applied (most Advice types always apply)
	rule.Advice.Apply(ctx)

	if !ctx.Applied {
		return // Advice skipped — no import, no transform
	}
	e.transformed = true

	// Track whatap imports to add and original imports to potentially remove
	if e.mode == ModeInject {
		whatapPkg := rule.Advice.WhatapImportPath()
		whatapAlias := rule.Advice.WhatapImportAlias()
		if whatapPkg != "" {
			e.whatapImports[whatapPkg] = whatapAlias
		}
		for pkg, alias := range ctx.ExtraImports {
			e.whatapImports[pkg] = alias
		}

		// Replace types change the package identifier — original may become unused
		switch rule.Advice.(type) {
		case *ReplaceFunction, *ReplaceWithCtx, *Transform:
			origImport := extractImportPath(rule.Target)
			if origImport != "" && ctx.PkgName != "" {
				if _, exists := e.replacedPkgs[origImport]; !exists {
					e.replacedPkgs[origImport] = ctx.PkgName
				}
			}
		}
	}
}

// matchAndApply resolves a node's target and applies the matching rule.
// Returns true if a rule was applied (caller should skip children to avoid re-matching).
func (e *Engine) matchAndApply(file *dst.File, node dst.Node, block *[]dst.Stmt, idx int, stmt dst.Stmt) bool {
	target := e.resolve(node)
	if target == "" {
		return false
	}

	if engineDebug {
		funcName := ""
		if e.enclosingFunc != nil {
			funcName = e.enclosingFunc.Name.Name
		}
		fmt.Fprintf(os.Stderr, "[v2-resolve] target=%q  func=%s\n", target, funcName)
	}

	// §271 — skip Rules whose target module is replaced in go.mod
	if e.isReplacedTarget(target) {
		if engineDebug {
			fmt.Fprintf(os.Stderr, "[v2-resolve] skip target=%q (replaced in go.mod)\n", target)
		}
		return false
	}

	// §272 Phase 3 Step 2 — ModeRemove 경로 미사용. forward map 만 조회.
	rule := e.registry.Lookup(target)
	if rule == nil {
		return false
	}

	ctx := e.buildContext(file, node, target, rule, block, idx, stmt)
	if ctx == nil {
		return false
	}

	// Optional filter chain (Steps 3-4 through 3-10)
	if !validateRule(ctx, rule) {
		return false
	}

	ctx.Applied = true // default: assume applied (most Advice types always apply)
	rule.Advice.Apply(ctx)

	if !ctx.Applied {
		return false // Advice skipped (e.g., MainInsert not in main()) — no import, no transform
	}
	e.transformed = true

	// Track whatap imports to add and original imports to potentially remove
	if e.mode == ModeInject {
		whatapPkg := rule.Advice.WhatapImportPath()
		whatapAlias := rule.Advice.WhatapImportAlias()
		if whatapPkg != "" {
			e.whatapImports[whatapPkg] = whatapAlias
		}
		for pkg, alias := range ctx.ExtraImports {
			e.whatapImports[pkg] = alias
		}

		// Replace types change the package identifier — original may become unused
		switch rule.Advice.(type) {
		case *ReplaceFunction, *ReplaceWithCtx, *Transform:
			origImport := extractImportPath(rule.Target)
			if origImport != "" && ctx.PkgName != "" {
				// Don't overwrite: first match has the correct original alias.
				// Later matches on the same node (double-visit from dst.Inspect + processNestedBlocks)
				// may see the already-transformed alias (e.g., "whatapfmt" instead of "fmt").
				if _, exists := e.replacedPkgs[origImport]; !exists {
					e.replacedPkgs[origImport] = ctx.PkgName
				}
			}
		}
	}
	return true
}

// processNestedBlocks recurses into nested block structures.
func (e *Engine) processNestedBlocks(file *dst.File, stmt dst.Stmt) {
	switch s := stmt.(type) {
	case *dst.IfStmt:
		if s.Body != nil {
			e.processBlock(file, &s.Body.List)
		}
		if s.Else != nil {
			switch els := s.Else.(type) {
			case *dst.BlockStmt:
				e.processBlock(file, &els.List)
			case *dst.IfStmt:
				if els.Body != nil {
					e.processBlock(file, &els.Body.List)
				}
				e.processNestedBlocks(file, els)
			}
		}
	case *dst.ForStmt:
		if s.Body != nil {
			e.processBlock(file, &s.Body.List)
		}
	case *dst.RangeStmt:
		if s.Body != nil {
			e.processBlock(file, &s.Body.List)
		}
	case *dst.SwitchStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CaseClause); ok {
					e.processBlock(file, &cc.Body)
				}
			}
		}
	case *dst.TypeSwitchStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CaseClause); ok {
					e.processBlock(file, &cc.Body)
				}
			}
		}
	case *dst.SelectStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CommClause); ok {
					e.processBlock(file, &cc.Body)
				}
			}
		}
	}
}

// buildContext constructs a MatchContext from a matched node.
func (e *Engine) buildContext(file *dst.File, node dst.Node, target string, rule *Rule, block *[]dst.Stmt, idx int, stmt dst.Stmt) *MatchContext {
	ctx := &MatchContext{
		File:          file,
		Mode:          e.mode,
		Target:        target,
		Rule:          rule,
		EnclosingFunc: e.enclosingFunc,
		EnclosingStmt: stmt,
		ParentBlock:   block,
		StmtIndex:     idx,
	}

	switch n := node.(type) {
	case *dst.CallExpr:
		ctx.Call = n
		if sel, ok := n.Fun.(*dst.SelectorExpr); ok {
			ctx.Sel = sel
			ctx.FuncName = sel.Sel.Name
			if ident, ok := sel.X.(*dst.Ident); ok {
				ctx.Ident = ident
				ctx.PkgName = ident.Name
			}
		}
	case *dst.CompositeLit:
		ctx.Lit = n
		if sel, ok := n.Type.(*dst.SelectorExpr); ok {
			ctx.Sel = sel
			ctx.FuncName = sel.Sel.Name
			if ident, ok := sel.X.(*dst.Ident); ok {
				ctx.Ident = ident
				ctx.PkgName = ident.Name
			}
		}
	}

	return ctx
}

// isRedundantAlias returns true if the alias matches the last segment of the import path.
// e.g., "whatapsql" is redundant for ".../whatapsql".
func isRedundantAlias(importPath, alias string) bool {
	return path.Base(importPath) == alias
}

// extractImportPath extracts the import path from a Target string.
// "database/sql.Open" → "database/sql"
// "github.com/jmoiron/sqlx.Connect" → "github.com/jmoiron/sqlx"
func extractImportPath(target string) string {
	lastDot := strings.LastIndex(target, ".")
	if lastDot < 0 {
		return ""
	}
	return target[:lastDot]
}

