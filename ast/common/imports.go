// Package common provides shared utilities for AST transformations.
package common

import (
	"go/token"
	"strconv"
	"strings"

	"github.com/dave/dst"
)

// AddImportWithAlias adds an import path with an alias to the file.
// Skips if already exists.
func AddImportWithAlias(file *dst.File, importPath, alias string) {
	path := strings.Trim(importPath, `"`)

	// Skip if already exists
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) == path {
			return
		}
	}

	newImport := &dst.ImportSpec{
		Path: &dst.BasicLit{
			Kind:  token.STRING,
			Value: strconv.Quote(path),
		},
	}

	// Add alias if provided
	if alias != "" {
		newImport.Name = &dst.Ident{Name: alias}
	}

	// Find import declaration
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}

		genDecl.Specs = append(genDecl.Specs, newImport)
		file.Imports = append(file.Imports, newImport)
		return
	}

	// Create new import declaration if none exists
	newDecl := &dst.GenDecl{
		Tok:   token.IMPORT,
		Specs: []dst.Spec{newImport},
	}
	file.Decls = append([]dst.Decl{newDecl}, file.Decls...)
	file.Imports = append(file.Imports, newImport)
}

// AddImport adds an import path to the file.
// Skips if already exists.
func AddImport(file *dst.File, importPath string) {
	// Remove quotes
	path := strings.Trim(importPath, `"`)

	// Skip if already exists
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) == path {
			return
		}
	}

	newImport := &dst.ImportSpec{
		Path: &dst.BasicLit{
			Kind:  token.STRING,
			Value: strconv.Quote(path),
		},
	}

	// Find import declaration
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.IMPORT {
			continue
		}

		genDecl.Specs = append(genDecl.Specs, newImport)
		file.Imports = append(file.Imports, newImport)
		return
	}

	// Create new import declaration if none exists
	newDecl := &dst.GenDecl{
		Tok:   token.IMPORT,
		Specs: []dst.Spec{newImport},
	}
	file.Decls = append([]dst.Decl{newDecl}, file.Decls...)
	file.Imports = append(file.Imports, newImport)
}

// RemoveImport removes an import path from the file.
func RemoveImport(file *dst.File, importPath string) {
	path := strings.Trim(importPath, `"`)

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

			impPath := strings.Trim(imp.Path.Value, `"`)
			if impPath != path {
				newSpecs = append(newSpecs, spec)
			}
		}
		genDecl.Specs = newSpecs
	}

	// Update file.Imports as well
	var newImports []*dst.ImportSpec
	for _, imp := range file.Imports {
		impPath := strings.Trim(imp.Path.Value, `"`)
		if impPath != path {
			newImports = append(newImports, imp)
		}
	}
	file.Imports = newImports
}

// HasImport checks if a specific import path exists.
func HasImport(file *dst.File, importPath string) bool {
	path := strings.Trim(importPath, `"`)
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) == path {
			return true
		}
	}
	return false
}

// HasImportPrefix checks if an import starting with a specific prefix exists.
func HasImportPrefix(file *dst.File, prefix string) bool {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// HasWhatapImport checks if whatap/go-api import exists.
func HasWhatapImport(file *dst.File) bool {
	return HasImportPrefix(file, "github.com/whatap/go-api")
}

// RemoveWhatapImports removes all whatap-related imports.
func RemoveWhatapImports(file *dst.File) {
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

	// Update file.Imports as well
	var newImports []*dst.ImportSpec
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if !strings.Contains(path, "whatap/go-api") {
			newImports = append(newImports, imp)
		}
	}
	file.Imports = newImports
}

// RemoveUnusedImport removes an unused import from the code.
// packageName is the imported package name (e.g., "context")
func RemoveUnusedImport(file *dst.File, packageName string) {
	// Check if the package is used in the code
	used := false

	dst.Inspect(file, func(n dst.Node) bool {
		sel, ok := n.(*dst.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			return true
		}

		if ident.Name == packageName {
			used = true
			return false
		}
		return true
	})

	if used {
		return
	}

	// Remove import if unused
	RemoveImport(file, packageName)
}

// GetImportAlias returns the alias of an import. Returns empty string if no alias.
func GetImportAlias(file *dst.File, importPath string) string {
	path := strings.Trim(importPath, `"`)
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) == path {
			if imp.Name != nil {
				return imp.Name.Name
			}
			return ""
		}
	}
	return ""
}

// GetPackageNameForImport returns the package name to use in code for the given import path.
// If the import has an alias, returns the alias. Otherwise returns the last segment of the path.
// Returns empty string if the import is not found.
// Example: "context" -> "context" or "ctx" (if aliased)
// Example: "net/http" -> "http" or "myhttp" (if aliased)
func GetPackageNameForImport(file *dst.File, importPath string) string {
	path := strings.Trim(importPath, `"`)
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) == path {
			if imp.Name != nil {
				return imp.Name.Name
			}
			// Return default package name (last segment of path)
			parts := strings.Split(path, "/")
			return parts[len(parts)-1]
		}
	}
	return ""
}

