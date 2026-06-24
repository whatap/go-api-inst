// Package common provides shared utilities for AST transformations.
package common

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gotypes"
)

// typeContextData holds go/types info for the current file being processed.
// Set by injector before transformer calls, cleared after each file.
// Not goroutine-safe — file processing is sequential (current design).
type typeContextData struct {
	typesInfo *types.Info
	nodeMap   map[dst.Node]ast.Node // dst → ast node mapping from decorator
}

var typeCtx typeContextData

// currentImportPath is the import path of the package currently being processed.
// Set alongside SetTypeContext (or via SetCurrentImportPath). Empty when unknown.
// Used by resolveFuncDeclTarget to emit "decl:pkgpath.FuncName" targets.
var currentImportPath string

// SetTypeContext sets the type context for the current file.
// typesInfo: go/types info from packages.Load (can be nil)
// nodeMap: dst→ast node mapping from decorator.Ast.Nodes (can be nil)
func SetTypeContext(typesInfo *types.Info, nodeMap map[dst.Node]ast.Node) {
	typeCtx.typesInfo = typesInfo
	typeCtx.nodeMap = nodeMap
}

// ClearTypeContext clears the type context after processing a file.
func ClearTypeContext() {
	typeCtx.typesInfo = nil
	typeCtx.nodeMap = nil
	currentImportPath = ""
}

// SetCurrentImportPath records the import path of the package currently being processed.
func SetCurrentImportPath(p string) {
	currentImportPath = p
}

// GetCurrentImportPath returns the import path of the package currently being processed,
// or "" when unknown.
func GetCurrentImportPath() string {
	return currentImportPath
}

// HasTypeInfo returns true if go/types info is available for the current file.
func HasTypeInfo() bool {
	return typeCtx.typesInfo != nil && typeCtx.nodeMap != nil
}

// GetIdentPath returns the import path of a dst.Ident node.
// Uses go/types Uses map to resolve the identifier to its imported package path.
//   - Package identifiers: "net/http", "database/sql", etc.
//   - Local variables/functions: "" (empty string)
//
// Returns empty string if type info is not available or ident is not a package reference.
func GetIdentPath(ident *dst.Ident) string {
	if ident == nil || typeCtx.typesInfo == nil || typeCtx.nodeMap == nil {
		return ""
	}
	astNode, ok := typeCtx.nodeMap[ident]
	if !ok {
		return ""
	}
	astIdent, ok := astNode.(*ast.Ident)
	if !ok {
		return ""
	}
	obj := typeCtx.typesInfo.Uses[astIdent]
	if obj == nil {
		return ""
	}
	if pkgName, ok := obj.(*types.PkgName); ok {
		return pkgName.Imported().Path()
	}
	return ""
}

// GetIdentFuncTarget resolves an identifier-only call expression to a target
// string of the form "<importpath>.<funcname>" when the identifier refers to
// a function (local or dot-imported). Returns "" if go/types is unavailable,
// the identifier doesn't refer to a function, or the function has no package
// (e.g. builtins like len, make).
//
// Used by §227 Step 5 to let hook/transform/inject custom rules target local
// functions called by their bare name (e.g. `fetchData()` inside package main).
func GetIdentFuncTarget(ident *dst.Ident) string {
	if ident == nil || typeCtx.typesInfo == nil || typeCtx.nodeMap == nil {
		return ""
	}
	astNode, ok := typeCtx.nodeMap[ident]
	if !ok {
		return ""
	}
	astIdent, ok := astNode.(*ast.Ident)
	if !ok {
		return ""
	}
	obj := typeCtx.typesInfo.Uses[astIdent]
	if obj == nil {
		return ""
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return ""
	}
	pkg := fn.Pkg()
	if pkg == nil {
		// Builtins (len, make, ...) live in the universe scope and have no Pkg.
		return ""
	}
	return pkg.Path() + "." + fn.Name()
}

