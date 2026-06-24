package ast

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/whatap/go-api-inst/ast/common"
	"github.com/whatap/go-api-inst/ast/custom"
	"github.com/whatap/go-api-inst/config"
	"github.com/whatap/go-api-inst/report"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// Injector injects monitoring code into source files using the v2 engine.
type Injector struct {
	errorTracingInjected bool // whether error tracing code has been injected
	ErrorTrackingEnabled bool // enabled via --error-tracking option

	// §242 — package-path filter. Values are full Rule.Target packages
	// (e.g. "fmt", "github.com/gin-gonic/gin"). Replaces the former single
	// EnabledPackages with Name keys.
	EnabledPackages  []string // opt-in list; OptIn rules register only if listed here
	DisabledPackages []string // exclusion list; rules register only if NOT listed here

	// ReplacedModules is the list of go.mod replace directive module paths (§205).
	ReplacedModules []string

	// SkipReplacedModules controls whether the engine skips Rules whose target
	// module appears in ReplacedModules (§205/§271). Default true preserves
	// the pre-§227-Step-5 behaviour. Toggled by
	// config.InstrumentationConfig.SkipReplacedModules.
	SkipReplacedModules bool

	// Config holds the full configuration (for custom rules access)
	Config *config.Config

	// typeChecker caches loaded packages for go/types type checking (§163)
	typeChecker *common.TypeChecker

	// registry holds v2 rules
	registry *Registry
}

// NewInjector creates a new v2 injector with type checking enabled.
func NewInjector() *Injector {
	inj := &Injector{
		ErrorTrackingEnabled: false,
		SkipReplacedModules:  true, // §271 default — preserve §205 behaviour
		typeChecker:          common.NewTypeChecker(),
	}
	inj.buildRegistry()
	return inj
}

// buildRegistry creates the v2 Registry from all declared rules.
//
// As of §227 Step 5 it loads two sources into the same registry:
//  1. built-in 92 rules from rules.yaml (or rules.go fallback)
//  2. user-defined rules from inj.Config (unified `rules:` schema, parsed by
//     LoadCustomRules). Skipped silently when Config is nil — callers that
//     attach a Config later should call SetConfig() so the registry is
//     re-built with the user rules.
func (inj *Injector) buildRegistry() {
	inj.registry = NewRegistry()
	inj.registry.SetPackageFilter(inj.EnabledPackages, inj.DisabledPackages)
	builtins := LoadBuiltinRules()
	// §242 Step 11 — warn (never fail) on user yaml paths that do not match
	// any built-in rule. Run once against the full built-in set before the
	// registry drops OptIn/disabled rules, so the warning reflects the user's
	// config as-written.
	ValidatePackageFilter(inj.EnabledPackages, inj.DisabledPackages, builtins)
	for _, r := range builtins {
		if r != nil {
			inj.registry.Register(r)
		}
	}
	builtinCount := inj.registry.Size()
	if inj.Config != nil {
		userRules, err := LoadCustomRules(inj.Config)
		if err != nil {
			// Don't fail the whole build for a single bad user rule —
			// surface it via stderr and continue with built-ins.
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] custom rules: %v\n", err)
			return
		}
		for _, r := range userRules {
			if r != nil {
				inj.registry.RegisterUser(r)
			}
		}
		if engineDebug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] buildRegistry: builtin=%d, user=%d, total=%d\n",
				builtinCount, len(userRules), inj.registry.Size())
		}
	} else if engineDebug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] buildRegistry: builtin=%d (no Config)\n", builtinCount)
	}
}

// SetConfig attaches a config.Config to the injector and re-builds the
// registry so user-defined rules from cfg.Rules are picked up. Use this
// instead of the bare `inj.Config = cfg` assignment when the caller wants
// custom rules to take effect.
func (inj *Injector) SetConfig(cfg *config.Config) {
	inj.Config = cfg
	if cfg != nil {
		inj.SkipReplacedModules = cfg.Instrumentation.ShouldSkipReplacedModules()
	}
	inj.buildRegistry()
}

