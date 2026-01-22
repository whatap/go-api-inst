// Package redigo provides the github.com/gomodule/redigo transformer.
package redigo

import (
	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for redigo.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "redigo"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/gomodule/redigo/redis"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/github.com/gomodule/redigo/whatapredigo"
}

// Detect checks if the file uses redigo.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// dialFuncs is the list of redigo dial functions to transform.
var dialFuncs = map[string]bool{
	"Dial":           true,
	"DialContext":    true,
	"DialURL":        true,
	"DialURLContext": true,
}

// Inject transforms redis.Dial* to whatapredigo.Dial*.
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false

	// Get the actual package name used in code (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

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

			// pkgName.Dial* -> whatapredigo.Dial*
			if ident.Name == pkgName && dialFuncs[sel.Sel.Name] {
				ident.Name = "whatapredigo"
				t.transformed = true
			}

			return true
		})
	}

	// Remove original import if no longer used
	if t.transformed {
		common.RemoveImportIfUnused(file, t.ImportPath(), pkgName)
	}

	return t.transformed, nil
}

// Remove transforms whatapredigo.Dial* back to redis.Dial*.
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

			// whatapredigo.Dial* -> redis.Dial*
			if ident.Name == "whatapredigo" && dialFuncs[sel.Sel.Name] {
				ident.Name = "redis"
			}

			return true
		})
	}
	return nil
}
