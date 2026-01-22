// Package nethttp provides the net/http transformer.
package nethttp

import (
	"go/token"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements ast.Transformer for net/http.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "nethttp"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "net/http"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
}

// Detect checks if the file uses net/http with handlers or clients.
func (t *Transformer) Detect(file *dst.File) bool {
	if !common.HasImport(file, t.ImportPath()) {
		return false
	}

	// Get the actual package name (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false
	}

	// Exclude files using gorilla mux (handled by whatapmux)
	if common.HasImport(file, "github.com/gorilla/mux") {
		// Still apply if http.Client or http.Get client calls exist even with gorilla mux
		return hasHttpClientCalls(file, pkgName)
	}

	// Only detect if there are actual handler calls or client calls
	return hasHttpHandlerCalls(file) || hasHttpClientCalls(file, pkgName)
}

// Inject adds net/http handler and client instrumentation.
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false

	// Get the actual package name (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

	t.injectHandlers(file)
	t.injectClients(file, pkgName)

	// Remove net/http import if no longer used after transformation (§59/§60 fix)
	// This happens when all http.Get/Post calls are transformed to whataphttp.HttpGet/HttpPost
	if t.transformed {
		common.RemoveImportIfUnused(file, t.ImportPath(), pkgName)
	}

	return t.transformed, nil
}

// Remove removes net/http instrumentation.
func (t *Transformer) Remove(file *dst.File) error {
	t.removeHandlerWrappers(file)
	t.removeClientWrappers(file)
	return nil
}

// injectHandlers wraps HandleFunc calls with whataphttp.Func.
// NOTE: Handle() is not supported because whataphttp.Handler doesn't exist in go-api.
func (t *Transformer) injectHandlers(file *dst.File) {
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
			// NOTE: Only HandleFunc is supported. Handle() requires whataphttp.Handler
			// which doesn't exist in go-api yet. See ISSUES.md §56.
			if methodName != "HandleFunc" {
				return true
			}

			if len(call.Args) < 2 {
				return true
			}

			handlerArg := call.Args[len(call.Args)-1]

			if isAlreadyWrappedWithWhataphttp(handlerArg) {
				return true
			}

			// HandleFunc → whataphttp.Func
			wrapFunc := "Func"

			wrappedHandler := &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent("whataphttp"),
					Sel: dst.NewIdent(wrapFunc),
				},
				Args: []dst.Expr{dst.Clone(handlerArg).(dst.Expr)},
			}

			call.Args[len(call.Args)-1] = wrappedHandler
			t.transformed = true
			return true
		})
	}
}

