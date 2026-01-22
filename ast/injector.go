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

// Injector injects monitoring code into source files
type Injector struct {
	errorTracingInjected bool // whether error tracing code has been injected
	ErrorTrackingEnabled bool // enabled via --error-tracking option

	// EnabledPackages is the list of enabled packages (nil means all packages enabled)
	// Set via Config.GetEnabledPackages()
	EnabledPackages []string

	// Config holds the full configuration (for custom rules access)
	Config *config.Config
}

// NewInjector creates a new injector
func NewInjector() *Injector {
	return &Injector{
		ErrorTrackingEnabled: false, // default: disabled
		EnabledPackages:      nil,   // default: all packages enabled
	}
}

// isPackageEnabled checks if a package is enabled
func (inj *Injector) isPackageEnabled(name string) bool {
	// nil means all packages enabled (backward compatibility)
	if inj.EnabledPackages == nil {
		return true
	}
	for _, pkg := range inj.EnabledPackages {
		if pkg == name {
			return true
		}
	}
	return false
}

// InjectFile injects monitoring code into a single file
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

	// Check for main function first (determines whether to add trace.Init)
	hasMainFunc := common.FindMainFunc(file) != nil

	// Phase 6: Use transformer registry
	// Get only enabled transformers from detected packages in file
	transformers := common.GetFilteredTransformers(file, inj.EnabledPackages)

	// Check if custom rules exist (§4)
	hasCustomRules := inj.Config != nil && (len(inj.Config.Custom.Inject) > 0 ||
		len(inj.Config.Custom.Replace) > 0 ||
		len(inj.Config.Custom.Hook) > 0 ||
		len(inj.Config.Custom.Transform) > 0)

	// Copy as-is if no main function and no packages to transform
	// However, if custom rules are defined, don't skip as they need to be applied
	// Even with preset: minimal, trace.Init must be added if main function exists
	if len(transformers) == 0 && !hasMainFunc && !hasCustomRules {
		report.Get().AddFile(report.FileReport{
			Path:   srcPath,
			Status: report.StatusSkipped,
			Reason: "no target packages and no main func",
		})
		return inj.copyFile(srcPath, dstPath)
	}

	// For recording changes
	var changes []string
	var transformerNames []string

	// Determine trace package alias (detect name conflicts)
	traceAlias := "trace"
	if common.IsNameDeclared(file, "trace") {
		traceAlias = "whataptrace"
	}

	if hasMainFunc {
		if traceAlias == "trace" {
			common.AddImport(file, `"github.com/whatap/go-api/trace"`)
		} else {
			common.AddImportWithAlias(file, "github.com/whatap/go-api/trace", traceAlias)
		}
		changes = append(changes, "added import: github.com/whatap/go-api/trace")
	}

	// Add trace.Init/Shutdown to main() function
	if hasMainFunc {
		changes = append(changes, fmt.Sprintf("added: %s.Init(nil)", traceAlias))
		changes = append(changes, fmt.Sprintf("added: defer %s.Shutdown()", traceAlias))
	}
	inj.injectMainInit(file, traceAlias)

	// Phase 6: Call transformer.Inject()
	// TypedTransformer uses go/types to get type information
	// §54: Add whatap import only when Inject() returns true
	srcDir := filepath.Dir(srcPath)
	k8sProcessed := false
	for _, t := range transformers {
		// k8s and k8srest use the same injection, so prevent duplicates
		name := t.Name()
		if name == "k8s" || name == "k8srest" {
			if k8sProcessed {
				continue
			}
			k8sProcessed = true
		}

		// If TypedTransformer interface is implemented, call with directory info
		var transformed bool
		var injErr error
		if typed, ok := t.(common.TypedTransformer); ok {
			transformed, injErr = typed.InjectWithDir(file, srcDir)
		} else {
			transformed, injErr = t.Inject(file)
		}

		if injErr != nil {
			report.Get().AddFile(report.FileReport{
				Path:   srcPath,
				Status: report.StatusError,
				Error:  fmt.Sprintf("inject %s: %v", name, injErr),
			})
			return fmt.Errorf("inject %s: %w", name, injErr)
		}

		// §54: Add whatap import only when actual transformation occurred
		if transformed {
			whatapImport := t.WhatapImport()
			if whatapImport != "" {
				// Use alias if AliasedImportTransformer is implemented
				if aliased, ok := t.(common.AliasedImportTransformer); ok {
					alias := aliased.WhatapImportAlias()
					common.AddImportWithAlias(file, whatapImport, alias)
					changes = append(changes, fmt.Sprintf("added import: %s (alias: %s)", whatapImport, alias))
				} else {
					common.AddImport(file, `"`+whatapImport+`"`)
					changes = append(changes, fmt.Sprintf("added import: %s", whatapImport))
				}
			}
			transformerNames = append(transformerNames, t.Name())
			changes = append(changes, fmt.Sprintf("applied: %s transformer", name))
		}
	}

	// Inject error tracing code (only if --error-tracking option is enabled)
	// Default is disabled (whatap package already tracks errors, avoid duplicates)
	if inj.ErrorTrackingEnabled {
		inj.injectErrorTracing(file)
		changes = append(changes, "added: error tracing")
	}

	// Apply custom rules (§4) (only if config exists)
	if inj.Config != nil {
		customChanges, err := inj.applyCustomRules(file, srcDir)
		if err != nil {
			report.Get().AddFile(report.FileReport{
				Path:   srcPath,
				Status: report.StatusError,
				Error:  fmt.Sprintf("apply custom rules: %v", err),
			})
			return fmt.Errorf("apply custom rules: %w", err)
		}
		changes = append(changes, customChanges...)
	}

	// Record in report
	report.Get().AddFile(report.FileReport{
		Path:         srcPath,
		Status:       report.StatusInstrumented,
		Transformers: transformerNames,
		Changes:      changes,
	})

	// NOTE: Unused import cleanup is handled by each transformer's RemoveImportIfUnused call.
	// CleanupAllUnusedImports was removed because it incorrectly removes imports like
	// "gopkg.in/yaml.v3" (parsed as "yaml.v3" instead of "yaml").

	// Generate result file
	return inj.writeFile(file, dstPath)
}

