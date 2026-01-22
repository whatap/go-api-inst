// Package sqlx provides the sqlx transformer.
package sqlx

import (
	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for sqlx.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "sqlx"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/jmoiron/sqlx"
}

// WhatapImport returns empty string because sqlx transformer adds import during Inject().
func (t *Transformer) WhatapImport() string {
	return "" // Import is added during Inject() if transformation occurs
}

// Detect checks if the file uses sqlx.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// sqlxFuncs is the list of sqlx functions to transform.
var sqlxFuncs = map[string]bool{
	"Open":           true,
	"Connect":        true,
	"ConnectContext": true,
	"MustConnect":    true,
	"MustOpen":       true,
}

// Inject transforms sqlx.Open/Connect/MustConnect to whatapsqlx.Open/Connect/MustConnect.
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

			// pkgName.Open/Connect/MustConnect -> whatapsqlx.Open/Connect/MustConnect
			if ident.Name == pkgName && sqlxFuncs[sel.Sel.Name] {
				ident.Name = "whatapsqlx"
				t.transformed = true
			}

			return true
		})
	}

	// Add import only if transformation occurred
	if t.transformed {
		common.AddImport(file, `"github.com/whatap/go-api/instrumentation/github.com/jmoiron/sqlx/whatapsqlx"`)
		// Remove sqlx import if no longer used after transformation
		common.RemoveImportIfUnused(file, t.ImportPath(), pkgName)
	}

	return t.transformed, nil
}

// Remove transforms whatapsqlx.Open/Connect/MustConnect back to sqlx.Open/Connect/MustConnect.
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

			// whatapsqlx.Open/Connect/MustConnect -> sqlx.Open/Connect/MustConnect
			if ident.Name == "whatapsqlx" && sqlxFuncs[sel.Sel.Name] {
				ident.Name = "sqlx"
			}

			return true
		})
	}
	return nil
}
