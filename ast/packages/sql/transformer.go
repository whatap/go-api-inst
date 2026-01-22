// Package sql provides the database/sql transformer.
package sql

import (
	"go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for database/sql.
type Transformer struct {
	transformed bool
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "sql"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "database/sql"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"
}

// Detect checks if the file uses database/sql.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// Inject transforms sql.Open to whatapsql.Open.
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false

	// Get the actual package name used in code (could be alias like "db")
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

			// pkgName.Open -> whatapsql.Open (pkgName could be "sql" or alias like "db")
			if ident.Name == pkgName && sel.Sel.Name == "Open" {
				ident.Name = "whatapsql"
				t.transformed = true
			}

			return true
		})
	}

	// Remove database/sql import if no longer used after transformation
	if t.transformed {
		common.RemoveImportIfUnused(file, t.ImportPath(), pkgName)
	}

	return t.transformed, nil
}

// Remove transforms whatapsql.Open back to sql.Open.
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

			// whatapsql.Open -> sql.Open
			if ident.Name == "whatapsql" && sel.Sel.Name == "Open" {
				ident.Name = "sql"
			}

			return true
		})
	}
	return nil
}