// Rules returns the set of rules currently registered in the injector's
// registry. Callers (notably the --report dependency matcher) read this to
// stay in lock step with what the engine will actually apply at run time
// — builtin rules after nameFilter, user rules, everything.
// §240.
func (inj *Injector) Rules() []*Rule {
	return inj.registry.AllRules()
}

// InjectFile injects monitoring code into a single file using the v2 engine.
func (inj *Injector) InjectFile(srcPath, dstPath string) error {
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
		return inj.copyFile(srcPath, dstPath)
	}

	// §169 Phase 1: Lightweight parse first (no type info, no packages.Load)
	file, parseErr := decorator.Parse(src)
	if parseErr != nil {
		report.Get().AddFile(report.FileReport{
			Path:   srcPath,
			Status: report.StatusError,
			Error:  fmt.Sprintf("parse error: %v", parseErr),
			Reason: "copied as-is",
		})
		return inj.copyFile(srcPath, dstPath)
	}

	// Skip if whatap is already imported
	if common.HasWhatapImport(file) {
		report.Get().AddFile(report.FileReport{
			Path:   srcPath,
			Status: report.StatusSkipped,
			Reason: "already instrumented",
		})
		return inj.copyFile(srcPath, dstPath)
	}

	// §169 Phase 2: Early filtering
	hasMainFunc := common.FindNonEmptyMainFunc(file) != nil

	hasCustomRules := inj.Config != nil && len(inj.Config.Rules) > 0

	// v2: Check if any rule targets could match imports in this file
	hasTargetImports := inj.hasTargetImports(file)

	if !hasTargetImports && !hasMainFunc && !hasCustomRules {
		report.Get().AddFile(report.FileReport{
			Path:   srcPath,
			Status: report.StatusSkipped,
			Reason: "no target packages and no main func",
		})
		return inj.copyFile(srcPath, dstPath)
	}

	// §169 Phase 3: Load type info only for files that need injection (~20%)
	if common.HasImportcfgTypeCache() {
		typedFile := common.TrySetupTypeContextFromImportcfg(srcPath)
		if typedFile != nil {
			file = typedFile
			hasMainFunc = common.FindNonEmptyMainFunc(file) != nil
		}
		defer common.ClearTypeContext()
	} else if inj.typeChecker != nil {
		typedFile := common.TrySetupTypeContext(inj.typeChecker, srcPath)
		if typedFile != nil {
			file = typedFile
			hasMainFunc = common.FindNonEmptyMainFunc(file) != nil
		}
		defer common.ClearTypeContext()
	}

	// §169 Phase 4: Inject
	var changes []string

	// §185: Always use "whataptrace" alias
	traceAlias := "whataptrace"

	if hasMainFunc {
		common.AddImportWithAlias(file, "github.com/whatap/go-api/trace", traceAlias)
		changes = append(changes, "added import: github.com/whatap/go-api/trace (alias whataptrace)")
	}

	// Add trace.Init/Shutdown to main() function
	if hasMainFunc {
		changes = append(changes, fmt.Sprintf("added: %s.Init(nil)", traceAlias))
		changes = append(changes, fmt.Sprintf("added: defer %s.Shutdown()", traceAlias))
	}
	inj.injectMainInit(file, traceAlias)

	// v2 Engine: single traversal with target-based matching
	engine := NewEngine(inj.registry, ModeInject, newResolveFunc())
	// §271 — wire go.mod replace skip-list through to the engine
	engine.SetReplacedModules(inj.ReplacedModules)
	engine.SetSkipReplacedModules(inj.SkipReplacedModules)
	engineTransformed := engine.Process(file)

	if engineTransformed {
		changes = append(changes, "applied: v2 engine rules")
	}

	// Inject error tracing code (only if --error-tracking option is enabled)
	if inj.ErrorTrackingEnabled {
		inj.injectErrorTracing(file)
		changes = append(changes, "added: error tracing")
	}

	// §227 Step 5: custom rules now live in the v2 Engine registry
	// (see buildRegistry) and run as part of engine.Process above. The
	// legacy applyCustomRules() pass has been removed.

	// Record in report
	status := report.StatusInstrumented
	if !hasMainFunc && !engineTransformed && !hasCustomRules {
		status = report.StatusSkipped
	}
	// §240: record file size/line count so report has rough per-file metrics
	// for remote diagnosis (e.g., large files that failed, tiny skipped ones).
	lineCount := 1
	for _, b := range src {
		if b == '\n' {
			lineCount++
		}
	}
	report.Get().AddFile(report.FileReport{
		Path:      srcPath,
		Status:    status,
		Changes:   changes,
		SizeBytes: len(src),
		LineCount: lineCount,
	})

	// Generate result file
	return inj.writeFile(file, dstPath)
}