// injectClients wraps http.Get/Post/Client calls.
// http.Get/Post/PostForm → whataphttp.HttpGet/HttpPost/HttpPostForm
// http.DefaultClient.Get → whataphttp.DefaultClientGet (marker function)
// http.Client{Transport: t} → http.Client{Transport: whataphttp.NewRoundTrip(ctx, t)}
// http.Client{} → http.Client{Transport: whataphttp.NewRoundTripWithEmptyTransport(ctx)}
//
// Context detection: If the function has a recognizable handler signature,
// uses the handler's context instead of context.Background().
// Supports: net/http, Gin, Echo, Fiber, FastHTTP handlers.
// Also detects context from anonymous functions (FuncLit) commonly used as framework handlers.
// pkgName is the actual package name used in code (could be alias).
func (t *Transformer) injectClients(file *dst.File, pkgName string) {
	clientFuncs := map[string]string{
		"Get":      "HttpGet",
		"Post":     "HttpPost",
		"PostForm": "HttpPostForm",
	}

	// processFunc handles HTTP client injection for a function body with given handler context
	// This includes both http.Get/Post and http.Client{} initialization
	var processFunc func(body *dst.BlockStmt, handlerCtx dst.Expr)
	processFunc = func(body *dst.BlockStmt, handlerCtx dst.Expr) {
		if body == nil {
			return
		}

		// Transform HTTP calls and handle http.Client{} initialization
		dst.Inspect(body, func(n dst.Node) bool {
			// Check for nested FuncLit (anonymous function) - process with its own context
			if funcLit, ok := n.(*dst.FuncLit); ok {
				nestedCtx := detectHandlerContextFromFuncType(funcLit.Type, pkgName)
				processFunc(funcLit.Body, nestedCtx)
				return false // Don't recurse into FuncLit body again
			}

			// Case: http.Get/Post/PostForm, http.DefaultClient.*
			if call, ok := n.(*dst.CallExpr); ok {
				if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
					// Case 1: Transform http.Get/Post/PostForm calls
					if ident, ok := sel.X.(*dst.Ident); ok {
						if ident.Name == pkgName {
							if whatapFunc, exists := clientFuncs[sel.Sel.Name]; exists {
								ident.Name = "whataphttp"
								sel.Sel.Name = whatapFunc
								ctxArg := createContextExpr(file, handlerCtx)
								call.Args = append([]dst.Expr{ctxArg}, call.Args...)
								t.transformed = true
							}
						}
					}

					// Case 2: Transform http.DefaultClient.Get/Post/PostForm calls
					defaultClientFuncs := map[string]string{
						"Get":      "DefaultClientGet",
						"Post":     "DefaultClientPost",
						"PostForm": "DefaultClientPostForm",
					}
					if innerSel, ok := sel.X.(*dst.SelectorExpr); ok {
						if ident, ok := innerSel.X.(*dst.Ident); ok {
							if ident.Name == pkgName && innerSel.Sel.Name == "DefaultClient" {
								if whatapFunc, exists := defaultClientFuncs[sel.Sel.Name]; exists {
									call.Fun = &dst.SelectorExpr{
										X:   dst.NewIdent("whataphttp"),
										Sel: dst.NewIdent(whatapFunc),
									}
									ctxArg := createContextExpr(file, handlerCtx)
									call.Args = append([]dst.Expr{ctxArg}, call.Args...)
									t.transformed = true
								}
							}
						}
					}
				}
			}

			// Case: http.Client{} initialization
			var compositeLit *dst.CompositeLit
			if unary, ok := n.(*dst.UnaryExpr); ok && unary.Op == token.AND {
				if cl, ok := unary.X.(*dst.CompositeLit); ok {
					compositeLit = cl
				}
			} else if cl, ok := n.(*dst.CompositeLit); ok {
				compositeLit = cl
			}

			if compositeLit != nil {
				if sel, ok := compositeLit.Type.(*dst.SelectorExpr); ok {
					if ident, ok := sel.X.(*dst.Ident); ok {
						if ident.Name == pkgName && sel.Sel.Name == "Client" {
							if !isHttpClientAlreadyWrapped(compositeLit) {
								transportFound := false
								for _, elt := range compositeLit.Elts {
									if kv, ok := elt.(*dst.KeyValueExpr); ok {
										if keyIdent, ok := kv.Key.(*dst.Ident); ok && keyIdent.Name == "Transport" {
											kv.Value = &dst.CallExpr{
												Fun: &dst.SelectorExpr{
													X:   dst.NewIdent("whataphttp"),
													Sel: dst.NewIdent("NewRoundTrip"),
												},
												Args: []dst.Expr{
													createContextExpr(file, handlerCtx),
													dst.Clone(kv.Value).(dst.Expr),
												},
											}
											transportFound = true
											t.transformed = true
											break
										}
									}
								}
								if !transportFound {
									newKV := &dst.KeyValueExpr{
										Key: dst.NewIdent("Transport"),
										Value: &dst.CallExpr{
											Fun: &dst.SelectorExpr{
												X:   dst.NewIdent("whataphttp"),
												Sel: dst.NewIdent("NewRoundTripWithEmptyTransport"),
											},
											Args: []dst.Expr{
												createContextExpr(file, handlerCtx),
											},
										},
									}
									compositeLit.Elts = append(compositeLit.Elts, newKV)
									t.transformed = true
								}
							}
						}
					}
				}
			}

			return true
		})
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		// Detect handler context from function parameters
		handlerCtx := detectHandlerContext(fn, pkgName)

		// Process HTTP calls and http.Client{} in the function
		processFunc(fn.Body, handlerCtx)
	}
}

