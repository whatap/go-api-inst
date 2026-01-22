// Package goredis provides the go-redis transformer.
package goredis

import (
	"strings"

	"go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for go-redis.
type Transformer struct{}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "goredis"
}

// ImportPath returns the original package import path.
// Note: go-redis has multiple import paths (v9: github.com/redis/go-redis, v8: github.com/go-redis/redis)
func (t *Transformer) ImportPath() string {
	return "github.com/redis/go-redis"
}

// WhatapImport returns empty because goredis handles import in Inject() based on version.
func (t *Transformer) WhatapImport() string {
	// Different imports needed for v8/v9, so handled in Inject()
	return ""
}

// WhatapImportForFile returns the correct whatap import path based on the file's imports.
func (t *Transformer) WhatapImportForFile(file *dst.File) string {
	if t.isV8(file) {
		return "github.com/whatap/go-api/instrumentation/github.com/go-redis/redis/v8/whatapgoredis"
	}
	return "github.com/whatap/go-api/instrumentation/github.com/redis/go-redis/v9/whatapgoredis"
}

// isV8 checks if the file uses go-redis v8.
func (t *Transformer) isV8(file *dst.File) bool {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		// github.com/go-redis/redis/v8 (old path)
		if strings.HasPrefix(path, "github.com/go-redis/redis") {
			return true
		}
	}
	return false
}

// Detect checks if the file uses go-redis (both v8 and v9 paths).
func (t *Transformer) Detect(file *dst.File) bool {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		// github.com/redis/go-redis/v9 (new path)
		if strings.HasPrefix(path, "github.com/redis/go-redis") {
			return true
		}
		// github.com/go-redis/redis/v8 (old path)
		if strings.HasPrefix(path, "github.com/go-redis/redis") {
			return true
		}
	}
	return false
}

// redisFuncs is the list of go-redis functions to transform.
var redisFuncs = map[string]bool{
	"NewClient":         true,
	"NewClusterClient":  true,
	"NewFailoverClient": true,
	"NewRing":           true,
}

// Inject transforms redis.NewClient* to whatapgoredis.NewClient*.
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	transformed := false

	// Get the actual package name used in code (could be alias)
	// Check both v9 (github.com/redis/go-redis) and v8 (github.com/go-redis/redis) paths
	pkgName := common.GetPackageNameForImportPrefix(file, "github.com/redis/go-redis")
	importPath := "github.com/redis/go-redis/v9"
	if pkgName == "" {
		pkgName = common.GetPackageNameForImportPrefix(file, "github.com/go-redis/redis")
		importPath = "github.com/go-redis/redis/v8"
	}
	if pkgName == "" {
		return false, nil
	}
	// Get actual import path for RemoveImportIfUnused
	importPath = t.getActualImportPath(file)

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

			sel, ok := call.Fun.(*dst.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := sel.X.(*dst.Ident)
			if !ok {
				return true
			}

			// pkgName.NewClient* -> whatapgoredis.NewClient*
			if ident.Name == pkgName && redisFuncs[sel.Sel.Name] {
				ident.Name = "whatapgoredis"
				transformed = true
			}

			return true
		})
	}

	// Add correct whatap import based on v8/v9
	if transformed {
		common.AddImport(file, t.WhatapImportForFile(file))
		// Remove original import if no longer used
		common.RemoveImportIfUnused(file, importPath, pkgName)
	}

	return transformed, nil
}

// getActualImportPath returns the actual import path used in the file.
func (t *Transformer) getActualImportPath(file *dst.File) string {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.HasPrefix(path, "github.com/redis/go-redis") ||
			strings.HasPrefix(path, "github.com/go-redis/redis") {
			return path
		}
	}
	return ""
}

// Remove transforms whatapgoredis.NewClient* back to redis.NewClient*.
func (t *Transformer) Remove(file *dst.File) error {
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

			sel, ok := call.Fun.(*dst.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := sel.X.(*dst.Ident)
			if !ok {
				return true
			}

			// whatapgoredis.NewClient* -> redis.NewClient*
			if ident.Name == "whatapgoredis" && redisFuncs[sel.Sel.Name] {
				ident.Name = "redis"
			}

			return true
		})
	}
	return nil
}