// ResolveType returns the actual Go type of a dst expression.
// Uses dst→ast mapping and go/types to resolve.
// Returns nil if type info is not available or resolution fails.
//
// Example:
//
//	sel := call.Fun.(*dst.SelectorExpr)
//	if t := common.ResolveType(sel.X); t != nil {
//	    // t is the receiver's actual type, e.g. *aerospike.Client
//	}
func ResolveType(expr dst.Expr) types.Type {
	if typeCtx.typesInfo == nil || typeCtx.nodeMap == nil {
		return nil
	}

	astNode, ok := typeCtx.nodeMap[expr]
	if !ok {
		return nil
	}

	astExpr, ok := astNode.(ast.Expr)
	if !ok {
		return nil
	}

	// 1. Types map — works for most expressions
	if tv, ok := typeCtx.typesInfo.Types[astExpr]; ok {
		return tv.Type
	}

	// 2. Uses map — works for identifiers referencing other declarations
	if astIdent, ok := astExpr.(*ast.Ident); ok {
		if obj := typeCtx.typesInfo.Uses[astIdent]; obj != nil {
			return obj.Type()
		}
	}

	// 3. SelectorExpr.Sel — try the selector's identifier
	if selExpr, ok := astExpr.(*ast.SelectorExpr); ok {
		if obj := typeCtx.typesInfo.Uses[selExpr.Sel]; obj != nil {
			return obj.Type()
		}
	}

	return nil
}

// IsReceiverOfType checks if the expression's resolved type matches pkgPath and typeName.
// Automatically dereferences pointer types (e.g., *http.ServeMux → http.ServeMux).
// Returns false if type info is not available or type doesn't match.
//
// Example:
//
//	// Check if sel.X is *http.ServeMux
//	if common.IsReceiverOfType(sel.X, "net/http", "ServeMux") { ... }
//
//	// Check if sel.X is *aerospike.Client
//	if common.IsReceiverOfType(sel.X, "github.com/aerospike/aerospike-client-go/v6", "Client") { ... }
func IsReceiverOfType(expr dst.Expr, pkgPath, typeName string) bool {
	if !HasTypeInfo() {
		return false
	}
	p, n, ok := NamedTypeOf(expr)
	return ok && p == pkgPath && n == typeName
}

// NamedTypeOf resolves expr's type and returns the origin named type's package
// path and type name, dereferencing pointers. For instantiated generic types
// (e.g. compose.Chain[string, *schema.Message]) it returns the *generic* type's
// path/name (compose, "Chain") — the type arguments are dropped, because
// types.Named.Obj() reports the generic type's TypeName for instances.
//
// Returns ok=false when type info is unavailable, the type is unnamed, or the
// package is nil (builtins like error). This is the single place that maps a
// receiver expression to a {pkgPath, typeName} pair; resolveMethodTarget,
// resolveFuncDeclMethodTarget, and IsReceiverOfType all go through it so that
// generic receivers are handled identically everywhere.
func NamedTypeOf(expr dst.Expr) (pkgPath, typeName string, ok bool) {
	if !HasTypeInfo() {
		return "", "", false
	}
	t := ResolveType(expr)
	if t == nil {
		return "", "", false
	}
	// Dereference pointer
	if ptr, isPtr := t.(*types.Pointer); isPtr {
		t = ptr.Elem()
	}
	named, isNamed := t.(*types.Named)
	if !isNamed {
		return "", "", false
	}
	pkg := named.Obj().Pkg()
	if pkg == nil {
		return "", "", false
	}
	return pkg.Path(), named.Obj().Name(), true
}

