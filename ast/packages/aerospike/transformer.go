// Package aerospike provides the Aerospike transformer.
// Uses sql.Wrap approach with go/types for version-safe instrumentation.
package aerospike

import (
	"fmt"
	"go/types"
	"os"
	"strings"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
	"golang.org/x/tools/go/packages"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for Aerospike.
// Also implements common.TypedTransformer for go/types support.
type Transformer struct {
	pkg       *packages.Package // loaded package for type info
	asAlias   string            // aerospike package alias
	asImport  string            // aerospike import path (v6 or v8)
	debug     bool
	needsFmt  bool // true if fmt.Sprintf is needed for host extraction
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "aerospike"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/aerospike/aerospike-client-go"
}

// WhatapImport returns the whatap sql import for Wrap functions.
// Note: Using alias "whatapsql" to avoid conflict with database/sql
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/sql"
}

// WhatapImportAlias returns the alias for whatap sql import.
func (t *Transformer) WhatapImportAlias() string {
	return "whatapsql"
}

// WhatapAsImport returns the whatapas helper import path based on aerospike version.
// e.g., github.com/aerospike/aerospike-client-go/v6 -> github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas
func (t *Transformer) WhatapAsImport() string {
	if t.asImport == "" {
		// Default to v6 if not detected
		return "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas"
	}
	return "github.com/whatap/go-api/instrumentation/" + t.asImport + "/whatapas"
}

// Detect checks if the file uses Aerospike (v6 or v8).
func (t *Transformer) Detect(file *dst.File) bool {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.HasPrefix(path, "github.com/aerospike/aerospike-client-go") {
			return true
		}
	}
	return false
}

// Inject transforms Aerospike calls (fallback without type info).
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	// Fallback: use hardcoded types when go/types not available
	return t.injectWithFallback(file)
}

// InjectWithDir transforms Aerospike calls using go/types for type inference.
// This is the preferred method that provides version-independent instrumentation.
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) InjectWithDir(file *dst.File, dir string) (bool, error) {
	t.debug = os.Getenv("GO_API_AST_DEBUG") != ""

	if t.debug {
		fmt.Printf("[DEBUG] aerospike: InjectWithDir called for dir=%s\n", dir)
	}

	if !t.Detect(file) {
		if t.debug {
			fmt.Println("[DEBUG] aerospike: Detect returned false")
		}
		return false, nil
	}

	// Get aerospike alias and import path
	t.asAlias = common.GetPackageNameForImportPrefix(file, t.ImportPath())
	t.asImport = t.getAerospikeImportPath(file)
	if t.asAlias == "" {
		return false, nil
	}

	// Try to load package for type information
	pkg, err := t.loadPackage(dir)
	if err != nil {
		if t.debug {
			fmt.Printf("[DEBUG] aerospike: failed to load types, using fallback: %v\n", err)
		}
		// Fallback to hardcoded types
		return t.injectWithFallback(file)
	}
	t.pkg = pkg

	if t.debug {
		fmt.Printf("[DEBUG] aerospike: loaded package %s with types\n", pkg.Name)
	}

	// Reset needsFmt flag
	t.needsFmt = false
	transformed := false

	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		t.processStatements(fn.Body.List, &transformed)
	}

	if transformed {
		common.AddImportWithAlias(file, t.WhatapImport(), t.WhatapImportAlias())
		common.AddImport(file, t.WhatapAsImport()) // whatapas for GetDbhost
		common.AddImport(file, "context")
		if t.needsFmt {
			common.AddImport(file, "fmt")
		}
	}

	return transformed, nil
}

// loadPackage loads the package from directory for type checking.
// Returns nil if loading fails (will use fallback).
func (t *Transformer) loadPackage(dir string) (pkg *packages.Package, err error) {
	// Recover from panics in go/types
	defer func() {
		if r := recover(); r != nil {
			if t.debug {
				fmt.Printf("[DEBUG] aerospike: recovered from panic in loadPackage: %v\n", r)
			}
			pkg = nil
			err = fmt.Errorf("panic during type checking: %v", r)
		}
	}()

	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax |
			packages.NeedImports | packages.NeedDeps | packages.NeedName,
		Dir: dir,
	}

	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("load package: %w", err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found")
	}

	pkg = pkgs[0]
	if len(pkg.Errors) > 0 {
		// Return first error but still try to use partial info
		if t.debug {
			fmt.Printf("[DEBUG] aerospike: package has errors: %v\n", pkg.Errors[0])
		}
	}

	return pkg, nil
}