// hasTargetImports checks if the file imports any package that has registered rules.
// This is a lightweight check for Phase 2 early filtering.
func (inj *Injector) hasTargetImports(file *dst.File) bool {
	if inj.registry.Size() == 0 {
		return false
	}
	// Check each import against registered rules
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		// Check if any rule target starts with this import path
		for _, rule := range inj.registry.AllRules() {
			if strings.HasPrefix(rule.Target, importPath+".") {
				return true
			}
		}
	}
	return false
}

// InjectDir injects monitoring code into all Go files in a directory
func (inj *Injector) InjectDir(srcDir, dstDir string) error {
	absSrcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	absDstDir, err := filepath.Abs(dstDir)
	if err != nil {
		return err
	}

	// §227 Step 5: Add rules now live at cfg.Add (top-level). Engine 밖
	// 처리는 그대로 — ast/custom/add.go 가 cfg.Add 를 소비.
	if inj.Config != nil && len(inj.Config.Add) > 0 {
		if err := custom.ApplyAddRules(dstDir, inj.Config.BaseDir, inj.Config.Add); err != nil {
			return fmt.Errorf("apply add rules: %w", err)
		}
	}

	walkErr := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		// Skip if output directory is inside source
		if info.IsDir() && strings.HasPrefix(absPath, absDstDir) {
			return filepath.SkipDir
		}

		// Skip directories based on exclude patterns
		if info.IsDir() {
			var excludePatterns []string
			if inj.Config != nil {
				excludePatterns = inj.Config.GetExcludePatterns()
			}
			if common.ShouldSkipDirectory(path, absSrcDir, excludePatterns) {
				return filepath.SkipDir
			}
		}

		relPath, err := filepath.Rel(absSrcDir, absPath)
		if err != nil {
			return err
		}

		// Skip output directory
		if strings.HasPrefix(relPath, "output") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// Process only .go files
		if strings.HasSuffix(path, ".go") {
			var excludePatterns []string
			if inj.Config != nil {
				excludePatterns = inj.Config.GetExcludePatterns()
			}
			if common.ShouldSkipFile(path, absSrcDir, excludePatterns) {
				report.Get().AddFile(report.FileReport{
					Path:   path,
					Status: report.StatusCopied,
					Reason: "excluded by pattern",
				})
				return inj.copyFile(path, dstPath)
			}
			return inj.InjectFile(path, dstPath)
		}

		// Copy other files
		report.Get().AddFile(report.FileReport{
			Path:   path,
			Status: report.StatusCopied,
			Reason: "non-go file",
		})
		return inj.copyFile(path, dstPath)
	})

	if walkErr != nil {
		return walkErr
	}

	return nil
}

// injectMainInit adds trace.Init/Shutdown to main() function
func (inj *Injector) injectMainInit(file *dst.File, traceAlias string) {
	dst.Inspect(file, func(n dst.Node) bool {
		fn, ok := n.(*dst.FuncDecl)
		if !ok || fn.Name.Name != "main" || fn.Recv != nil {
			return true
		}

		if fn.Body == nil || len(fn.Body.List) == 0 {
			return true
		}

		initStmt := &dst.ExprStmt{
			X: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent(traceAlias),
					Sel: dst.NewIdent("Init"),
				},
				Args: []dst.Expr{dst.NewIdent("nil")},
			},
		}
		initStmt.Decs.After = dst.NewLine

		shutdownStmt := &dst.DeferStmt{
			Call: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent(traceAlias),
					Sel: dst.NewIdent("Shutdown"),
				},
			},
		}
		shutdownStmt.Decs.After = dst.NewLine

		newList := make([]dst.Stmt, 0, len(fn.Body.List)+2)
		newList = append(newList, initStmt, shutdownStmt)
		newList = append(newList, fn.Body.List...)
		fn.Body.List = newList

		return false
	})
}

