package common

import "github.com/dave/dst"

// DetectHandlerContext detects the context expression from a function declaration's parameters.
// Searches for context.Context, *http.Request, and framework-specific handler contexts.
// Returns nil if no context is found.
//
// Priority order:
//  1. context.Context parameter → paramName
//  2. *http.Request parameter → paramName.Context()
//  3. *gin.Context → paramName.Request.Context()
//  4. *fiber.Ctx → paramName.UserContext()
//  5. *fasthttp.RequestCtx → paramName
//  6. echo.Context → paramName.Request().Context()
func DetectHandlerContext(fn *dst.FuncDecl) dst.Expr {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil {
		return nil
	}
	return detectHandlerContextFromParams(fn.Type.Params.List)
}

// DetectHandlerContextFromFuncType detects handler context from FuncType (for anonymous functions).
func DetectHandlerContextFromFuncType(ft *dst.FuncType) dst.Expr {
	if ft == nil || ft.Params == nil {
		return nil
	}
	return detectHandlerContextFromParams(ft.Params.List)
}

func detectHandlerContextFromParams(params []*dst.Field) dst.Expr {
	// 1. Check for explicit context.Context parameter (highest priority)
	for _, param := range params {
		if len(param.Names) == 0 || param.Names[0].Name == "_" {
			continue
		}
		if sel, ok := param.Type.(*dst.SelectorExpr); ok {
			if ident, ok := sel.X.(*dst.Ident); ok {
				if MatchIdentPkg(ident, "context", "context") && sel.Sel.Name == "Context" {
					return dst.NewIdent(param.Names[0].Name)
				}
			}
		}
	}

	// 2. Check for *http.Request parameter → r.Context()
	if len(params) >= 2 {
		for _, param := range params {
			if star, ok := param.Type.(*dst.StarExpr); ok {
				if sel, ok := star.X.(*dst.SelectorExpr); ok {
					if ident, ok := sel.X.(*dst.Ident); ok {
						if MatchIdentPkg(ident, "http", "net/http") && sel.Sel.Name == "Request" {
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

	// 3. Check framework-specific handler contexts
	for _, param := range params {
		if len(param.Names) == 0 || param.Names[0].Name == "_" {
			continue
		}
		paramName := param.Names[0].Name

		// Pointer types: *gin.Context, *fiber.Ctx, *fasthttp.RequestCtx
		if star, ok := param.Type.(*dst.StarExpr); ok {
			if sel, ok := star.X.(*dst.SelectorExpr); ok {
				if ident, ok := sel.X.(*dst.Ident); ok {
					// Gin: *gin.Context → c.Request.Context()
					if MatchIdentPkg(ident, "gin", "github.com/gin-gonic/gin") && sel.Sel.Name == "Context" {
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
					if MatchIdentPkg(ident, "fiber", "github.com/gofiber/fiber") && sel.Sel.Name == "Ctx" {
						return &dst.CallExpr{
							Fun: &dst.SelectorExpr{
								X:   dst.NewIdent(paramName),
								Sel: dst.NewIdent("UserContext"),
							},
						}
					}
					// FastHTTP: *fasthttp.RequestCtx → ctx
					if MatchIdentPkg(ident, "fasthttp", "github.com/valyala/fasthttp") && sel.Sel.Name == "RequestCtx" {
						return dst.NewIdent(paramName)
					}
				}
			}
		}

		// Non-pointer types: echo.Context
		if sel, ok := param.Type.(*dst.SelectorExpr); ok {
			if ident, ok := sel.X.(*dst.Ident); ok {
				if MatchIdentPkg(ident, "echo", "github.com/labstack/echo") && sel.Sel.Name == "Context" {
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