// getAerospikeImportPath returns the import path for aerospike package.
func (t *Transformer) getAerospikeImportPath(file *dst.File) string {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.HasPrefix(path, "github.com/aerospike/aerospike-client-go") {
			return path
		}
	}
	return ""
}

// processStatements recursively processes statements for Aerospike calls.
func (t *Transformer) processStatements(stmts []dst.Stmt, transformed *bool) {
	for i := range stmts {
		stmt := stmts[i]

		switch s := stmt.(type) {
		case *dst.AssignStmt:
			// Handle: client, err := aerospike.NewClient(...)
			// Handle: record, err := client.Get(...)
			for j, rhs := range s.Rhs {
				if call, ok := rhs.(*dst.CallExpr); ok {
					if t.isNewClientCall(call) {
						s.Rhs[j] = t.wrapNewClient(call)
						*transformed = true
					} else if wrapped := t.tryWrapMethodCall(call, s); wrapped != nil {
						s.Rhs[j] = wrapped
						*transformed = true
					}
				}
			}

		case *dst.ExprStmt:
			// Handle: client.Put(...)
			if call, ok := s.X.(*dst.CallExpr); ok {
				if wrapped := t.tryWrapMethodCall(call, nil); wrapped != nil {
					s.X = wrapped
					*transformed = true
				}
			}

		case *dst.IfStmt:
			// Handle: if record, err := client.Get(...); err != nil { }
			if s.Init != nil {
				if assign, ok := s.Init.(*dst.AssignStmt); ok {
					for j, rhs := range assign.Rhs {
						if call, ok := rhs.(*dst.CallExpr); ok {
							if wrapped := t.tryWrapMethodCall(call, assign); wrapped != nil {
								assign.Rhs[j] = wrapped
								*transformed = true
							}
						}
					}
				}
			}
			if s.Body != nil {
				t.processStatements(s.Body.List, transformed)
			}
			if s.Else != nil {
				if block, ok := s.Else.(*dst.BlockStmt); ok {
					t.processStatements(block.List, transformed)
				}
			}

		case *dst.ForStmt:
			if s.Body != nil {
				t.processStatements(s.Body.List, transformed)
			}

		case *dst.RangeStmt:
			if s.Body != nil {
				t.processStatements(s.Body.List, transformed)
			}

		case *dst.BlockStmt:
			t.processStatements(s.List, transformed)

		case *dst.DeferStmt:
			// Handle: defer client.Close()
			if call := s.Call; call != nil {
				if wrapped := t.tryWrapMethodCall(call, nil); wrapped != nil {
					if wrappedCall, ok := wrapped.(*dst.CallExpr); ok {
						s.Call = wrappedCall
						*transformed = true
					}
				}
			}
		}
	}
}

// isNewClientCall checks if the call is aerospike.NewClient*.
func (t *Transformer) isNewClientCall(call *dst.CallExpr) bool {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	newClientFuncs := map[string]bool{
		"NewClient":                  true,
		"NewClientWithPolicy":        true,
		"NewClientWithPolicyAndHost": true,
	}

	return ident.Name == t.asAlias && newClientFuncs[sel.Sel.Name]
}

// wrapNewClient wraps NewClient call with sql.WrapOpen.
func (t *Transformer) wrapNewClient(call *dst.CallExpr) *dst.CallExpr {
	// Create closure: func() (*aerospike.Client, error) { return aerospike.NewClient(...) }
	closure := &dst.FuncLit{
		Type: &dst.FuncType{
			Params: &dst.FieldList{},
			Results: &dst.FieldList{
				List: []*dst.Field{
					{Type: &dst.StarExpr{X: &dst.SelectorExpr{
						X:   &dst.Ident{Name: t.asAlias},
						Sel: &dst.Ident{Name: "Client"},
					}}},
					{Type: &dst.Ident{Name: "error"}},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.ReturnStmt{
					Results: []dst.Expr{dst.Clone(call).(dst.Expr)},
				},
			},
		},
	}

	// Extract host expression from NewClient call
	dbhostExpr := t.extractDbhostExpr(call)

	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "whatapsql"},
			Sel: &dst.Ident{Name: "WrapOpen"},
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: "context"},
					Sel: &dst.Ident{Name: "Background"},
				},
			},
			dbhostExpr,
			closure,
		},
	}
}