// InjectDir injects monitoring code into all Go files in a directory
func (inj *Injector) InjectDir(srcDir, dstDir string) error {
	// Convert to absolute paths
	absSrcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	absDstDir, err := filepath.Abs(dstDir)
	if err != nil {
		return err
	}

	// §4 Add rules: Create new files before file processing (except append)
	if inj.Config != nil && len(inj.Config.Custom.Add) > 0 {
		if err := custom.ApplyAddRules(dstDir, inj.Config.BaseDir, inj.Config.Custom.Add); err != nil {
			return fmt.Errorf("apply add rules: %w", err)
		}
	}

	// Process files
	walkErr := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Convert to absolute path
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

		// Process only .go files (skip based on exclude patterns)
		if strings.HasSuffix(path, ".go") {
			var excludePatterns []string
			if inj.Config != nil {
				excludePatterns = inj.Config.GetExcludePatterns()
			}
			if common.ShouldSkipFile(path, absSrcDir, excludePatterns) {
				// Copy instead of inject
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

	// §4 Append rules: Add code to existing files after file copy
	if inj.Config != nil && len(inj.Config.Custom.Add) > 0 {
		if err := custom.ApplyAppendRules(dstDir, inj.Config.BaseDir, inj.Config.Custom.Add); err != nil {
			return fmt.Errorf("apply append rules: %w", err)
		}
	}

	return nil
}

// injectMainInit adds trace.Init/Shutdown to main() function
// traceAlias: alias for trace package (default "trace", "whataptrace" on name conflict)
func (inj *Injector) injectMainInit(file *dst.File, traceAlias string) {
	dst.Inspect(file, func(n dst.Node) bool {
		fn, ok := n.(*dst.FuncDecl)
		if !ok || fn.Name.Name != "main" || fn.Recv != nil {
			return true
		}

		if fn.Body == nil || len(fn.Body.List) == 0 {
			return true
		}

		// Create trace.Init(nil) statement
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

		// Create defer trace.Shutdown() statement
		shutdownStmt := &dst.DeferStmt{
			Call: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent(traceAlias),
					Sel: dst.NewIdent("Shutdown"),
				},
			},
		}
		shutdownStmt.Decs.After = dst.NewLine

		// Insert before existing statements
		newList := make([]dst.Stmt, 0, len(fn.Body.List)+2)
		newList = append(newList, initStmt, shutdownStmt)
		newList = append(newList, fn.Body.List...)
		fn.Body.List = newList

		return false
	})
}

