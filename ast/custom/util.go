// Package custom handles custom instrumentation rules
package custom

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// parseCodeBlock parses a Go code string into DST statements
func parseCodeBlock(code string) ([]dst.Stmt, error) {
	if code == "" {
		return nil, nil
	}

	// Wrap in function to parse as statements
	wrapped := "package p\nfunc f() {\n" + code + "\n}"

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", wrapped, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Convert to DST
	dstFile, err := decorator.DecorateFile(fset, f)
	if err != nil {
		return nil, err
	}

	// Extract statements from function body
	fn := dstFile.Decls[0].(*dst.FuncDecl)
	return fn.Body.List, nil
}

// matchesFunction checks if function name matches the pattern
// Patterns:
//   - "*" : all functions
//   - "Handle*" : starts with Handle (prefix match)
//   - "*Handler" : ends with Handler (suffix match)
//   - "Get*DB" : starts with Get and ends with DB (prefix + suffix match)
//   - "ProcessOrder" : exact match
func matchesFunction(funcName, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}

	// Pattern with * in the middle: "Get*DB" → prefix="Get", suffix="DB"
	if idx := strings.Index(pattern, "*"); idx != -1 {
		prefix := pattern[:idx]
		suffix := pattern[idx+1:]

		// Match prefix only if suffix is empty (e.g., "Handle*")
		if suffix == "" {
			return strings.HasPrefix(funcName, prefix)
		}
		// Match suffix only if prefix is empty (e.g., "*Handler")
		if prefix == "" {
			return strings.HasSuffix(funcName, suffix)
		}
		// Match both prefix and suffix (e.g., "Get*DB")
		return strings.HasPrefix(funcName, prefix) && strings.HasSuffix(funcName, suffix)
	}

	return funcName == pattern
}

// getImportAlias returns the alias of an import path in the file
// Returns empty string if not found
func getImportAlias(file *dst.File, importPath string) string {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == importPath {
			if imp.Name != nil {
				return imp.Name.Name
			}
			// Use last path component as default alias
			parts := strings.Split(path, "/")
			return parts[len(parts)-1]
		}
	}
	return ""
}

// hasImportPath checks if the file has the specified import
func hasImportPath(file *dst.File, importPath string) bool {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == importPath {
			return true
		}
	}
	return false
}

// parseQualifiedName splits "pkg.Func" format into (pkg, Func)
func parseQualifiedName(name string) (pkg, fn string) {
	parts := strings.SplitN(name, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", name
}

// packageToPath converts Go package path to filesystem path
func packageToPath(pkgPath string) string {
	return strings.ReplaceAll(pkgPath, "/", string(filepath.Separator))
}

// matchesPackageOrFile checks if file matches package or file pattern
// pkgPattern: file's package name (e.g., "main", "user", "order")
// filePattern: file name pattern (e.g., "*.go", "handler_*.go")
func matchesPackageOrFile(file *dst.File, pkgPattern, filePattern, srcPath string) bool {
	// If package pattern is specified
	if pkgPattern != "" {
		// Compare with file's actual package name
		pkgName := file.Name.Name
		if !matchesFunction(pkgName, pkgPattern) {
			return false
		}
	}

	// If file pattern is specified
	if filePattern != "" {
		matched, _ := filepath.Match(filePattern, filepath.Base(srcPath))
		if !matched {
			return false
		}
	}

	// Return true if no pattern or all patterns match
	return true
}

// containsCall checks if statement contains a specific package.function call
// Supported patterns:
//   - pkg.Func()                        : ExprStmt
//   - err := pkg.Func()                 : AssignStmt
//   - if err := pkg.Func(); err != nil  : IfStmt.Init
//   - switch pkg.Func() { ... }         : SwitchStmt.Tag
func containsCall(stmt dst.Stmt, pkgAlias, funcName string) bool {
	switch s := stmt.(type) {
	case *dst.ExprStmt:
		// Simple call: pkg.Func()
		return isPkgCallExpr(s.X, pkgAlias, funcName)

	case *dst.AssignStmt:
		// Assignment: err := pkg.Func()
		if len(s.Rhs) > 0 {
			return isPkgCallExpr(s.Rhs[0], pkgAlias, funcName)
		}

	case *dst.IfStmt:
		// if initialization: if err := pkg.Func(); err != nil { }
		if s.Init != nil {
			if assign, ok := s.Init.(*dst.AssignStmt); ok {
				if len(assign.Rhs) > 0 {
					return isPkgCallExpr(assign.Rhs[0], pkgAlias, funcName)
				}
			}
		}

	case *dst.SwitchStmt:
		// switch condition: switch pkg.Func() { ... }
		if s.Tag != nil {
			return isPkgCallExpr(s.Tag, pkgAlias, funcName)
		}
	}

	return false
}

// isPkgCallExpr checks if expression is a package.function call (supports wildcard pattern)
func isPkgCallExpr(expr dst.Expr, pkgAlias, funcPattern string) bool {
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

	return ident.Name == pkgAlias && matchesFunction(sel.Sel.Name, funcPattern)
}