// extractDbhostExpr extracts host information from NewClient call and returns
// an expression for the dbhost parameter.
//
// Supported patterns:
//   - NewClientWithPolicy(policy, host, port) → fmt.Sprintf("aerospike://%v:%v", host, port)
//   - NewClient(policy, hosts...) → "aerospike" (fallback)
func (t *Transformer) extractDbhostExpr(call *dst.CallExpr) dst.Expr {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return t.defaultDbhostLit()
	}

	methodName := sel.Sel.Name

	// NewClientWithPolicy(policy, host, port) - has host and port as separate args
	if methodName == "NewClientWithPolicy" && len(call.Args) >= 3 {
		hostArg := call.Args[1] // 2nd arg: host (string)
		portArg := call.Args[2] // 3rd arg: port (int)

		// Generate: fmt.Sprintf("aerospike://%v:%v", host, port)
		t.needsFmt = true
		return &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   &dst.Ident{Name: "fmt"},
				Sel: &dst.Ident{Name: "Sprintf"},
			},
			Args: []dst.Expr{
				&dst.BasicLit{Kind: 9, Value: `"aerospike://%v:%v"`}, // token.STRING = 9
				dst.Clone(hostArg).(dst.Expr),
				dst.Clone(portArg).(dst.Expr),
			},
		}
	}

	// NewClient(policy, hosts...*Host) or NewClientWithPolicyAndHost(policy, hosts...*Host)
	// Try to extract from first Host argument
	if (methodName == "NewClient" || methodName == "NewClientWithPolicyAndHost") && len(call.Args) >= 2 {
		// Check if second arg is aerospike.NewHost("host", port)
		if hostCall, ok := call.Args[1].(*dst.CallExpr); ok {
			if hostExpr := t.extractFromNewHost(hostCall); hostExpr != nil {
				return hostExpr
			}
		}
	}

	// Fallback: return static "aerospike"
	return t.defaultDbhostLit()
}

// extractFromNewHost extracts host info from aerospike.NewHost(host, port) call.
func (t *Transformer) extractFromNewHost(call *dst.CallExpr) dst.Expr {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return nil
	}

	// Check if it's as.NewHost or aerospike.NewHost
	ident, ok := sel.X.(*dst.Ident)
	if !ok || ident.Name != t.asAlias {
		return nil
	}

	if sel.Sel.Name != "NewHost" || len(call.Args) < 2 {
		return nil
	}

	hostArg := call.Args[0] // 1st arg: host (string)
	portArg := call.Args[1] // 2nd arg: port (int)

	// Generate: fmt.Sprintf("aerospike://%v:%v", host, port)
	t.needsFmt = true
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "fmt"},
			Sel: &dst.Ident{Name: "Sprintf"},
		},
		Args: []dst.Expr{
			&dst.BasicLit{Kind: 9, Value: `"aerospike://%v:%v"`},
			dst.Clone(hostArg).(dst.Expr),
			dst.Clone(portArg).(dst.Expr),
		},
	}
}

// defaultDbhostLit returns the default "aerospike" literal.
func (t *Transformer) defaultDbhostLit() dst.Expr {
	return &dst.BasicLit{Kind: 9, Value: `"aerospike"`} // token.STRING = 9
}

