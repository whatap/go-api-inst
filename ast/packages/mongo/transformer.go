// Package mongo provides the MongoDB transformer.
package mongo

import (
	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for MongoDB.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "mongo"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "go.mongodb.org/mongo-driver/mongo"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/go.mongodb.org/mongo-driver/mongo/whatapmongo"
}

// Detect checks if the file uses MongoDB.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// mongoFuncs is the list of mongo functions to transform.
var mongoFuncs = map[string]bool{
	"Connect":   true,
	"NewClient": true, // deprecated but still supported
}

// Inject transforms mongo.Connect/NewClient to whatapmongo.Connect/NewClient.
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

			// pkgName.Connect/NewClient -> whatapmongo.Connect/NewClient
			if ident.Name == pkgName && mongoFuncs[sel.Sel.Name] {
				ident.Name = "whatapmongo"
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

// Remove transforms whatapmongo.Connect/NewClient back to mongo.Connect/NewClient.
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

			// whatapmongo.Connect/NewClient -> mongo.Connect/NewClient
			if ident.Name == "whatapmongo" && mongoFuncs[sel.Sel.Name] {
				ident.Name = "mongo"
			}

			return true
		})
	}
	return nil
}