// writeFile writes the transformed file to disk
func (inj *Injector) writeFile(file *dst.File, dstPath string) error {
	return common.WriteDstFile(file, dstPath)
}

// copyFile copies a file
func (inj *Injector) copyFile(src, dstPath string) error {
	return common.CopyFile(src, dstPath)
}

// injectErrorTracing injects error tracing code into the file
func (inj *Injector) injectErrorTracing(file *dst.File) {
	inj.errorTracingInjected = false
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if fn.Name.Name == "main" && fn.Recv == nil {
			continue
		}
		fn.Body.List = inj.processStmtsForErrorTracing(fn.Body.List)
	}
	if inj.errorTracingInjected {
		common.AddImport(file, "context")
	}
}

// processStmtsForErrorTracing processes error check patterns in statement list
func (inj *Injector) processStmtsForErrorTracing(stmts []dst.Stmt) []dst.Stmt {
	var newStmts []dst.Stmt
	var prevStmt dst.Stmt

	for i, stmt := range stmts {
		newStmts = append(newStmts, stmt)

		skipErrorTracing := false
		if prevStmt != nil && inj.isWhatapPackageCall(prevStmt) {
			if _, ok := stmt.(*dst.IfStmt); ok {
				skipErrorTracing = true
			}
		}

		if !skipErrorTracing {
			inj.processStmtForErrorTracing(stmt)
		}

		if ifStmt, ok := stmt.(*dst.IfStmt); ok {
			if prevStmt != nil && inj.isWhatapPackageCall(prevStmt) {
				prevStmt = stmt
				continue
			}
			errVarName := inj.getErrEqualNilVarName(ifStmt.Cond)
			if errVarName != "" && inj.blockHasReturn(ifStmt.Body) {
				if i < len(stmts)-1 {
					traceErrorStmt := inj.createTraceErrorStmt(errVarName)
					newStmts = append(newStmts, traceErrorStmt)
					inj.errorTracingInjected = true
				}
			}
		}

		prevStmt = stmt
	}

	return newStmts
}

// processStmtForErrorTracing processes error check patterns in a single statement
func (inj *Injector) processStmtForErrorTracing(stmt dst.Stmt) {
	switch s := stmt.(type) {
	case *dst.IfStmt:
		errVarName := inj.getErrCheckVarName(s.Cond)
		if errVarName != "" {
			inj.insertTraceErrorBeforeReturn(s.Body, errVarName)
		}
		errEqualNilVarName := inj.getErrEqualNilVarName(s.Cond)
		if errEqualNilVarName != "" && s.Else != nil {
			if elseBlock, ok := s.Else.(*dst.BlockStmt); ok {
				inj.insertTraceErrorBeforeReturn(elseBlock, errEqualNilVarName)
			}
		}
		if s.Else != nil {
			if elseBlock, ok := s.Else.(*dst.BlockStmt); ok {
				elseBlock.List = inj.processStmtsForErrorTracing(elseBlock.List)
			} else if elseIf, ok := s.Else.(*dst.IfStmt); ok {
				inj.processStmtForErrorTracing(elseIf)
			}
		}
		s.Body.List = inj.processStmtsForErrorTracing(s.Body.List)
	case *dst.BlockStmt:
		s.List = inj.processStmtsForErrorTracing(s.List)
	case *dst.ForStmt:
		if s.Body != nil {
			s.Body.List = inj.processStmtsForErrorTracing(s.Body.List)
		}
	case *dst.RangeStmt:
		if s.Body != nil {
			s.Body.List = inj.processStmtsForErrorTracing(s.Body.List)
		}
	case *dst.SwitchStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CaseClause); ok {
					cc.Body = inj.processStmtsForErrorTracing(cc.Body)
				}
			}
		}
	case *dst.SelectStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CommClause); ok {
					cc.Body = inj.processStmtsForErrorTracing(cc.Body)
				}
			}
		}
	}
}