// tryWrapMethodCall checks if call is an Aerospike Client method and wraps it.
// assignStmt is the parent assignment statement (if any) for type inference from LHS.
func (t *Transformer) tryWrapMethodCall(call *dst.CallExpr, assignStmt *dst.AssignStmt) dst.Expr {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return nil
	}

	methodName := sel.Sel.Name

	// Get return type using go/types
	returnType := t.getMethodReturnType(sel, methodName)
	if returnType == nil {
		return nil
	}

	if t.debug {
		fmt.Printf("[DEBUG] aerospike: wrapping %s with return type %v\n", methodName, returnType)
	}

	// Extract receiver (e.g., "client" from "client.Put(...)")
	receiver := sel.X

	// Check if method returns only error (use WrapError)
	if len(returnType.types) == 1 && returnType.isError {
		return t.wrapWithWrapError(call, methodName, receiver)
	}

	// Method returns (T, error) - use Wrap with proper type
	// Note: returnType.types may be nil/empty in fallback mode, but we still wrap
	// using hardcoded types from aerospikeMethodReturnTypes map
	if _, hasType := aerospikeMethodReturnTypes[methodName]; hasType || len(returnType.types) >= 1 {
		return t.wrapWithWrap(call, methodName, returnType, receiver)
	}

	return nil
}

// methodReturnInfo holds return type information
type methodReturnInfo struct {
	types   []types.Type // return types
	isError bool         // true if last return is error
}

// Hardcoded return types for Aerospike Client methods.
// These are used when go/types is not available (fallback mode).
// Format: method name -> return type string (for first return value)
var aerospikeMethodReturnTypes = map[string]string{
	"Get":             "*aerospike.Record",
	"GetHeader":       "*aerospike.Record",
	"Exists":          "bool",
	"Delete":          "bool",
	"BatchGet":        "[]*aerospike.Record",
	"BatchGetHeader":  "[]*aerospike.Record",
	"BatchExists":     "[]bool",
	"BatchDelete":     "[]*aerospike.BatchRecord",
	"Query":           "*aerospike.Recordset",
	"ScanAll":         "*aerospike.Recordset",
	"ScanNode":        "*aerospike.Recordset",
	"Operate":         "*aerospike.Record",
	"Execute":         "interface{}",
	"QueryAggregate":  "*aerospike.Recordset",
}

// getMethodReturnType gets the return type of a method call.
// Uses hardcoded types in fallback mode.
func (t *Transformer) getMethodReturnType(sel *dst.SelectorExpr, methodName string) *methodReturnInfo {
	// Known Aerospike Client methods that return only error
	// Note: Close is excluded because:
	// 1. Client.Close() returns void in Aerospike v6 (not error)
	// 2. Recordset.Close() also exists and shouldn't be wrapped
	errorOnlyMethods := map[string]bool{
		"Put": true, "PutBins": true, "Append": true, "Prepend": true,
		"Add": true, "Touch": true,
		"Truncate": true, "CreateIndex": true, "DropIndex": true,
	}

	if errorOnlyMethods[methodName] {
		return &methodReturnInfo{isError: true, types: []types.Type{types.Universe.Lookup("error").Type()}}
	}

	// Check if method has a known return type
	if _, ok := aerospikeMethodReturnTypes[methodName]; !ok {
		return nil
	}

	// Try to get actual type from go/types
	if t.pkg != nil && t.pkg.Types != nil {
		// Find aerospike package in imports
		for _, imported := range t.pkg.Imports {
			if strings.HasPrefix(imported.PkgPath, "github.com/aerospike/aerospike-client-go") {
				// Look up Client type
				clientObj := imported.Types.Scope().Lookup("Client")
				if clientObj == nil {
					continue
				}

				named, ok := clientObj.Type().(*types.Named)
				if !ok {
					continue
				}

				// Find method on *Client
				ptrType := types.NewPointer(named)
				methodSet := types.NewMethodSet(ptrType)

				for i := 0; i < methodSet.Len(); i++ {
					m := methodSet.At(i)
					if m.Obj().Name() == methodName {
						if sig, ok := m.Type().(*types.Signature); ok {
							results := sig.Results()
							if results.Len() > 0 {
								info := &methodReturnInfo{
									types: make([]types.Type, results.Len()),
								}
								for j := 0; j < results.Len(); j++ {
									info.types[j] = results.At(j).Type()
								}
								// Check if last is error
								lastType := results.At(results.Len() - 1).Type()
								if named, ok := lastType.(*types.Named); ok {
									info.isError = named.Obj().Name() == "error"
								}
								return info
							}
						}
					}
				}
			}
		}
	}

	// Fallback: use hardcoded type
	return &methodReturnInfo{
		types:   nil, // will use hardcoded type string
		isError: true,
	}
}

