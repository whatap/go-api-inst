// Package fasthttp provides the FastHTTP framework transformer.
package fasthttp

import (
	"go/token"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements ast.Transformer for FastHTTP.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "fasthttp"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/valyala/fasthttp"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp"
}

// Detect checks if the file uses FastHTTP.
func (t *Transformer) Detect(file *dst.File) bool {
	if !common.HasImport(file, t.ImportPath()) {
		return false
	}
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false
	}
	return hasHandlerMethodCalls(file) || hasFasthttpServerLiterals(file, pkgName)
}

// Inject adds FastHTTP handler instrumentation.
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}
	t.injectHandlerMethods(file)
	t.injectServerHandlers(file, pkgName)
	return t.transformed, nil
}

// Remove removes FastHTTP handler instrumentation.
func (t *Transformer) Remove(file *dst.File) error {
	t.removeHandlerMethodWrappers(file)
	t.removeServerHandlerWrappers(file)
	return nil
}

// hasHandlerMethodCalls checks if file has router method calls (GET, POST, etc.).
func hasHandlerMethodCalls(file *dst.File) bool {
	httpMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true, "ANY": true,
	}

	found := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		dst.Inspect(fn.Body, func(n dst.Node) bool {
			if found {
				return false
			}
			exprStmt, ok := n.(*dst.ExprStmt)
			if !ok {
				return true
			}
			call, ok := exprStmt.X.(*dst.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*dst.SelectorExpr)
			if !ok {
				return true
			}
			if httpMethods[sel.Sel.Name] && len(call.Args) >= 2 {
				found = true
				return false
			}
			return true
		})
	}
	return found
}

// hasFasthttpServerLiterals checks if file has fasthttp.Server{Handler: ...} literals.
func hasFasthttpServerLiterals(file *dst.File, pkgName string) bool {
	found := false

	// Check package-level declarations
	for _, decl := range file.Decls {
		if found {
			break
		}
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*dst.ValueSpec)
			if !ok {
				continue
			}
			for _, value := range valueSpec.Values {
				if isFasthttpServerWithHandler(value, pkgName) {
					found = true
					break
				}
			}
		}
	}

	if found {
		return true
	}

	// Check function bodies
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		dst.Inspect(fn.Body, func(n dst.Node) bool {
			if found {
				return false
			}
			if isFasthttpServerWithHandler(n, pkgName) {
				found = true
				return false
			}
			return true
		})
	}
	return found
}

// isFasthttpServerWithHandler checks if a node is fasthttp.Server{Handler: <non-nil>}.
func isFasthttpServerWithHandler(n dst.Node, pkgName string) bool {
	var compositeLit *dst.CompositeLit
	if unary, ok := n.(*dst.UnaryExpr); ok && unary.Op == token.AND {
		if cl, ok := unary.X.(*dst.CompositeLit); ok {
			compositeLit = cl
		}
	} else if cl, ok := n.(*dst.CompositeLit); ok {
		compositeLit = cl
	}

	if compositeLit == nil {
		return false
	}

	sel, ok := compositeLit.Type.(*dst.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*dst.Ident)
	if !ok || ident.Name != pkgName || sel.Sel.Name != "Server" {
		return false
	}

	// Check if Handler field exists and is non-nil
	for _, elt := range compositeLit.Elts {
		kv, ok := elt.(*dst.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*dst.Ident)
		if !ok || keyIdent.Name != "Handler" {
			continue
		}
		// Skip if Handler: nil
		if valIdent, ok := kv.Value.(*dst.Ident); ok && valIdent.Name == "nil" {
			return false
		}
		return true
	}
	return false
}

// injectHandlerMethods wraps router method handler args (GET, POST, etc.) with whatapfasthttp.Func.
func (t *Transformer) injectHandlerMethods(file *dst.File) {
	httpMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true, "ANY": true,
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		dst.Inspect(fn.Body, func(n dst.Node) bool {
			exprStmt, ok := n.(*dst.ExprStmt)
			if !ok {
				return true
			}

			call, ok := exprStmt.X.(*dst.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*dst.SelectorExpr)
			if !ok {
				return true
			}

			methodName := sel.Sel.Name
			if !httpMethods[methodName] {
				return true
			}

			if len(call.Args) < 2 {
				return true
			}

			handlerArg := call.Args[len(call.Args)-1]

			if isAlreadyWrappedWithWhatapfasthttp(handlerArg) {
				return true
			}

			wrappedHandler := &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("whatapfasthttp"),
					Sel: dst.NewIdent("Func"),
				},
				Args: []dst.Expr{dst.Clone(handlerArg).(dst.Expr)},
			}

			call.Args[len(call.Args)-1] = wrappedHandler
			t.transformed = true
			return true
		})
	}
}

// injectServerHandlers wraps fasthttp.Server{Handler: handler} with whatapfasthttp.WrapHandler.
func (t *Transformer) injectServerHandlers(file *dst.File, pkgName string) {
	// Skip if handler method calls exist (individual handler wrapping already works)
	if hasHandlerMethodCalls(file) {
		return
	}

	// Process function bodies
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		dst.Inspect(fn.Body, func(n dst.Node) bool {
			t.wrapServerHandler(n, pkgName)
			return true
		})
	}

	// Process package-level declarations
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*dst.ValueSpec)
			if !ok {
				continue
			}

			for _, value := range valueSpec.Values {
				t.wrapServerHandler(value, pkgName)
			}
		}
	}
}