// TrySetupTypeContext tries to load type info and set up the type context.
// Returns the decorated dst.File if successful, nil otherwise (caller should fallback).
//
// On success: HasTypeInfo() == true, GetIdentPath/ResolveType available.
// On failure: HasTypeInfo() == false, caller should use decorator.Parse() instead.
//
// Uses NewDecorator (NOT NewDecoratorFromPackage) to avoid import management.
// Import management causes issues with restoration because our transformers
// manage imports manually. The dst↔ast node mapping still works correctly,
// enabling GetIdentPath and ResolveType to resolve types via TypesInfo.
func TrySetupTypeContext(tc *TypeChecker, srcPath string) *dst.File {
	dir := filepath.Dir(srcPath)

	// §169: Evict previous directory's cache to allow GC.
	// filepath.Walk processes files in lexicographic order,
	// so same-directory files are consecutive — cache hit preserved.
	tc.EvictExcept(dir)

	pkg, err := tc.LoadPackage(dir)
	if err != nil || pkg == nil {
		ClearTypeContext()
		return nil
	}

	// Defensive: ensure required fields are populated
	if pkg.TypesInfo == nil || pkg.Fset == nil || len(pkg.Syntax) == 0 {
		ClearTypeContext()
		return nil
	}

	absPath, absErr := filepath.Abs(srcPath)
	if absErr != nil {
		ClearTypeContext()
		return nil
	}

	// Find the ast.File matching srcPath in the loaded package
	for _, f := range pkg.Syntax {
		pos := pkg.Fset.Position(f.Pos())
		if filepath.Clean(pos.Filename) == filepath.Clean(absPath) {
			// §163: Use NewDecorator (no import management) instead of NewDecoratorFromPackage.
			// NewDecoratorFromPackage enables import management which collapses SelectorExpr
			// (e.g., gin.Engine → Ident{Name:"Engine",Path:"github.com/gin-gonic/gin"}).
			// This requires NewRestorerWithImports for restoration, which conflicts with
			// our transformers' manual import management.
			// NewDecorator preserves the original AST structure and still provides
			// dst↔ast node mapping for ResolveType and GetIdentPath (via TypesInfo.Uses).
			dec := decorator.NewDecorator(pkg.Fset)
			dstFile, decErr := dec.DecorateFile(f)
			if decErr != nil {
				ClearTypeContext()
				return nil
			}
			SetTypeContext(pkg.TypesInfo, dec.Ast.Nodes)
			SetCurrentImportPath(pkg.PkgPath)
			return dstFile
		}
	}

	// File not in package (excluded by build tags, test file, etc.)
	ClearTypeContext()
	return nil
}

// MatchIdentPkg checks if ident refers to the specified package.
// With go/types: uses GetIdentPath() for precise matching (alias-aware, variable-safe).
// Without go/types: falls back to ident.Name == pkgName (existing behavior).
//
// importPrefix is the base import path without version suffix.
// Versioned imports (e.g., "github.com/labstack/echo/v4") are matched automatically
// when importPrefix is "github.com/labstack/echo".
//
// Returns false for variables/functions when go/types is available (GetIdentPath returns "").
func MatchIdentPkg(ident *dst.Ident, pkgName, importPrefix string) bool {
	if HasTypeInfo() {
		identPath := GetIdentPath(ident)
		if identPath == "" {
			return false // variable, not package
		}
		if identPath == importPrefix {
			return true
		}
		// Versioned: "github.com/labstack/echo/v4" matches prefix "github.com/labstack/echo"
		// But reject subpackages: "github.com/gofiber/fiber/v2/middleware/logger" must NOT match
		if !strings.HasPrefix(identPath, importPrefix+"/") {
			return false
		}
		remainder := identPath[len(importPrefix)+1:]
		return isVersionSuffix(remainder)
	}
	return ident.Name == pkgName
}

// MatchCallPkg extracts ident and funcName from a CallExpr and checks package match.
// Returns (ident, funcName, matched).
//
// The returned *dst.Ident is needed for rename-pattern transformers (e.g., ident.Name = "whatapsql").
// funcName is the selector name (e.g., "Open" from sql.Open).
//
// Only matches pkg.Func() patterns (SelectorExpr with Ident receiver).
// Returns (nil, "", false) for non-matching patterns.
func MatchCallPkg(call *dst.CallExpr, pkgName, importPrefix string) (*dst.Ident, string, bool) {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return nil, "", false
	}
	return ident, sel.Sel.Name, MatchIdentPkg(ident, pkgName, importPrefix)
}

// --- importcfg-based type checking for toolexec (fast) mode ---
// Uses importer.ForCompiler instead of packages.Load to avoid nested build panics.
// Reference: orchestrion internal/injector/typed/typecheck.go