// wrapWithWrapError wraps call with sql.WrapError for error-only methods.
// receiver is the client variable (e.g., "client" from "client.Put(...)").
func (t *Transformer) wrapWithWrapError(call *dst.CallExpr, methodName string, receiver dst.Expr) *dst.CallExpr {
	// For Put method, use specialized WrapPut with key/bins extraction
	if methodName == "Put" && len(call.Args) >= 3 {
		return t.wrapPutCall(call, receiver)
	}

	// For other error-only methods, use generic wrapper
	closure := &dst.FuncLit{
		Type: &dst.FuncType{
			Params: &dst.FieldList{},
			Results: &dst.FieldList{
				List: []*dst.Field{
					{Type: &dst.Ident{Name: "error"}},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.ReturnStmt{
					Results: []dst.Expr{dst.Clone(call).(dst.Expr)},
				},
			},
		},
	}

	// Generate: whatapas.GetDbhost(receiver)
	dbhostExpr := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "whatapas"},
			Sel: &dst.Ident{Name: "GetDbhost"},
		},
		Args: []dst.Expr{
			dst.Clone(receiver).(dst.Expr),
		},
	}

	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "whatapsql"},
			Sel: &dst.Ident{Name: "WrapError"},
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: "context"},
					Sel: &dst.Ident{Name: "Background"},
				},
			},
			dbhostExpr,
			&dst.BasicLit{Kind: 9, Value: `"` + methodName + `"`},
			closure,
		},
	}
}

// wrapPutCall wraps client.Put(policy, key, bins) with whatapas.WrapPut.
// Extracts key and bins from call arguments for SQL-like query collection.
func (t *Transformer) wrapPutCall(call *dst.CallExpr, receiver dst.Expr) *dst.CallExpr {
	// call.Args[0] = policy, call.Args[1] = key, call.Args[2] = bins
	keyArg := call.Args[1]
	binsArg := call.Args[2]

	// Create closure: func() error { return client.Put(...) }
	closure := &dst.FuncLit{
		Type: &dst.FuncType{
			Params: &dst.FieldList{},
			Results: &dst.FieldList{
				List: []*dst.Field{
					{Type: &dst.Ident{Name: "error"}},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.ReturnStmt{
					Results: []dst.Expr{dst.Clone(call).(dst.Expr)},
				},
			},
		},
	}

	// Generate: whatapas.WrapPut(context.Background(), client, key, bins, fn)
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "whatapas"},
			Sel: &dst.Ident{Name: "WrapPut"},
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: "context"},
					Sel: &dst.Ident{Name: "Background"},
				},
			},
			dst.Clone(receiver).(dst.Expr),
			dst.Clone(keyArg).(dst.Expr),
			dst.Clone(binsArg).(dst.Expr),
			closure,
		},
	}
}

// wrapWithWrap wraps call with sql.Wrap for methods returning (T, error).
// receiver is the client variable (e.g., "client" from "client.Get(...)").
func (t *Transformer) wrapWithWrap(call *dst.CallExpr, methodName string, retInfo *methodReturnInfo, receiver dst.Expr) *dst.CallExpr {
	// For Get method, use specialized WrapGet with key extraction
	if methodName == "Get" && len(call.Args) >= 2 {
		return t.wrapGetCall(call, receiver)
	}

	// For Delete method, use specialized WrapDelete with key extraction
	if methodName == "Delete" && len(call.Args) >= 2 {
		return t.wrapDeleteCall(call, receiver)
	}

	// For Exists method, use specialized WrapExists with key extraction
	if methodName == "Exists" && len(call.Args) >= 2 {
		return t.wrapExistsCall(call, receiver)
	}

	// For other methods, use generic wrapper
	// Build return type expression
	var returnTypeExpr dst.Expr

	if retInfo.types != nil && len(retInfo.types) > 0 {
		// Use actual type from go/types
		returnTypeExpr = t.typeToExpr(retInfo.types[0])
	} else if typeStr, ok := aerospikeMethodReturnTypes[methodName]; ok {
		// Use hardcoded type from map
		returnTypeExpr = t.parseTypeString(typeStr)
	} else {
		// Fallback to 'any'
		returnTypeExpr = &dst.Ident{Name: "any"}
	}

	closure := &dst.FuncLit{
		Type: &dst.FuncType{
			Params: &dst.FieldList{},
			Results: &dst.FieldList{
				List: []*dst.Field{
					{Type: returnTypeExpr},
					{Type: &dst.Ident{Name: "error"}},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.ReturnStmt{
					Results: []dst.Expr{dst.Clone(call).(dst.Expr)},
				},
			},
		},
	}

	// Generate: whatapas.GetDbhost(receiver)
	dbhostExpr := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "whatapas"},
			Sel: &dst.Ident{Name: "GetDbhost"},
		},
		Args: []dst.Expr{
			dst.Clone(receiver).(dst.Expr),
		},
	}

	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "whatapsql"},
			Sel: &dst.Ident{Name: "Wrap"},
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: "context"},
					Sel: &dst.Ident{Name: "Background"},
				},
			},
			dbhostExpr,
			&dst.BasicLit{Kind: 9, Value: `"` + methodName + `"`},
			closure,
		},
	}
}

