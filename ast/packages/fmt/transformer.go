// Package fmt provides the standard fmt package transformer.
// It transforms fmt.Print/Printf/Println to whatapfmt equivalents
// for transaction-linked stdout logging.
package fmt

import (
	"strings"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements ast.Transformer for standard fmt package.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "fmt"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "fmt"
}

// WhatapImport returns empty string because fmt transformer adds import during Inject().
// This is necessary because WhatapImport() is called before Inject(),
// but we only know if transformation is needed after scanning for Print/Printf/Println calls.
func (t *Transformer) WhatapImport() string {
	return "" // Import is added during Inject() if transformation occurs
}

// Detect checks if the file uses fmt package.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// targetFuncs are the fmt functions to transform.
var targetFuncs = map[string]bool{
	"Print":   true,
	"Printf":  true,
	"Println": true,
}

// Inject transforms fmt.Print/Printf/Println to whatapfmt equivalents.
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	// Reset transformation flag
	t.transformed = false

	// Get the actual package name used in code (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

	dst.Inspect(file, func(n dst.Node) bool {
		call, ok := n.(*dst.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*dst.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*dst.Ident)
		if !ok || ident.Name != pkgName {
			return true
		}

		// Only transform Print, Printf, Println
		if !targetFuncs[sel.Sel.Name] {
			return true
		}

		// Transform: fmt.Print -> whatapfmt.Print
		ident.Name = "whatapfmt"
		t.transformed = true

		return true
	})

	// If transformed, add whatapfmt import and handle fmt import
	if t.transformed {
		// Add whatapfmt import
		common.AddImport(file, `"github.com/whatap/go-api/instrumentation/fmt/whatapfmt"`)

		// Check if fmt is still used elsewhere
		// If not, remove the fmt import to avoid "imported and not used" error
		if !isFmtStillUsedWithPkg(file, pkgName) {
			removeFmtImportWithPkg(file, pkgName)
		}
	}

	return t.transformed, nil
}

// isFmtStillUsed checks if fmt package is still used in the file after transformation.
func isFmtStillUsed(file *dst.File) bool {
	return isFmtStillUsedWithPkg(file, "fmt")
}

// isFmtStillUsedWithPkg checks if package (by name) is still used in the file after transformation.
func isFmtStillUsedWithPkg(file *dst.File, pkgName string) bool {
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

		if ident.Name == pkgName {
			used = true
			return false // Stop inspection
		}
		return true
	})
	return used
}

// removeFmtImport removes the fmt import from the file.
func removeFmtImport(file *dst.File) {
	removeFmtImportWithPkg(file, "fmt")
}

// removeFmtImportWithPkg removes the import for a specific package from the file.
func removeFmtImportWithPkg(file *dst.File, pkgName string) {
	// Find the import path for the package name
	var importPath string
	for _, imp := range file.Imports {
		if imp.Name != nil && imp.Name.Name == pkgName {
			importPath = imp.Path.Value
			break
		}
		// Check if default name matches
		path := strings.Trim(imp.Path.Value, `"`)
		parts := strings.Split(path, "/")
		if parts[len(parts)-1] == pkgName {
			importPath = imp.Path.Value
			break
		}
	}

	if importPath == "" {
		importPath = `"fmt"` // fallback for default case
	}

	dstutil.Apply(file, func(c *dstutil.Cursor) bool {
		n := c.Node()
		importSpec, ok := n.(*dst.ImportSpec)
		if !ok {
			return true
		}

		if importSpec.Path != nil && importSpec.Path.Value == importPath {
			c.Delete()
		}
		return true
	}, nil)

	// Clean up empty import declarations
	newDecls := make([]dst.Decl, 0, len(file.Decls))
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok {
			newDecls = append(newDecls, decl)
			continue
		}

		// Skip empty import blocks
		if genDecl.Tok.String() == "import" && len(genDecl.Specs) == 0 {
			continue
		}
		newDecls = append(newDecls, decl)
	}
	file.Decls = newDecls
}

// Remove restores whatapfmt.Print/Printf/Println to fmt equivalents.
func (t *Transformer) Remove(file *dst.File) error {
	// Note: whatapfmt import is already removed by remover.removeWhatapImports()
	// We just need to restore the function calls

	dst.Inspect(file, func(n dst.Node) bool {
		call, ok := n.(*dst.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*dst.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*dst.Ident)
		if !ok || ident.Name != "whatapfmt" {
			return true
		}

		// Only restore Print, Printf, Println
		if !targetFuncs[sel.Sel.Name] {
			return true
		}

		// Restore: whatapfmt.Print -> fmt.Print
		ident.Name = "fmt"

		return true
	})

	return nil
}
