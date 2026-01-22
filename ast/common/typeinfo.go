// Package common provides shared utilities for AST transformations.
package common

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// TypeChecker provides type information for method return types.
// Uses go/types to analyze source code and extract type signatures.
type TypeChecker struct {
	mu    sync.RWMutex
	pkgs  map[string]*packages.Package // cache loaded packages
	fset  *token.FileSet
	debug bool
}

// NewTypeChecker creates a new TypeChecker instance.
func NewTypeChecker() *TypeChecker {
	return &TypeChecker{
		pkgs: make(map[string]*packages.Package),
		fset: token.NewFileSet(),
	}
}

// SetDebug enables debug output.
func (tc *TypeChecker) SetDebug(debug bool) {
	tc.debug = debug
}

// LoadPackage loads and type-checks a package from the given directory.
// Returns the loaded package or an error.
func (tc *TypeChecker) LoadPackage(dir string) (*packages.Package, error) {
	tc.mu.RLock()
	if pkg, ok := tc.pkgs[dir]; ok {
		tc.mu.RUnlock()
		return pkg, nil
	}
	tc.mu.RUnlock()

	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax |
			packages.NeedImports | packages.NeedDeps | packages.NeedName,
		Dir:  dir,
		Fset: tc.fset,
	}

	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to load package: %w", err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found in %s", dir)
	}

	pkg := pkgs[0]

	// Check for package errors
	if len(pkg.Errors) > 0 {
		var errs []string
		for _, e := range pkg.Errors {
			errs = append(errs, e.Error())
		}
		return nil, fmt.Errorf("package errors: %s", strings.Join(errs, "; "))
	}

	tc.mu.Lock()
	tc.pkgs[dir] = pkg
	tc.mu.Unlock()

	return pkg, nil
}

// MethodReturnType represents the return type(s) of a method.
type MethodReturnType struct {
	Types    []string // Return type strings (e.g., "*Record", "error")
	HasError bool     // True if last return is error type
}

// GetMethodReturnType returns the return type of a method call.
// receiverType is the type of the receiver (e.g., "*Client")
// methodName is the name of the method (e.g., "Get")
// pkgPath is the import path of the package (e.g., "github.com/aerospike/aerospike-client-go/v6")
func (tc *TypeChecker) GetMethodReturnType(pkg *packages.Package, receiverType, methodName string) (*MethodReturnType, error) {
	if pkg == nil || pkg.Types == nil {
		return nil, fmt.Errorf("package types not available")
	}

	// Find the receiver type in the package scope
	scope := pkg.Types.Scope()
	if scope == nil {
		return nil, fmt.Errorf("package scope not available")
	}

	// Strip pointer from receiver type for lookup
	baseName := strings.TrimPrefix(receiverType, "*")

	// Look up the type
	obj := scope.Lookup(baseName)
	if obj == nil {
		// Try imported packages
		for _, imported := range pkg.Imports {
			if imported.Types != nil {
				obj = imported.Types.Scope().Lookup(baseName)
				if obj != nil {
					break
				}
			}
		}
	}

	if obj == nil {
		return nil, fmt.Errorf("type %s not found", baseName)
	}

	// Get the named type
	namedType, ok := obj.Type().(*types.Named)
	if !ok {
		return nil, fmt.Errorf("%s is not a named type", baseName)
	}

	// Find the method
	for i := 0; i < namedType.NumMethods(); i++ {
		method := namedType.Method(i)
		if method.Name() == methodName {
			return tc.extractReturnType(method.Type().(*types.Signature))
		}
	}

	// Also check pointer receiver methods
	ptrType := types.NewPointer(namedType)
	methodSet := types.NewMethodSet(ptrType)
	for i := 0; i < methodSet.Len(); i++ {
		sel := methodSet.At(i)
		if sel.Obj().Name() == methodName {
			if sig, ok := sel.Type().(*types.Signature); ok {
				return tc.extractReturnType(sig)
			}
		}
	}

	return nil, fmt.Errorf("method %s not found on type %s", methodName, receiverType)
}

// extractReturnType extracts return types from a function signature.
func (tc *TypeChecker) extractReturnType(sig *types.Signature) (*MethodReturnType, error) {
	results := sig.Results()
	if results == nil || results.Len() == 0 {
		return &MethodReturnType{}, nil
	}

	ret := &MethodReturnType{
		Types: make([]string, results.Len()),
	}

	for i := 0; i < results.Len(); i++ {
		param := results.At(i)
		typeStr := types.TypeString(param.Type(), nil)
		ret.Types[i] = typeStr

		// Check if it's an error type
		if i == results.Len()-1 {
			if named, ok := param.Type().(*types.Named); ok {
				if named.Obj().Name() == "error" {
					ret.HasError = true
				}
			}
			// Also check interface error
			if iface, ok := param.Type().Underlying().(*types.Interface); ok {
				if iface.NumMethods() == 1 && iface.Method(0).Name() == "Error" {
					ret.HasError = true
				}
			}
		}
	}

	return ret, nil
}

// GetExprType returns the type of an expression from TypesInfo.
func (tc *TypeChecker) GetExprType(pkg *packages.Package, expr ast.Expr) (types.Type, error) {
	if pkg == nil || pkg.TypesInfo == nil {
		return nil, fmt.Errorf("types info not available")
	}

	if t, ok := pkg.TypesInfo.Types[expr]; ok {
		return t.Type, nil
	}

	return nil, fmt.Errorf("type not found for expression")
}

// TypeToString converts a types.Type to a simplified string representation.
// The result is suitable for use in generated code.
func TypeToString(t types.Type, pkgAlias string) string {
	switch typ := t.(type) {
	case *types.Pointer:
		return "*" + TypeToString(typ.Elem(), pkgAlias)
	case *types.Slice:
		return "[]" + TypeToString(typ.Elem(), pkgAlias)
	case *types.Named:
		name := typ.Obj().Name()
		pkg := typ.Obj().Pkg()
		if pkg != nil && pkgAlias != "" {
			return pkgAlias + "." + name
		}
		return name
	case *types.Basic:
		return typ.Name()
	case *types.Interface:
		if typ.NumMethods() == 0 {
			return "any"
		}
		return "interface{}"
	default:
		return types.TypeString(t, nil)
	}
}

// ParseTypeExpr creates a DST type expression from a type string.
// typeStr examples: "*aerospike.Record", "bool", "[]byte"
func ParseTypeExpr(typeStr string) interface{} {
	// For simple types, just return the string
	// The actual DST node creation is done by the caller
	return typeStr
}