// isHttpClientAlreadyWrapped checks if http.Client's Transport is already wrapped with whataphttp
func isHttpClientAlreadyWrapped(compositeLit *dst.CompositeLit) bool {
	for _, elt := range compositeLit.Elts {
		kv, ok := elt.(*dst.KeyValueExpr)
		if !ok {
			continue
		}

		keyIdent, ok := kv.Key.(*dst.Ident)
		if !ok || keyIdent.Name != "Transport" {
			continue
		}

		// Check if whataphttp.NewRoundTrip(...)
		call, ok := kv.Value.(*dst.CallExpr)
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

		return ident.Name == "whataphttp"
	}
	return false
}

// removeHandlerWrappers removes whataphttp.Func/Handler wrappers.
func (t *Transformer) removeHandlerWrappers(file *dst.File) {
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
			if methodName != "HandleFunc" && methodName != "Handle" {
				return true
			}

			if len(call.Args) < 2 {
				return true
			}

			lastArg := call.Args[len(call.Args)-1]
			if wrapCall, ok := lastArg.(*dst.CallExpr); ok {
				if wrapSel, ok := wrapCall.Fun.(*dst.SelectorExpr); ok {
					if wrapIdent, ok := wrapSel.X.(*dst.Ident); ok {
						if wrapIdent.Name == "whataphttp" && len(wrapCall.Args) == 1 {
							call.Args[len(call.Args)-1] = wrapCall.Args[0]
						}
					}
				}
			}

			return true
		})
	}
}

// removeClientWrappers removes whataphttp HTTP client wrappers.
func (t *Transformer) removeClientWrappers(file *dst.File) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		dst.Inspect(fn.Body, func(n dst.Node) bool {
			// Restore whataphttp.HttpGet -> http.Get
			if call, ok := n.(*dst.CallExpr); ok {
				t.restoreHttpCall(call)
			}

			// Remove Transport from http.Client{}
			if compositeLit, ok := n.(*dst.CompositeLit); ok {
				t.removeTransportField(compositeLit)
			}

			return true
		})
	}
}

// restoreHttpCall restores whataphttp.HttpGet to http.Get.
func (t *Transformer) restoreHttpCall(call *dst.CallExpr) {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok || ident.Name != "whataphttp" {
		return
	}

	// Case 1: http.Get/Post/PostForm
	httpFuncs := map[string]string{
		"HttpGet":      "Get",
		"HttpPost":     "Post",
		"HttpPostForm": "PostForm",
		"HttpHead":     "Head",
	}

	if originalFunc, ok := httpFuncs[sel.Sel.Name]; ok {
		// Restore to http.Xxx and remove first argument (context)
		call.Fun = &dst.SelectorExpr{
			X:   dst.NewIdent("http"),
			Sel: dst.NewIdent(originalFunc),
		}
		if len(call.Args) > 0 {
			call.Args = call.Args[1:]
		}
		return
	}

	// Case 2: http.DefaultClient.Get/Post/PostForm (marker functions)
	defaultClientFuncs := map[string]string{
		"DefaultClientGet":      "Get",
		"DefaultClientPost":     "Post",
		"DefaultClientPostForm": "PostForm",
	}

	if originalMethod, ok := defaultClientFuncs[sel.Sel.Name]; ok {
		// Restore to http.DefaultClient.Xxx and remove first argument (context)
		call.Fun = &dst.SelectorExpr{
			X: &dst.SelectorExpr{
				X:   dst.NewIdent("http"),
				Sel: dst.NewIdent("DefaultClient"),
			},
			Sel: dst.NewIdent(originalMethod),
		}
		if len(call.Args) > 0 {
			call.Args = call.Args[1:]
		}
	}
}

