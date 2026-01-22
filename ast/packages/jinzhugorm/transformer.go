// Package jinzhugorm provides the github.com/jinzhu/gorm transformer.
package jinzhugorm

import (
	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for github.com/jinzhu/gorm.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "jinzhugorm"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/jinzhu/gorm"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/github.com/jinzhu/gorm/whatapgorm"
}

// Detect checks if the file uses github.com/jinzhu/gorm.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// Inject transforms gorm.Open to whatapgorm.Open.
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

			// pkgName.Open -> whatapgorm.Open
			if ident.Name == pkgName && sel.Sel.Name == "Open" {
				ident.Name = "whatapgorm"
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

// Remove transforms whatapgorm.Open back to gorm.Open.
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

			// whatapgorm.Open -> gorm.Open
			if ident.Name == "whatapgorm" && sel.Sel.Name == "Open" {
				ident.Name = "gorm"
			}

			return true
		})
	}
	return nil
}
