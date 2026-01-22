// Package common provides shared utilities for AST transformations.
package common

import (
	"github.com/dave/dst"
)

// FindFuncDecl finds a function declaration with the specified name in the file.
func FindFuncDecl(file *dst.File, funcName string) *dst.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		if fn.Name.Name == funcName {
			return fn
		}
	}
	return nil
}

// FindMainFunc finds the main() function.
func FindMainFunc(file *dst.File) *dst.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Name == nil || fn.Recv != nil {
			continue
		}
		if fn.Name.Name == "main" {
			return fn
		}
	}
	return nil
}

// IsMainPackage checks if the file is a main package.
func IsMainPackage(file *dst.File) bool {
	return file.Name != nil && file.Name.Name == "main"
}

// CreateCallExpr creates a function call expression.
// e.g., CreateCallExpr("trace", "Init", nil) → trace.Init(nil)
func CreateCallExpr(pkgName, funcName string, args ...dst.Expr) *dst.CallExpr {
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   dst.NewIdent(pkgName),
			Sel: dst.NewIdent(funcName),
		},
		Args: args,
	}
}

// CreateSelectorExpr creates a selector expression.
// e.g., CreateSelectorExpr("trace", "Init") → trace.Init
func CreateSelectorExpr(pkgName, fieldName string) *dst.SelectorExpr {
	return &dst.SelectorExpr{
		X:   dst.NewIdent(pkgName),
		Sel: dst.NewIdent(fieldName),
	}
}

// CreateExprStmt creates an expression statement.
func CreateExprStmt(expr dst.Expr) *dst.ExprStmt {
	return &dst.ExprStmt{X: expr}
}

// InsertStmtAtBeginning inserts a statement at the beginning of the function body.
func InsertStmtAtBeginning(fn *dst.FuncDecl, stmt dst.Stmt) {
	if fn.Body == nil {
		fn.Body = &dst.BlockStmt{}
	}
	fn.Body.List = append([]dst.Stmt{stmt}, fn.Body.List...)
}

// InsertStmtAfterFirst inserts a statement after the first statement in the function body.
func InsertStmtAfterFirst(fn *dst.FuncDecl, stmt dst.Stmt) {
	if fn.Body == nil || len(fn.Body.List) == 0 {
		InsertStmtAtBeginning(fn, stmt)
		return
	}
	fn.Body.List = append([]dst.Stmt{fn.Body.List[0], stmt}, fn.Body.List[1:]...)
}

// IsCallExpr checks if the node is a function call from a specific package.
func IsCallExpr(node dst.Node, pkgName, funcName string) bool {
	call, ok := node.(*dst.CallExpr)
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

	return ident.Name == pkgName && sel.Sel.Name == funcName
}

// FindCallExpr finds a specific function call in the function body.
func FindCallExpr(fn *dst.FuncDecl, pkgName, funcName string) *dst.CallExpr {
	if fn.Body == nil {
		return nil
	}

	var result *dst.CallExpr
	dst.Inspect(fn.Body, func(n dst.Node) bool {
		if result != nil {
			return false
		}
		if call, ok := n.(*dst.CallExpr); ok {
			if IsCallExpr(call, pkgName, funcName) {
				result = call
				return false
			}
		}
		return true
	})
	return result
}

// RemoveStmt removes statements that satisfy a predicate from the function body.
func RemoveStmt(fn *dst.FuncDecl, predicate func(dst.Stmt) bool) bool {
	if fn.Body == nil {
		return false
	}

	var newList []dst.Stmt
	removed := false
	for _, stmt := range fn.Body.List {
		if predicate(stmt) {
			removed = true
		} else {
			newList = append(newList, stmt)
		}
	}
	fn.Body.List = newList
	return removed
}

// ContainsPackageUsage checks if a specific package is used in the code.
func ContainsPackageUsage(file *dst.File, packageName string) bool {
	found := false
	dst.Inspect(file, func(n dst.Node) bool {
		if found {
			return false
		}

		sel, ok := n.(*dst.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			return true
		}

		if ident.Name == packageName {
			found = true
			return false
		}
		return true
	})
	return found
}

// FindDeferShutdownIndex finds the position of defer trace.Shutdown() in main() function.
// Returns the index if found, -1 if not found.
func FindDeferShutdownIndex(fn *dst.FuncDecl) int {
	if fn == nil || fn.Body == nil {
		return -1
	}

	for i, stmt := range fn.Body.List {
		deferStmt, ok := stmt.(*dst.DeferStmt)
		if !ok {
			continue
		}

		// Check defer trace.Shutdown()
		// deferStmt.Call is already *dst.CallExpr
		sel, ok := deferStmt.Call.Fun.(*dst.SelectorExpr)
		if !ok {
			continue
		}

		ident, ok := sel.X.(*dst.Ident)
		if !ok {
			continue
		}

		if ident.Name == "trace" && sel.Sel.Name == "Shutdown" {
			return i
		}
	}

	return -1
}

// InsertStmtAfterIndex inserts a statement after a specific index in the function body.
func InsertStmtAfterIndex(fn *dst.FuncDecl, index int, stmt dst.Stmt) {
	if fn.Body == nil {
		fn.Body = &dst.BlockStmt{}
	}

	if index < 0 || index >= len(fn.Body.List) {
		// Append to end if index is invalid
		fn.Body.List = append(fn.Body.List, stmt)
		return
	}

	// Insert after index
	newList := make([]dst.Stmt, 0, len(fn.Body.List)+1)
	newList = append(newList, fn.Body.List[:index+1]...)
	newList = append(newList, stmt)
	newList = append(newList, fn.Body.List[index+1:]...)
	fn.Body.List = newList
}

// IsNameDeclared checks if a specific name is declared in the file.
// Checks var, const, type, and function declarations.
func IsNameDeclared(file *dst.File, name string) bool {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *dst.GenDecl:
			// var, const, type declarations
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *dst.ValueSpec:
					// var or const
					for _, ident := range s.Names {
						if ident.Name == name {
							return true
						}
					}
				case *dst.TypeSpec:
					// type
					if s.Name != nil && s.Name.Name == name {
						return true
					}
				}
			}
		case *dst.FuncDecl:
			// func (excluding methods)
			if d.Recv == nil && d.Name != nil && d.Name.Name == name {
				return true
			}
		}
	}
	return false
}

// GetPackageNameFromImport extracts the package name from an import path.
// e.g., "github.com/gin-gonic/gin" → "gin"
func GetPackageNameFromImport(importPath string) string {
	// Remove version suffix
	path := importPath
	if v := ExtractVersion(path); v != "" {
		// Use the part before the version
		idx := len(path) - len(v) - 1
		if idx > 0 {
			path = path[:idx]
		}
	}

	// Return the last path element
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