// wrapGetCall wraps client.Get(policy, key, binNames...) with whatapas.WrapGet.
func (t *Transformer) wrapGetCall(call *dst.CallExpr, receiver dst.Expr) *dst.CallExpr {
	// call.Args[0] = policy, call.Args[1] = key, call.Args[2:] = binNames (optional)
	keyArg := call.Args[1]

	// Extract binNames if present (variadic string arguments)
	var binNamesExpr dst.Expr
	if len(call.Args) > 2 {
		// Create []string{binNames...}
		binNamesExpr = &dst.CompositeLit{
			Type: &dst.ArrayType{Elt: &dst.Ident{Name: "string"}},
			Elts: func() []dst.Expr {
				elts := make([]dst.Expr, 0, len(call.Args)-2)
				for i := 2; i < len(call.Args); i++ {
					elts = append(elts, dst.Clone(call.Args[i]).(dst.Expr))
				}
				return elts
			}(),
		}
	} else {
		// No binNames, use nil
		binNamesExpr = &dst.Ident{Name: "nil"}
	}

	// Create closure: func() (*aerospike.Record, error) { return client.Get(...) }
	closure := &dst.FuncLit{
		Type: &dst.FuncType{
			Params: &dst.FieldList{},
			Results: &dst.FieldList{
				List: []*dst.Field{
					{Type: &dst.StarExpr{X: &dst.SelectorExpr{
						X:   &dst.Ident{Name: t.asAlias},
						Sel: &dst.Ident{Name: "Record"},
					}}},
					{Type: &dst.Ident{Name: "error"}},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.ReturnStmt{
					Results: []dst.Expr{dst.Clone(call).(dst.Expr)},
				},
			},
		},
	}

	// Generate: whatapas.WrapGet(context.Background(), client, key, binNames, fn)
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "whatapas"},
			Sel: &dst.Ident{Name: "WrapGet"},
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: "context"},
					Sel: &dst.Ident{Name: "Background"},
				},
			},
			dst.Clone(receiver).(dst.Expr),
			dst.Clone(keyArg).(dst.Expr),
			binNamesExpr,
			closure,
		},
	}
}

// wrapDeleteCall wraps client.Delete(policy, key) with whatapas.WrapDelete.
func (t *Transformer) wrapDeleteCall(call *dst.CallExpr, receiver dst.Expr) *dst.CallExpr {
	// call.Args[0] = policy, call.Args[1] = key
	keyArg := call.Args[1]

	// Create closure: func() (bool, error) { return client.Delete(...) }
	closure := &dst.FuncLit{
		Type: &dst.FuncType{
			Params: &dst.FieldList{},
			Results: &dst.FieldList{
				List: []*dst.Field{
					{Type: &dst.Ident{Name: "bool"}},
					{Type: &dst.Ident{Name: "error"}},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.ReturnStmt{
					Results: []dst.Expr{dst.Clone(call).(dst.Expr)},
				},
			},
		},
	}

	// Generate: whatapas.WrapDelete(context.Background(), client, key, fn)
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "whatapas"},
			Sel: &dst.Ident{Name: "WrapDelete"},
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: "context"},
					Sel: &dst.Ident{Name: "Background"},
				},
			},
			dst.Clone(receiver).(dst.Expr),
			dst.Clone(keyArg).(dst.Expr),
			closure,
		},
	}
}