// importcfgTypeCache caches type info loaded from importcfg for the current package.
// In toolexec mode, processCompileArgs calls SetupImportcfgTypeInfo once per package,
// then TrySetupTypeContextFromImportcfg per file.
var importcfgTypeCache struct {
	fset       *token.FileSet
	typesInfo  *types.Info
	astFiles   map[string]*ast.File // absolute path → ast.File
	importPath string               // current package import path (from TOOLEXEC_IMPORTPATH)
	entries    map[string]string    // import path → .a archive path (for lookupResolver)
	valid      bool
}

// SetupImportcfgTypeInfo pre-loads type info for all goFiles using importcfg.
// Called once per package in toolexec mode. Uses importer.ForCompiler to read
// type info from already-compiled .a archives (dependencies compile before dependents).
//
// Parameters:
//   - importcfgEntries: packagefile map from readImportCfg (import path → .a path)
//   - goFiles: all .go files being compiled in this package
//   - importPath: the import path of the current package (from TOOLEXEC_IMPORTPATH)
//   - debug: enable debug output
func SetupImportcfgTypeInfo(importcfgEntries map[string]string, goFiles []string, importPath string, debug bool) error {
	importcfgTypeCache.valid = false

	if len(importcfgEntries) == 0 || len(goFiles) == 0 {
		return fmt.Errorf("importcfg entries or goFiles empty")
	}

	// Build lookup function from importcfg packagefile entries.
	// importer.ForCompiler calls this to resolve import paths to .a archive files.
	lookup := func(path string) (io.ReadCloser, error) {
		if archivePath, ok := importcfgEntries[path]; ok {
			return os.Open(archivePath)
		}
		return nil, fmt.Errorf("package %s not found in importcfg", path)
	}

	// Parse all go files
	fset := token.NewFileSet()
	var astFileList []*ast.File
	astFileMap := make(map[string]*ast.File)

	for _, goFile := range goFiles {
		f, err := parser.ParseFile(fset, goFile, nil, parser.ParseComments)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] importcfg typecheck: parse error %s: %v\n", goFile, err)
			}
			continue
		}
		astFileList = append(astFileList, f)
		absPath, _ := filepath.Abs(goFile)
		astFileMap[filepath.Clean(absPath)] = f
	}

	if len(astFileList) == 0 {
		return fmt.Errorf("no parseable files")
	}

	// Type-check with importer.ForCompiler (reads .a files, no go list, no nested build)
	typesInfo := &types.Info{
		Types:  make(map[ast.Expr]types.TypeAndValue),
		Uses:   make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope),
	}

	cfg := &types.Config{
		Importer: importer.ForCompiler(fset, runtime.Compiler, lookup),
		Error:    func(err error) {}, // ignore type errors — don't break injection
	}

	pkgName := astFileList[0].Name.Name
	if importPath == "" {
		importPath = pkgName
	}
	pkg := types.NewPackage(importPath, pkgName)
	checker := types.NewChecker(cfg, fset, pkg, typesInfo)

	// Check all files together (ignore errors)
	if err := checker.Files(astFileList); err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] importcfg typecheck: checker error (non-fatal): %v\n", err)
		}
	}

	importcfgTypeCache.fset = fset
	importcfgTypeCache.typesInfo = typesInfo
	importcfgTypeCache.astFiles = astFileMap
	importcfgTypeCache.importPath = importPath
	importcfgTypeCache.entries = importcfgEntries
	importcfgTypeCache.valid = true

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] importcfg typecheck: loaded %d files, %d type entries\n",
			len(astFileList), len(typesInfo.Types))
	}

	return nil
}