// removeTransportField removes/restores whataphttp Transport from http.Client{}.
// - NewRoundTripWithEmptyTransport(ctx) → remove Transport field entirely
// - NewRoundTrip(ctx, originalTransport) → restore to Transport: originalTransport
func (t *Transformer) removeTransportField(compositeLit *dst.CompositeLit) {
	sel, ok := compositeLit.Type.(*dst.SelectorExpr)
	if !ok {
		return
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok || ident.Name != "http" || sel.Sel.Name != "Client" {
		return
	}

	var newElts []dst.Expr
	for _, elt := range compositeLit.Elts {
		if kv, ok := elt.(*dst.KeyValueExpr); ok {
			if key, ok := kv.Key.(*dst.Ident); ok && key.Name == "Transport" {
				if call, ok := kv.Value.(*dst.CallExpr); ok {
					if callSel, ok := call.Fun.(*dst.SelectorExpr); ok {
						if callIdent, ok := callSel.X.(*dst.Ident); ok && callIdent.Name == "whataphttp" {
							// NewRoundTripWithEmptyTransport → remove field
							if callSel.Sel.Name == "NewRoundTripWithEmptyTransport" {
								continue // Remove this field entirely
							}
							// NewRoundTrip(ctx, originalTransport) → restore originalTransport
							if callSel.Sel.Name == "NewRoundTrip" && len(call.Args) >= 2 {
								kv.Value = dst.Clone(call.Args[1]).(dst.Expr)
								newElts = append(newElts, elt)
								continue
							}
						}
					}
				}
			}
		}
		newElts = append(newElts, elt)
	}
	compositeLit.Elts = newElts
}

// Helper functions

func isAlreadyWrappedWithWhataphttp(expr dst.Expr) bool {
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

	return ident.Name == "whataphttp"
}

func isWhataphttpTransport(expr dst.Expr) bool {
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

	// Recognize both NewRoundTrip and NewRoundTripWithEmptyTransport
	return ident.Name == "whataphttp" &&
		(sel.Sel.Name == "NewRoundTrip" || sel.Sel.Name == "NewRoundTripWithEmptyTransport")
}

func hasHttpHandlerCalls(file *dst.File) bool {
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

			call, ok := n.(*dst.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*dst.SelectorExpr)
			if !ok {
				return true
			}

			if sel.Sel.Name == "HandleFunc" || sel.Sel.Name == "Handle" {
				found = true
				return false
			}

			return true
		})
	}
	return found
}

// hasHttpClientCalls checks if file has http client calls.
// pkgName is the actual package name used in code (could be alias).
func hasHttpClientCalls(file *dst.File, pkgName string) bool {
	clientFuncs := map[string]bool{
		"Get": true, "Post": true, "PostForm": true, "Head": true,
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

			// Check http.Get/Post/PostForm or http.DefaultClient.Get/Post/PostForm
			if call, ok := n.(*dst.CallExpr); ok {
				if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
					// http.Get/Post/PostForm
					if ident, ok := sel.X.(*dst.Ident); ok {
						if ident.Name == pkgName && clientFuncs[sel.Sel.Name] {
							found = true
							return false
						}
					}
					// http.DefaultClient.Get/Post/PostForm
					if innerSel, ok := sel.X.(*dst.SelectorExpr); ok {
						if ident, ok := innerSel.X.(*dst.Ident); ok {
							if ident.Name == pkgName && innerSel.Sel.Name == "DefaultClient" && clientFuncs[sel.Sel.Name] {
								found = true
								return false
							}
						}
					}
				}
			}

			// Check http.Client{} initialization
			var compositeLit *dst.CompositeLit
			if unary, ok := n.(*dst.UnaryExpr); ok && unary.Op == token.AND {
				if cl, ok := unary.X.(*dst.CompositeLit); ok {
					compositeLit = cl
				}
			} else if cl, ok := n.(*dst.CompositeLit); ok {
				compositeLit = cl
			}

			if compositeLit != nil {
				if sel, ok := compositeLit.Type.(*dst.SelectorExpr); ok {
					if ident, ok := sel.X.(*dst.Ident); ok {
						if ident.Name == pkgName && sel.Sel.Name == "Client" {
							found = true
							return false
						}
					}
				}
			}

			return true
		})
	}
	return found
}

