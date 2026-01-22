// Package fasthttp provides the FastHTTP framework transformer.
package fasthttp

import (
	"go-api-inst/ast/common"

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
	return common.HasImport(file, t.ImportPath())
}

// Inject adds FastHTTP handler instrumentation.
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false
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
	return t.transformed, nil
}

// Remove removes FastHTTP handler instrumentation.
func (t *Transformer) Remove(file *dst.File) error {
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
	return nil
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