func (inj *Injector) getErrCheckVarName(cond dst.Expr) string {
	binExpr, ok := cond.(*dst.BinaryExpr)
	if !ok || binExpr.Op != token.NEQ {
		return ""
	}
	if ident, ok := binExpr.Y.(*dst.Ident); !ok || ident.Name != "nil" {
		return ""
	}
	if ident, ok := binExpr.X.(*dst.Ident); ok {
		if ident.Name == "err" || ident.Name == "e" || ident.Name == "error" {
			return ident.Name
		}
	}
	return ""
}

func (inj *Injector) getErrEqualNilVarName(cond dst.Expr) string {
	binExpr, ok := cond.(*dst.BinaryExpr)
	if !ok || binExpr.Op != token.EQL {
		return ""
	}
	if ident, ok := binExpr.Y.(*dst.Ident); !ok || ident.Name != "nil" {
		return ""
	}
	if ident, ok := binExpr.X.(*dst.Ident); ok {
		if ident.Name == "err" || ident.Name == "e" || ident.Name == "error" {
			return ident.Name
		}
	}
	return ""
}

func (inj *Injector) blockHasReturn(block *dst.BlockStmt) bool {
	for _, stmt := range block.List {
		if _, ok := stmt.(*dst.ReturnStmt); ok {
			return true
		}
	}
	return false
}

func (inj *Injector) createTraceErrorStmt(errVarName string) *dst.ExprStmt {
	stmt := &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("whataptrace"),
				Sel: dst.NewIdent("Error"),
			},
			Args: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent("context"),
						Sel: dst.NewIdent("Background"),
					},
				},
				dst.NewIdent(errVarName),
			},
		},
	}
	stmt.Decs.After = dst.NewLine
	return stmt
}

func (inj *Injector) insertTraceErrorBeforeReturn(block *dst.BlockStmt, errVarName string) {
	var newList []dst.Stmt
	for _, stmt := range block.List {
		if _, ok := stmt.(*dst.ReturnStmt); ok {
			if len(newList) > 0 && inj.isTraceErrorCall(newList[len(newList)-1], errVarName) {
				newList = append(newList, stmt)
				continue
			}
			traceErrorStmt := &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent("whataptrace"),
						Sel: dst.NewIdent("Error"),
					},
					Args: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.SelectorExpr{
								X:   dst.NewIdent("context"),
								Sel: dst.NewIdent("Background"),
							},
						},
						dst.NewIdent(errVarName),
					},
				},
			}
			traceErrorStmt.Decs.After = dst.NewLine
			newList = append(newList, traceErrorStmt, stmt)
			inj.errorTracingInjected = true
		} else {
			newList = append(newList, stmt)
		}
	}
	block.List = newList
}

func (inj *Injector) isTraceErrorCall(stmt dst.Stmt, errVarName string) bool {
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

func (inj *Injector) isWhatapPackageCall(stmt dst.Stmt) bool {
	assignStmt, ok := stmt.(*dst.AssignStmt)
	if !ok {
		return false
	}
	if len(assignStmt.Rhs) != 1 {
		return false
	}
	call, ok := assignStmt.Rhs[0].(*dst.CallExpr)
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
		if identPath != "" {
			return strings.HasPrefix(identPath, "github.com/whatap/")
		}
	}
	return strings.HasPrefix(ident.Name, "whatap")
}

// (§227 Step 5: applyCustomRules removed — custom rules are now registered
// in the v2 Engine registry via buildRegistry/LoadCustomRules and execute
// during the same single AST pass as built-in rules. The legacy ast/custom/
// {replace,hook,inject,transform} files are deleted in the same step;
// add.go remains for the Engine-out file-creation rules.)