// wrapExistsCall wraps client.Exists(policy, key) with whatapas.WrapExists.
func (t *Transformer) wrapExistsCall(call *dst.CallExpr, receiver dst.Expr) *dst.CallExpr {
	// call.Args[0] = policy, call.Args[1] = key
	keyArg := call.Args[1]

	// Create closure: func() (bool, error) { return client.Exists(...) }
	closure := &dst.FuncLit{
		Type: &dst.FuncType{
			Params: &dst.FieldList{},
			Results: &dst.FieldList{
				List: []*dst.Field{
					{Type: &dst.Ident{Name: "bool"}},
					{Type: &dst.Ident{Name: "error"}},
				},
			},
		},
		Body: &dst.BlockStmt{
			List: []dst.Stmt{
				&dst.ReturnStmt{
					Results: []dst.Expr{dst.Clone(call).(dst.Expr)},
				},
			},
		},
	}

	// Generate: whatapas.WrapExists(context.Background(), client, key, fn)
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "whatapas"},
			Sel: &dst.Ident{Name: "WrapExists"},
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: "context"},
					Sel: &dst.Ident{Name: "Background"},
				},
			},
			dst.Clone(receiver).(dst.Expr),
			dst.Clone(keyArg).(dst.Expr),
			closure,
		},
	}
}

// parseTypeString converts a type string to dst.Expr.
// Supports: "*pkg.Type", "[]pkg.Type", "bool", "interface{}"
func (t *Transformer) parseTypeString(typeStr string) dst.Expr {
	// Handle pointer types
	if strings.HasPrefix(typeStr, "*") {
		return &dst.StarExpr{X: t.parseTypeString(typeStr[1:])}
	}

	// Handle slice types
	if strings.HasPrefix(typeStr, "[]") {
		return &dst.ArrayType{Elt: t.parseTypeString(typeStr[2:])}
	}

	// Handle interface{}
	if typeStr == "interface{}" {
		return &dst.InterfaceType{}
	}

	// Handle qualified types (pkg.Type)
	if strings.Contains(typeStr, ".") {
		parts := strings.SplitN(typeStr, ".", 2)
		pkgName := parts[0]
		typeName := parts[1]

		// Use the actual alias for aerospike package
		if pkgName == "aerospike" {
			pkgName = t.asAlias
		}

		return &dst.SelectorExpr{
			X:   &dst.Ident{Name: pkgName},
			Sel: &dst.Ident{Name: typeName},
		}
	}

	// Simple type (bool, int, etc.)
	return &dst.Ident{Name: typeStr}
}

// typeToExpr converts a types.Type to dst.Expr for code generation.
func (t *Transformer) typeToExpr(typ types.Type) dst.Expr {
	switch ty := typ.(type) {
	case *types.Pointer:
		return &dst.StarExpr{X: t.typeToExpr(ty.Elem())}

	case *types.Slice:
		return &dst.ArrayType{Elt: t.typeToExpr(ty.Elem())}

	case *types.Named:
		name := ty.Obj().Name()
		pkg := ty.Obj().Pkg()
		if pkg != nil {
			pkgPath := pkg.Path()
			// Use alias for aerospike package
			if strings.HasPrefix(pkgPath, "github.com/aerospike/aerospike-client-go") {
				return &dst.SelectorExpr{
					X:   &dst.Ident{Name: t.asAlias},
					Sel: &dst.Ident{Name: name},
				}
			}
			// For other packages, use last part of path
			parts := strings.Split(pkgPath, "/")
			pkgName := parts[len(parts)-1]
			return &dst.SelectorExpr{
				X:   &dst.Ident{Name: pkgName},
				Sel: &dst.Ident{Name: name},
			}
		}
		return &dst.Ident{Name: name}

	case *types.Basic:
		return &dst.Ident{Name: ty.Name()}

	case *types.Interface:
		if ty.NumMethods() == 0 {
			return &dst.Ident{Name: "any"}
		}
		return &dst.Ident{Name: "interface{}"}

	default:
		// Fallback
		return &dst.Ident{Name: "any"}
	}
}