// detectHandlerContext detects the context source from handler function parameters.
// Returns the AST expression for context accessor, or nil if not detectable.
// Supports:
// - net/http: func(w http.ResponseWriter, r *http.Request) → r.Context()
// - Gin: func(c *gin.Context) → c.Request.Context()
// - Echo: func(c echo.Context) → c.Request().Context()
// - Fiber: func(c *fiber.Ctx) → c.UserContext()
// - FastHTTP: func(ctx *fasthttp.RequestCtx) → ctx
// - context.Context: func(ctx context.Context) → ctx
// httpPkgName is the actual http package name used in code (could be alias).
func detectHandlerContext(fn *dst.FuncDecl, httpPkgName string) dst.Expr {
	if fn.Type == nil || fn.Type.Params == nil {
		return nil
	}

	params := fn.Type.Params.List

	// First, check for context.Context parameter (highest priority for helper functions)
	for _, param := range params {
		if len(param.Names) == 0 {
			continue
		}
		// Skip blank identifier `_` - cannot be used as value (§60 fix)
		if param.Names[0].Name == "_" {
			continue
		}
		if sel, ok := param.Type.(*dst.SelectorExpr); ok {
			if ident, ok := sel.X.(*dst.Ident); ok {
				if ident.Name == "context" && sel.Sel.Name == "Context" {
					return dst.NewIdent(param.Names[0].Name)
				}
			}
		}
	}

	// Check for net/http handler: func(w http.ResponseWriter, r *http.Request)
	if len(params) >= 2 {
		// Look for *http.Request parameter
		for _, param := range params {
			if star, ok := param.Type.(*dst.StarExpr); ok {
				if sel, ok := star.X.(*dst.SelectorExpr); ok {
					if ident, ok := sel.X.(*dst.Ident); ok {
						if ident.Name == httpPkgName && sel.Sel.Name == "Request" {
							// Found *http.Request, return paramName.Context()
							// Skip blank identifier `_` (§60 fix)
							if len(param.Names) > 0 && param.Names[0].Name != "_" {
								return &dst.CallExpr{
									Fun: &dst.SelectorExpr{
										X:   dst.NewIdent(param.Names[0].Name),
										Sel: dst.NewIdent("Context"),
									},
								}
							}
						}
					}
				}
			}
		}
	}

	// Check for framework handlers with single context parameter
	for _, param := range params {
		if len(param.Names) == 0 {
			continue
		}
		// Skip blank identifier `_` (§60 fix)
		if param.Names[0].Name == "_" {
			continue
		}
		paramName := param.Names[0].Name

		// Gin: *gin.Context → c.Request.Context()
		if star, ok := param.Type.(*dst.StarExpr); ok {
			if sel, ok := star.X.(*dst.SelectorExpr); ok {
				if ident, ok := sel.X.(*dst.Ident); ok {
					if ident.Name == "gin" && sel.Sel.Name == "Context" {
						return &dst.CallExpr{
							Fun: &dst.SelectorExpr{
								X: &dst.SelectorExpr{
									X:   dst.NewIdent(paramName),
									Sel: dst.NewIdent("Request"),
								},
								Sel: dst.NewIdent("Context"),
							},
						}
					}
					// Fiber: *fiber.Ctx → c.UserContext()
					if ident.Name == "fiber" && sel.Sel.Name == "Ctx" {
						return &dst.CallExpr{
							Fun: &dst.SelectorExpr{
								X:   dst.NewIdent(paramName),
								Sel: dst.NewIdent("UserContext"),
							},
						}
					}
					// FastHTTP: *fasthttp.RequestCtx → ctx (already a context)
					if ident.Name == "fasthttp" && sel.Sel.Name == "RequestCtx" {
						return dst.NewIdent(paramName)
					}
				}
			}
		}

		// Echo: echo.Context (not a pointer) → c.Request().Context()
		if sel, ok := param.Type.(*dst.SelectorExpr); ok {
			if ident, ok := sel.X.(*dst.Ident); ok {
				if ident.Name == "echo" && sel.Sel.Name == "Context" {
					return &dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X: &dst.CallExpr{
								Fun: &dst.SelectorExpr{
									X:   dst.NewIdent(paramName),
									Sel: dst.NewIdent("Request"),
								},
							},
							Sel: dst.NewIdent("Context"),
						},
					}
				}
			}
		}
	}

	return nil
}

// createContextExpr creates the context expression to use.
// Uses detected handler context if available, otherwise returns nil.
// go-api functions handle nil context gracefully, avoiding context import conflicts.
func createContextExpr(file *dst.File, handlerCtx dst.Expr) dst.Expr {
	if handlerCtx != nil {
		return dst.Clone(handlerCtx).(dst.Expr)
	}

	// Return nil instead of context.Background() to avoid context import conflicts
	// (e.g., when another package uses "context" as its name like "code.gitea.io/gitea/services/context")
	return dst.NewIdent("nil")
}