// writeFile writes the transformed file to disk
func (inj *Injector) writeFile(file *dst.File, dstPath string) error {
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
func (inj *Injector) copyFile(src, dst string) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// injectErrorTracing injects error tracing code into the file
func (inj *Injector) injectErrorTracing(file *dst.File) {
	inj.errorTracingInjected = false
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		// Skip main function (error before trace.Init should not occur)
		if fn.Name.Name == "main" && fn.Recv == nil {
			continue
		}

		// Traverse all if statements in function body
		fn.Body.List = inj.processStmtsForErrorTracing(fn.Body.List)
	}

	// Add context import if error tracing was injected
	if inj.errorTracingInjected {
		common.AddImport(file, "context")
	}
}

// processStmtsForErrorTracing processes error check patterns in statement list
func (inj *Injector) processStmtsForErrorTracing(stmts []dst.Stmt) []dst.Stmt {
	var newStmts []dst.Stmt
	var prevStmt dst.Stmt // Track previous statement

	for i, stmt := range stmts {
		newStmts = append(newStmts, stmt)

		// Skip if err != nil immediately after whatap package call
		skipErrorTracing := false
		if prevStmt != nil && inj.isWhatapPackageCall(prevStmt) {
			if _, ok := stmt.(*dst.IfStmt); ok {
				skipErrorTracing = true
				if os.Getenv("GO_API_AST_DEBUG") != "" {
					fmt.Println("[DEBUG] Skipping trace.Error injection after whatap package call")
				}
			}
		}

		if !skipErrorTracing {
			inj.processStmtForErrorTracing(stmt)
		}

		// Handle if err == nil { return } pattern
		if ifStmt, ok := stmt.(*dst.IfStmt); ok {
			// Skip if immediately after whatap call
			if prevStmt != nil && inj.isWhatapPackageCall(prevStmt) {
				prevStmt = stmt
				continue
			}

			errVarName := inj.getErrEqualNilVarName(ifStmt.Cond)
			if errVarName != "" && inj.blockHasReturn(ifStmt.Body) {
				// If if block has return, insert trace.Error after if statement
				// Only if there are following statements (subsequent code is err != nil case)
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
		// Check if it's if err != nil pattern
		errVarName := inj.getErrCheckVarName(s.Cond)
		if errVarName != "" {
			// Insert trace.Error before return statement in if block
			inj.insertTraceErrorBeforeReturn(s.Body, errVarName)
		}

		// Handle if err == nil { } else { return } pattern
		errEqualNilVarName := inj.getErrEqualNilVarName(s.Cond)
		if errEqualNilVarName != "" && s.Else != nil {
			// Insert trace.Error before return in else block
			if elseBlock, ok := s.Else.(*dst.BlockStmt); ok {
				inj.insertTraceErrorBeforeReturn(elseBlock, errEqualNilVarName)
			}
		}

		// Recursively process else block
		if s.Else != nil {
			if elseBlock, ok := s.Else.(*dst.BlockStmt); ok {
				elseBlock.List = inj.processStmtsForErrorTracing(elseBlock.List)
			} else if elseIf, ok := s.Else.(*dst.IfStmt); ok {
				inj.processStmtForErrorTracing(elseIf)
			}
		}

		// Also process nested if inside if block
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

// getErrCheckVarName extracts error variable name from err != nil condition
// Returns: error variable name (err, e, etc.) or empty string
func (inj *Injector) getErrCheckVarName(cond dst.Expr) string {
	// err != nil pattern
	binExpr, ok := cond.(*dst.BinaryExpr)
	if !ok || binExpr.Op != token.NEQ {
		return ""
	}

	// Check if right side is nil
	if ident, ok := binExpr.Y.(*dst.Ident); !ok || ident.Name != "nil" {
		return ""
	}

	// Check if left side is err variable
	if ident, ok := binExpr.X.(*dst.Ident); ok {
		// Names that look like error variables: err, e, error
		if ident.Name == "err" || ident.Name == "e" || ident.Name == "error" {
			return ident.Name
		}
	}

	return ""
}

// getErrEqualNilVarName extracts error variable name from err == nil condition
func (inj *Injector) getErrEqualNilVarName(cond dst.Expr) string {
	// err == nil pattern
	binExpr, ok := cond.(*dst.BinaryExpr)
	if !ok || binExpr.Op != token.EQL {
		return ""
	}

	// Check if right side is nil
	if ident, ok := binExpr.Y.(*dst.Ident); !ok || ident.Name != "nil" {
		return ""
	}

	// Check if left side is err variable
	if ident, ok := binExpr.X.(*dst.Ident); ok {
		if ident.Name == "err" || ident.Name == "e" || ident.Name == "error" {
			return ident.Name
		}
	}

	return ""
}

// blockHasReturn checks if block contains a return statement
func (inj *Injector) blockHasReturn(block *dst.BlockStmt) bool {
	for _, stmt := range block.List {
		if _, ok := stmt.(*dst.ReturnStmt); ok {
			return true
		}
	}
	return false
}

// createTraceErrorStmt creates trace.Error(context.Background(), err) statement
func (inj *Injector) createTraceErrorStmt(errVarName string) *dst.ExprStmt {
	stmt := &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("trace"),
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

// insertTraceErrorBeforeReturn inserts trace.Error(err) before return statement
func (inj *Injector) insertTraceErrorBeforeReturn(block *dst.BlockStmt, errVarName string) {
	var newList []dst.Stmt

	for _, stmt := range block.List {
		// Check if it's a return statement
		if _, ok := stmt.(*dst.ReturnStmt); ok {
			// Check if trace.Error already exists (prevent duplicates)
			if len(newList) > 0 && inj.isTraceErrorCall(newList[len(newList)-1], errVarName) {
				newList = append(newList, stmt)
				continue
			}

			// Create trace.Error(ctx, err) statement
			traceErrorStmt := &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent("trace"),
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

// isTraceErrorCall checks if statement is trace.Error(err) call
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

	return ident.Name == "trace" && sel.Sel.Name == "Error"
}

// isWhatapPackageCall checks if statement is a whatap package function call
// e.g., resp, err := whataphttp.HttpGet(ctx, url)
// e.g., db, err := whatapsql.Open("mysql", dsn)
func (inj *Injector) isWhatapPackageCall(stmt dst.Stmt) bool {
	// Check if AssignStmt (x, err := func() pattern)
	assignStmt, ok := stmt.(*dst.AssignStmt)
	if !ok {
		return false
	}

	// Check if right side is a function call
	if len(assignStmt.Rhs) != 1 {
		return false
	}

	call, ok := assignStmt.Rhs[0].(*dst.CallExpr)
	if !ok {
		return false
	}

	// Check if function is a selector (package.function form)
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	// Check package name
	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	// Check if it's a whatap package
	pkgName := ident.Name
	whatapPackages := []string{
		"whatapsql",
		"whatapsqlx",
		"whataphttp",
		"whatapredigo",
		"whatapgoredis",
		"whatapgorm",
		"whatapsarama",
		"whatapgin",
		"whatapecho",
		"whatapfiber",
		"whatapchi",
		"whatapmux",
		"whatapfasthttp",
		"whatapgrpc",
		"whatapkubernetes",
		"whatapmongo",
		"trace", // includes trace.Start, trace.StartMethod, etc.
	}

	for _, wp := range whatapPackages {
		if pkgName == wp {
			return true
		}
	}

	return false
}

// applyCustomRules applies custom rules (§4)
// Execution order: inject → replace → hook → transform
// (add is executed before file processing in InjectDir())
func (inj *Injector) applyCustomRules(file *dst.File, srcDir string) ([]string, error) {
	var changes []string

	if inj.Config == nil {
		return nil, nil
	}

	cfg := inj.Config.Custom

	// 1. inject rules: Insert code inside function definitions
	if len(cfg.Inject) > 0 {
		if err := custom.ApplyInjectRules(file, cfg.Inject, srcDir); err != nil {
			return changes, fmt.Errorf("inject rules: %w", err)
		}
		changes = append(changes, fmt.Sprintf("applied: %d inject rule(s)", len(cfg.Inject)))
	}

	// 2. replace rules: Replace function calls
	if len(cfg.Replace) > 0 {
		if err := custom.ApplyReplaceRules(file, cfg.Replace); err != nil {
			return changes, fmt.Errorf("replace rules: %w", err)
		}
		changes = append(changes, fmt.Sprintf("applied: %d replace rule(s)", len(cfg.Replace)))
	}

	// 3. hook rules: Insert before/after function calls
	if len(cfg.Hook) > 0 {
		if err := custom.ApplyHookRules(file, cfg.Hook); err != nil {
			return changes, fmt.Errorf("hook rules: %w", err)
		}
		changes = append(changes, fmt.Sprintf("applied: %d hook rule(s)", len(cfg.Hook)))
	}

	// 4. transform rules: Template-based transformation
	if len(cfg.Transform) > 0 {
		if err := custom.ApplyTransformRules(file, inj.Config.BaseDir, cfg.Transform); err != nil {
			return changes, fmt.Errorf("transform rules: %w", err)
		}
		changes = append(changes, fmt.Sprintf("applied: %d transform rule(s)", len(cfg.Transform)))
	}

	return changes, nil
}