// TrySetupTypeContextFromImportcfg sets up type context for a single file
// using cached importcfg type info. Returns the decorated dst.File if successful.
// Called per file in toolexec mode after SetupImportcfgTypeInfo.
func TrySetupTypeContextFromImportcfg(srcPath string) *dst.File {
	if !importcfgTypeCache.valid {
		return nil
	}

	absPath, err := filepath.Abs(srcPath)
	if err != nil {
		return nil
	}

	astFile, ok := importcfgTypeCache.astFiles[filepath.Clean(absPath)]
	if !ok {
		return nil
	}

	dec := decorator.NewDecorator(importcfgTypeCache.fset)
	dstFile, err := dec.DecorateFile(astFile)
	if err != nil {
		return nil
	}

	SetTypeContext(importcfgTypeCache.typesInfo, dec.Ast.Nodes)
	SetCurrentImportPath(importcfgTypeCache.importPath)
	return dstFile
}

// HasImportcfgTypeCache returns true if importcfg type cache is loaded.
func HasImportcfgTypeCache() bool {
	return importcfgTypeCache.valid
}

// TrySetupDecoratorContextFromImportcfg sets up a decorator context with ident.Path
// using cached importcfg type info. Uses gotypes.New(Uses) to create a resolver
// that populates ident.Path on all identifiers (Datadog orchestrion approach).
// Returns the decorated dst.File with ident.Path populated, or nil on failure.
func TrySetupDecoratorContextFromImportcfg(srcPath string) *dst.File {
	if !importcfgTypeCache.valid || importcfgTypeCache.typesInfo == nil {
		return nil
	}

	absPath, err := filepath.Abs(srcPath)
	if err != nil {
		return nil
	}

	astFile, ok := importcfgTypeCache.astFiles[filepath.Clean(absPath)]
	if !ok {
		return nil
	}

	// Use gotypes resolver: go/types Uses map → ident.Path auto-populated
	resolver := gotypes.New(importcfgTypeCache.typesInfo.Uses)
	dec := decorator.NewDecoratorWithImports(importcfgTypeCache.fset, importcfgTypeCache.importPath, resolver)
	dstFile, err := dec.DecorateFile(astFile)
	if err != nil {
		return nil
	}

	// Also set go/types context for method call resolution (ResolveType)
	SetTypeContext(importcfgTypeCache.typesInfo, dec.Ast.Nodes)
	SetCurrentImportPath(importcfgTypeCache.importPath)
	return dstFile
}

// GetImportcfgPackageNames builds a map of import path → package name
// from the importcfg type cache. Used by Restorer (guess.WithMap) to resolve
// package names correctly for paths like "github.com/labstack/echo/v4" → "echo".
func GetImportcfgPackageNames() map[string]string {
	if !importcfgTypeCache.valid || importcfgTypeCache.typesInfo == nil {
		return nil
	}
	m := make(map[string]string)
	for _, obj := range importcfgTypeCache.typesInfo.Uses {
		if pn, ok := obj.(*types.PkgName); ok {
			m[pn.Imported().Path()] = pn.Imported().Name()
		}
	}
	return m
}

// GetImportcfgLookup returns a lookup function that resolves import paths to .a archive readers.
// Used by lookupResolver to read package names from compiled archives (Datadog orchestrion approach).
// Returns nil if importcfg type cache is not available.
func GetImportcfgLookup() func(path string) (io.ReadCloser, error) {
	if !importcfgTypeCache.valid || importcfgTypeCache.entries == nil {
		return nil
	}
	entries := importcfgTypeCache.entries
	return func(path string) (io.ReadCloser, error) {
		if archivePath, ok := entries[path]; ok {
			return os.Open(archivePath)
		}
		return nil, fmt.Errorf("package %s not found in importcfg", path)
	}
}

// GetImportcfgImportPath returns the current package's import path from the importcfg cache.
// Used by Restorer to set the package path for import management.
func GetImportcfgImportPath() string {
	if !importcfgTypeCache.valid {
		return ""
	}
	return importcfgTypeCache.importPath
}

// ClearImportcfgTypeCache clears the importcfg type cache.
// Called after processCompileArgs finishes processing a package.
func ClearImportcfgTypeCache() {
	importcfgTypeCache.fset = nil
	importcfgTypeCache.typesInfo = nil
	importcfgTypeCache.astFiles = nil
	importcfgTypeCache.importPath = ""
	importcfgTypeCache.entries = nil
	importcfgTypeCache.valid = false
}
