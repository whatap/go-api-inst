// Package grpc provides the gRPC transformer.
package grpc

import (
	"go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for gRPC.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "grpc"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "google.golang.org/grpc"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/google.golang.org/grpc/whatapgrpc"
}

// Detect checks if the file uses gRPC.
func (t *Transformer) Detect(file *dst.File) bool {
	return common.HasImport(file, t.ImportPath())
}

// Inject adds whatapgrpc interceptors to grpc.NewServer() and grpc.Dial()/NewClient().
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
			if !ok || ident.Name != pkgName {
				return true
			}

			switch sel.Sel.Name {
			case "NewServer":
				// Add server interceptors
				call.Args = append(call.Args, t.createServerInterceptorArgs(pkgName)...)
				t.transformed = true
			case "Dial", "DialContext", "NewClient":
				// Add client interceptors
				call.Args = append(call.Args, t.createClientInterceptorArgs(pkgName)...)
				t.transformed = true
			}

			return true
		})
	}
	return t.transformed, nil
}

// Remove removes whatapgrpc interceptors from grpc calls.
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
			if !ok || ident.Name != "grpc" {
				return true
			}

			switch sel.Sel.Name {
			case "NewServer", "Dial", "DialContext", "NewClient":
				call.Args = t.filterGrpcArgs(call.Args)
			}

			return true
		})
	}
	return nil
}

// createServerInterceptorArgs creates server interceptor options.
// grpc.UnaryInterceptor(whatapgrpc.UnaryServerInterceptor())
// grpc.StreamInterceptor(whatapgrpc.StreamServerInterceptor())
func (t *Transformer) createServerInterceptorArgs(pkgName string) []dst.Expr {
	unaryInterceptor := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   dst.NewIdent(pkgName),
			Sel: dst.NewIdent("UnaryInterceptor"),
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("whatapgrpc"),
					Sel: dst.NewIdent("UnaryServerInterceptor"),
				},
			},
		},
	}

	streamInterceptor := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   dst.NewIdent(pkgName),
			Sel: dst.NewIdent("StreamInterceptor"),
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("whatapgrpc"),
					Sel: dst.NewIdent("StreamServerInterceptor"),
				},
			},
		},
	}

	return []dst.Expr{unaryInterceptor, streamInterceptor}
}

// createClientInterceptorArgs creates client interceptor options.
// grpc.WithUnaryInterceptor(whatapgrpc.UnaryClientInterceptor())
// grpc.WithStreamInterceptor(whatapgrpc.StreamClientInterceptor())
func (t *Transformer) createClientInterceptorArgs(pkgName string) []dst.Expr {
	unaryInterceptor := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   dst.NewIdent(pkgName),
			Sel: dst.NewIdent("WithUnaryInterceptor"),
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("whatapgrpc"),
					Sel: dst.NewIdent("UnaryClientInterceptor"),
				},
			},
		},
	}

	streamInterceptor := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   dst.NewIdent(pkgName),
			Sel: dst.NewIdent("WithStreamInterceptor"),
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("whatapgrpc"),
					Sel: dst.NewIdent("StreamClientInterceptor"),
				},
			},
		},
	}

	return []dst.Expr{unaryInterceptor, streamInterceptor}
}

// filterGrpcArgs removes whatapgrpc interceptor arguments.
func (t *Transformer) filterGrpcArgs(args []dst.Expr) []dst.Expr {
	var filtered []dst.Expr
	for _, arg := range args {
		if !t.isWhatapGrpcInterceptor(arg) {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}

// isWhatapGrpcInterceptor checks if arg is a whatapgrpc interceptor.
func (t *Transformer) isWhatapGrpcInterceptor(expr dst.Expr) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok || ident.Name != "grpc" {
		return false
	}

	// Check for grpc.UnaryInterceptor, grpc.StreamInterceptor,
	// grpc.WithUnaryInterceptor, grpc.WithStreamInterceptor
	interceptorFuncs := map[string]bool{
		"UnaryInterceptor":      true,
		"StreamInterceptor":     true,
		"WithUnaryInterceptor":  true,
		"WithStreamInterceptor": true,
	}

	if !interceptorFuncs[sel.Sel.Name] {
		return false
	}

	// Check if argument is whatapgrpc.*
	if len(call.Args) == 0 {
		return false
	}

	argCall, ok := call.Args[0].(*dst.CallExpr)
	if !ok {
		return false
	}

	argSel, ok := argCall.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	argIdent, ok := argSel.X.(*dst.Ident)
	if !ok {
		return false
	}

	return argIdent.Name == "whatapgrpc"
}