// injectWithFallback uses hardcoded types when go/types is not available.
func (t *Transformer) injectWithFallback(file *dst.File) (bool, error) {
	if !t.Detect(file) {
		return false, nil
	}

	t.asAlias = common.GetPackageNameForImportPrefix(file, t.ImportPath())
	t.asImport = t.getAerospikeImportPath(file)
	if t.asAlias == "" {
		return false, nil
	}

	// Use fallback with nil pkg (will use 'any' type)
	t.pkg = nil
	t.needsFmt = false // Reset needsFmt flag
	transformed := false

	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		t.processStatements(fn.Body.List, &transformed)
	}

	if transformed {
		common.AddImportWithAlias(file, t.WhatapImport(), t.WhatapImportAlias())
		common.AddImport(file, t.WhatapAsImport()) // whatapas for GetDbhost
		common.AddImport(file, "context")
		if t.needsFmt {
			common.AddImport(file, "fmt")
		}
	}

	return transformed, nil
}

// Remove restores original Aerospike calls from sql.Wrap patterns.
func (t *Transformer) Remove(file *dst.File) error {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		t.removeWrapCalls(fn.Body)
	}
	return nil
}

// removeWrapCalls recursively removes sql.Wrap* wrappers.
func (t *Transformer) removeWrapCalls(block *dst.BlockStmt) {
	for _, stmt := range block.List {
		switch s := stmt.(type) {
		case *dst.AssignStmt:
			for j, rhs := range s.Rhs {
				if unwrapped := t.unwrapCall(rhs); unwrapped != nil {
					s.Rhs[j] = unwrapped
				}
			}

		case *dst.ExprStmt:
			if unwrapped := t.unwrapCall(s.X); unwrapped != nil {
				s.X = unwrapped
			}

		case *dst.IfStmt:
			if s.Init != nil {
				if assign, ok := s.Init.(*dst.AssignStmt); ok {
					for j, rhs := range assign.Rhs {
						if unwrapped := t.unwrapCall(rhs); unwrapped != nil {
							assign.Rhs[j] = unwrapped
						}
					}
				}
			}
			if s.Body != nil {
				t.removeWrapCalls(s.Body)
			}
			if block, ok := s.Else.(*dst.BlockStmt); ok {
				t.removeWrapCalls(block)
			}

		case *dst.ForStmt:
			if s.Body != nil {
				t.removeWrapCalls(s.Body)
			}

		case *dst.RangeStmt:
			if s.Body != nil {
				t.removeWrapCalls(s.Body)
			}

		case *dst.BlockStmt:
			t.removeWrapCalls(s)

		case *dst.DeferStmt:
			if unwrapped := t.unwrapCall(s.Call); unwrapped != nil {
				if call, ok := unwrapped.(*dst.CallExpr); ok {
					s.Call = call
				}
			}
		}
	}
}

// unwrapCall extracts original call from sql.Wrap* or whatapas.Wrap* wrapper.
func (t *Transformer) unwrapCall(expr dst.Expr) dst.Expr {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return nil
	}

	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return nil
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return nil
	}

	// Check for whatapsql wrappers
	if ident.Name == "whatapsql" {
		wrapFuncs := map[string]bool{
			"Wrap": true, "WrapError": true, "WrapOpen": true,
		}
		if !wrapFuncs[sel.Sel.Name] {
			return nil
		}
	} else if ident.Name == "whatapas" {
		// Check for whatapas wrappers
		wrapFuncs := map[string]bool{
			"WrapPut": true, "WrapPutBins": true, "WrapGet": true,
			"WrapDelete": true, "WrapExists": true,
			"WrapGeneric": true, "WrapGenericWithKey": true,
		}
		if !wrapFuncs[sel.Sel.Name] {
			return nil
		}
	} else {
		return nil
	}

	// Extract closure (last argument)
	if len(call.Args) < 3 {
		return nil
	}

	closure, ok := call.Args[len(call.Args)-1].(*dst.FuncLit)
	if !ok || closure.Body == nil || len(closure.Body.List) == 0 {
		return nil
	}

	retStmt, ok := closure.Body.List[0].(*dst.ReturnStmt)
	if !ok || len(retStmt.Results) == 0 {
		return nil
	}

	return dst.Clone(retStmt.Results[0]).(dst.Expr)
}