// detectHandlerContextFromFuncType detects handler context from FuncType (for anonymous functions).
// This is used for FuncLit (anonymous function literals) commonly used as framework handlers.
// Supports: net/http, Gin, Echo, Fiber, FastHTTP handlers, and context.Context parameter.
// httpPkgName is the actual http package name used in code (could be alias).
func detectHandlerContextFromFuncType(ft *dst.FuncType, httpPkgName string) dst.Expr {
	if ft == nil || ft.Params == nil {
		return nil
	}

	params := ft.Params.List

	// First, check for context.Context parameter (highest priority for helper functions)
	for _, param := range params {
		if len(param.Names) == 0 {
			continue
		}
		// Skip blank identifier `_` - cannot be used as value (§60 fix)
		if param.Names[0].Name == "_" {
			continue
		}
		if sel, ok := param.Type.(*dst.SelectorExpr); ok {
			if ident, ok := sel.X.(*dst.Ident); ok {
				if ident.Name == "context" && sel.Sel.Name == "Context" {
					return dst.NewIdent(param.Names[0].Name)
				}
			}
		}
	}

	// Check for net/http handler: func(w http.ResponseWriter, r *http.Request)
	if len(params) >= 2 {
		for _, param := range params {
			if star, ok := param.Type.(*dst.StarExpr); ok {
				if sel, ok := star.X.(*dst.SelectorExpr); ok {
					if ident, ok := sel.X.(*dst.Ident); ok {
						if ident.Name == httpPkgName && sel.Sel.Name == "Request" {
							// Skip blank identifier `_` (§60 fix)
							if len(param.Names) > 0 && param.Names[0].Name != "_" {
								return &dst.CallExpr{
									Fun: &dst.SelectorExpr{
										X:   dst.NewIdent(param.Names[0].Name),
										Sel: dst.NewIdent("Context"),
									},
								}
							}
						}
					}
				}
			}
		}
	}

	// Check for framework handlers
	for _, param := range params {
		if len(param.Names) == 0 {
			continue
		}
		// Skip blank identifier `_` (§60 fix)
		if param.Names[0].Name == "_" {
			continue
		}
		paramName := param.Names[0].Name

		// Gin: *gin.Context → c.Request.Context()
		if star, ok := param.Type.(*dst.StarExpr); ok {
			if sel, ok := star.X.(*dst.SelectorExpr); ok {
				if ident, ok := sel.X.(*dst.Ident); ok {
					if ident.Name == "gin" && sel.Sel.Name == "Context" {
						return &dst.CallExpr{
							Fun: &dst.SelectorExpr{
								X: &dst.SelectorExpr{
									X:   dst.NewIdent(paramName),
									Sel: dst.NewIdent("Request"),
								},
								Sel: dst.NewIdent("Context"),
							},
						}
					}
					// Fiber: *fiber.Ctx → c.UserContext()
					if ident.Name == "fiber" && sel.Sel.Name == "Ctx" {
						return &dst.CallExpr{
							Fun: &dst.SelectorExpr{
								X:   dst.NewIdent(paramName),
								Sel: dst.NewIdent("UserContext"),
							},
						}
					}
					// FastHTTP: *fasthttp.RequestCtx → ctx
					if ident.Name == "fasthttp" && sel.Sel.Name == "RequestCtx" {
						return dst.NewIdent(paramName)
					}
				}
			}
		}

		// Echo: echo.Context (not a pointer) → c.Request().Context()
		if sel, ok := param.Type.(*dst.SelectorExpr); ok {
			if ident, ok := sel.X.(*dst.Ident); ok {
				if ident.Name == "echo" && sel.Sel.Name == "Context" {
					return &dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X: &dst.CallExpr{
								Fun: &dst.SelectorExpr{
									X:   dst.NewIdent(paramName),
									Sel: dst.NewIdent("Request"),
								},
							},
							Sel: dst.NewIdent("Context"),
						},
					}
				}
			}
		}
	}

	return nil
}