// wrapServerHandler wraps a single fasthttp.Server{Handler: handler} node.
func (t *Transformer) wrapServerHandler(n dst.Node, pkgName string) {
	var compositeLit *dst.CompositeLit
	if unary, ok := n.(*dst.UnaryExpr); ok && unary.Op == token.AND {
		if cl, ok := unary.X.(*dst.CompositeLit); ok {
			compositeLit = cl
		}
	} else if cl, ok := n.(*dst.CompositeLit); ok {
		compositeLit = cl
	}

	if compositeLit == nil {
		return
	}

	sel, ok := compositeLit.Type.(*dst.SelectorExpr)
	if !ok {
		return
	}
	ident, ok := sel.X.(*dst.Ident)
	if !ok || ident.Name != pkgName || sel.Sel.Name != "Server" {
		return
	}

	for _, elt := range compositeLit.Elts {
		kv, ok := elt.(*dst.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*dst.Ident)
		if !ok || keyIdent.Name != "Handler" {
			continue
		}

		// Skip if Handler: nil
		if valIdent, ok := kv.Value.(*dst.Ident); ok && valIdent.Name == "nil" {
			return
		}

		// Skip if already wrapped
		if isAlreadyWrappedWithWhatapfasthttp(kv.Value) {
			return
		}

		// Wrap: Handler: expr → Handler: whatapfasthttp.WrapHandler(expr)
		kv.Value = &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("whatapfasthttp"),
				Sel: dst.NewIdent("WrapHandler"),
			},
			Args: []dst.Expr{dst.Clone(kv.Value).(dst.Expr)},
		}
		t.transformed = true
		return
	}
}

// removeHandlerMethodWrappers removes whatapfasthttp.Func from router method handler args.
func (t *Transformer) removeHandlerMethodWrappers(file *dst.File) {
	httpMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true, "ANY": true,
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

			methodName := sel.Sel.Name
			if !httpMethods[methodName] {
				return true
			}

			if len(call.Args) < 2 {
				return true
			}

			lastArg := call.Args[len(call.Args)-1]
			if wrapCall, ok := lastArg.(*dst.CallExpr); ok {
				if wrapSel, ok := wrapCall.Fun.(*dst.SelectorExpr); ok {
					if wrapIdent, ok := wrapSel.X.(*dst.Ident); ok {
						if wrapIdent.Name == "whatapfasthttp" && wrapSel.Sel.Name == "Func" && len(wrapCall.Args) == 1 {
							call.Args[len(call.Args)-1] = wrapCall.Args[0]
						}
					}
				}
			}

			return true
		})
	}
}

// removeServerHandlerWrappers removes whatapfasthttp.WrapHandler from fasthttp.Server{Handler: ...}.
func (t *Transformer) removeServerHandlerWrappers(file *dst.File) {
	// Process function bodies
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		dst.Inspect(fn.Body, func(n dst.Node) bool {
			t.unwrapServerHandler(n)
			return true
		})
	}

	// Process package-level declarations
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*dst.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*dst.ValueSpec)
			if !ok {
				continue
			}

			for _, value := range valueSpec.Values {
				t.unwrapServerHandler(value)
			}
		}
	}
}

// unwrapServerHandler restores a single fasthttp.Server{Handler: whatapfasthttp.WrapHandler(h)} → Handler: h.
func (t *Transformer) unwrapServerHandler(n dst.Node) {
	var compositeLit *dst.CompositeLit
	if unary, ok := n.(*dst.UnaryExpr); ok && unary.Op == token.AND {
		if cl, ok := unary.X.(*dst.CompositeLit); ok {
			compositeLit = cl
		}
	} else if cl, ok := n.(*dst.CompositeLit); ok {
		compositeLit = cl
	}

	if compositeLit == nil {
		return
	}

	// Check type is *.Server (any package name)
	sel, ok := compositeLit.Type.(*dst.SelectorExpr)
	if !ok || sel.Sel.Name != "Server" {
		return
	}

	for _, elt := range compositeLit.Elts {
		kv, ok := elt.(*dst.KeyValueExpr)
		if !ok {
			continue
		}
		keyIdent, ok := kv.Key.(*dst.Ident)
		if !ok || keyIdent.Name != "Handler" {
			continue
		}

		// Check if whatapfasthttp.WrapHandler(originalHandler)
		call, ok := kv.Value.(*dst.CallExpr)
		if !ok {
			return
		}
		callSel, ok := call.Fun.(*dst.SelectorExpr)
		if !ok {
			return
		}
		callIdent, ok := callSel.X.(*dst.Ident)
		if !ok {
			return
		}
		if callIdent.Name == "whatapfasthttp" && callSel.Sel.Name == "WrapHandler" && len(call.Args) == 1 {
			kv.Value = dst.Clone(call.Args[0]).(dst.Expr)
		}
		return
	}
}

func isAlreadyWrappedWithWhatapfasthttp(expr dst.Expr) bool {
	call, ok := expr.(*dst.CallExpr)
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

	return ident.Name == "whatapfasthttp"
}