// GetPackageNameForImportPrefix returns the package name for imports matching a prefix.
// Useful for packages with version suffixes like github.com/labstack/echo (v3) and echo/v4.
// Returns empty string if no matching import is found.
func GetPackageNameForImportPrefix(file *dst.File, importPrefix string) string {
	prefix := strings.Trim(importPrefix, `"`)
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.HasPrefix(path, prefix) {
			if imp.Name != nil {
				return imp.Name.Name
			}
			// Return default package name (last segment before version or end)
			// e.g., github.com/labstack/echo/v4 -> echo
			parts := strings.Split(path, "/")
			for i := len(parts) - 1; i >= 0; i-- {
				// Skip version suffixes like v2, v4, v5
				if len(parts[i]) >= 2 && parts[i][0] == 'v' && parts[i][1] >= '0' && parts[i][1] <= '9' {
					continue
				}
				return parts[i]
			}
			return parts[len(parts)-1]
		}
	}
	return ""
}

// GetContextPackageName returns the package name to use for context in code.
// Returns "context" if context is imported normally, or the alias if aliased.
// Returns empty string if context is not imported.
func GetContextPackageName(file *dst.File) string {
	return GetPackageNameForImport(file, "context")
}

// ExtractVersion extracts version from import path (e.g., /v2, /v4)
func ExtractVersion(importPath string) string {
	parts := strings.Split(importPath, "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if len(last) >= 2 && last[0] == 'v' && last[1] >= '0' && last[1] <= '9' {
			return last
		}
	}
	return ""
}

// IsPackageUsed checks if a package identifier is used in the file.
// packageName is the identifier used in code (e.g., "http", "fmt", "sql", "redis")
func IsPackageUsed(file *dst.File, packageName string) bool {
	used := false
	dst.Inspect(file, func(n dst.Node) bool {
		sel, ok := n.(*dst.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			return true
		}

		if ident.Name == packageName {
			used = true
			return false // Stop inspection
		}
		return true
	})
	return used
}

// RemoveImportIfUnused removes an import if the package is not used in the file.
// importPath is the full import path (e.g., "net/http", "database/sql")
// packageName is the identifier used in code (e.g., "http", "sql")
// Also cleans up empty import blocks.
func RemoveImportIfUnused(file *dst.File, importPath, packageName string) {
	if IsPackageUsed(file, packageName) {
		return
	}

	// Remove the import
	RemoveImport(file, importPath)

	// Clean up empty import declarations
	CleanupEmptyImports(file)
}

// CleanupEmptyImports removes empty import declaration blocks from the file.
func CleanupEmptyImports(file *dst.File) {
	newDecls := make([]dst.Decl, 0, len(file.Decls))
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok {
			newDecls = append(newDecls, decl)
			continue
		}

		// Skip empty import blocks
		if genDecl.Tok == token.IMPORT && len(genDecl.Specs) == 0 {
			continue
		}
		newDecls = append(newDecls, decl)
	}
	file.Decls = newDecls
}

// CleanupAllUnusedImports removes all unused imports from the file.
// This should be called at the end of injection after all transformers have run.
// It checks each import and removes it if the package is not used in the code.
func CleanupAllUnusedImports(file *dst.File) {
	// Collect imports to check (copy to avoid modifying while iterating)
	importsToCheck := make([]*dst.ImportSpec, len(file.Imports))
	copy(importsToCheck, file.Imports)

	for _, imp := range importsToCheck {
		path := strings.Trim(imp.Path.Value, `"`)

		// Skip blank imports (e.g., _ "github.com/lib/pq")
		if imp.Name != nil && imp.Name.Name == "_" {
			continue
		}

		// Skip dot imports (e.g., . "math")
		if imp.Name != nil && imp.Name.Name == "." {
			continue
		}

		// Get package name used in code
		var pkgName string
		if imp.Name != nil {
			pkgName = imp.Name.Name
		} else {
			pkgName = getDefaultPackageName(path)
		}

		// Remove import if package is not used
		if !IsPackageUsed(file, pkgName) {
			RemoveImport(file, path)
		}
	}

	// Clean up empty import blocks
	CleanupEmptyImports(file)
}

// getDefaultPackageName extracts the default package name from an import path.
// Handles version suffixes like /v2, /v9 - uses the segment before the version.
// Examples:
//   - "github.com/redis/go-redis/v9" → "redis" (from go-redis)
//   - "github.com/go-redis/redis/v8" → "redis"
//   - "gorm.io/gorm" → "gorm"
//   - "net/http" → "http"
func getDefaultPackageName(importPath string) string {
	parts := strings.Split(importPath, "/")
	if len(parts) == 0 {
		return ""
	}

	last := parts[len(parts)-1]

	// Check if last segment is a version suffix (v2, v3, ..., v9, v10, etc.)
	if isVersionSuffix(last) && len(parts) >= 2 {
		// Use the segment before version
		beforeVersion := parts[len(parts)-2]
		// Handle cases like "go-redis" → "redis"
		if strings.HasPrefix(beforeVersion, "go-") {
			return strings.TrimPrefix(beforeVersion, "go-")
		}
		return beforeVersion
	}

	return last
}

// isVersionSuffix checks if a string is a Go module version suffix (v2, v3, etc.)
func isVersionSuffix(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	// Check if rest is a number
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
