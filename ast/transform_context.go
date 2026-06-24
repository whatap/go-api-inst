package ast

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/dave/dst"
	"github.com/whatap/go-api-inst/ast/common"
)

// evalTemplateStmts evaluates a Go text/template against a TransformContext and parses
// the result into dst statements. Returns (nil, nil) for empty code, or an error for
// template parse/execute failures.
//
// Used by Transform, Hook, and Inject Advice to turn user-provided code strings into
// AST nodes with template variables like {{.FuncName}}, {{.ArgsList}}, {{.HasCtx}}.
func evalTemplateStmts(tc TransformContext, code string) ([]dst.Stmt, error) {
	if strings.TrimSpace(code) == "" {
		return nil, nil
	}
	tmpl, err := template.New("advice").Parse(code)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tc); err != nil {
		return nil, err
	}
	return parseCodeBlock(buf.String())
}

// ArgAt returns the i-th argument as a string, or "" if i is out of range.
// Used from templates as {{.ArgAt 4}}.
func (tc TransformContext) ArgAt(i int) string {
	if i < 0 || i >= len(tc.ArgsList) {
		return ""
	}
	return tc.ArgsList[i]
}

// buildCallTransformContext constructs a TransformContext for call-site Advice
// (Transform, Hook). Populates all template variables including §227 Q6 additions
// (ArgsList, ArgCount, HasCtx, File).
//
// Requires ctx.Call to be set.
func buildCallTransformContext(ctx *MatchContext) TransformContext {
	tc := TransformContext{
		FuncName: ctx.FuncName,
		PkgName:  ctx.PkgName,
	}

	if ctx.Call != nil {
		tc.Original = nodeToString(ctx.Call)
		tc.Args = argsToString(ctx.Call.Args)
		tc.ArgCount = len(ctx.Call.Args)
		tc.ArgsList = make([]string, len(ctx.Call.Args))
		for i, a := range ctx.Call.Args {
			tc.ArgsList[i] = nodeToString(a)
		}
		if len(ctx.Call.Args) > 0 {
			tc.Arg0 = tc.ArgsList[0]
		}
		if len(ctx.Call.Args) > 1 {
			tc.Arg1 = tc.ArgsList[1]
		}
		if len(ctx.Call.Args) > 2 {
			tc.Arg2 = tc.ArgsList[2]
		}
		if len(ctx.Call.Args) > 3 {
			tc.Arg3 = tc.ArgsList[3]
		}
		tc.HasCtx = callHasContextArg(ctx.Call)
		tc.IsSpread = ctx.Call.Ellipsis
		if len(ctx.Call.Args) > 1 {
			rest := tc.ArgsList[1:]
			joined := strings.Join(rest, ", ")
			if ctx.Call.Ellipsis {
				// Preserve `...` so the substituted call still passes the
				// slice as variadic to the wrapped helper.
				joined += "..."
			}
			tc.Args1Plus = joined
		}
	}

	if ctx.Sel != nil {
		tc.Receiver = nodeToString(ctx.Sel.X)
	}

	tc.TargetPkg = resolveTargetPkgAlias(ctx)

	// Ambient handler context (used by aerospike template etc.)
	ctxExpr := detectCtxExpr(ctx)
	ctxStr := nodeToString(ctxExpr)
	if ctxStr == "nil" {
		tc.Ctx = "context.Background()"
	} else {
		tc.Ctx = ctxStr
	}

	// Var/Var1: extract from enclosing assignment
	if assign, ok := ctx.EnclosingStmt.(*dst.AssignStmt); ok {
		if len(assign.Lhs) > 0 {
			if ident, ok := assign.Lhs[0].(*dst.Ident); ok {
				tc.Var = ident.Name
			}
		}
		if len(assign.Lhs) > 1 {
			if ident, ok := assign.Lhs[1].(*dst.Ident); ok {
				tc.Var1 = ident.Name
			}
		}
	}

	tc.File = fileRelPath(ctx.File)

	return tc
}

// buildDeclTransformContext constructs a TransformContext for FuncDecl-site Advice
// (Inject). Populates template variables for function declaration matching.
//
// Requires ctx.Decl to be set.
func buildDeclTransformContext(ctx *MatchContext) TransformContext {
	tc := TransformContext{
		FuncName: ctx.FuncName,
		PkgName:  ctx.PkgName,
	}

	if ctx.Decl != nil && ctx.Decl.Type != nil && ctx.Decl.Type.Params != nil {
		params := ctx.Decl.Type.Params.List
		// Flatten: one param field may declare multiple names (e.g. `a, b int`).
		var names []string
		for _, p := range params {
			if len(p.Names) == 0 {
				names = append(names, nodeToString(p.Type))
				continue
			}
			for _, n := range p.Names {
				names = append(names, n.Name)
			}
		}
		tc.ArgsList = names
		tc.ArgCount = len(names)
		if len(names) > 0 {
			tc.Arg0 = names[0]
		}
		if len(names) > 1 {
			tc.Arg1 = names[1]
		}
		if len(names) > 2 {
			tc.Arg2 = names[2]
		}
		if len(names) > 3 {
			tc.Arg3 = names[3]
		}
		tc.Args = strings.Join(names, ", ")
		tc.HasCtx = declHasContextParam(ctx.Decl)
	}

	// Receiver: method receiver variable name
	if ctx.Decl != nil && ctx.Decl.Recv != nil && len(ctx.Decl.Recv.List) > 0 {
		recv := ctx.Decl.Recv.List[0]
		if len(recv.Names) > 0 {
			tc.Receiver = recv.Names[0].Name
		}
	}

	tc.File = fileRelPath(ctx.File)

	return tc
}

// callHasContextArg returns true if any argument of the call has type context.Context.
// Uses go/types when available; otherwise falls back to a conservative name check.
func callHasContextArg(call *dst.CallExpr) bool {
	if call == nil {
		return false
	}
	if common.HasTypeInfo() {
		for _, arg := range call.Args {
			t := common.ResolveType(arg)
			if t == nil {
				continue
			}
			if isContextType(t.String()) {
				return true
			}
		}
		return false
	}
	// go/types unavailable — syntactic check: argument named "ctx" or a call that looks like r.Context()
	for _, arg := range call.Args {
		if id, ok := arg.(*dst.Ident); ok {
			if id.Name == "ctx" {
				return true
			}
		}
	}
	return false
}

// declHasContextParam returns true if the function declaration has a context.Context parameter.
func declHasContextParam(fn *dst.FuncDecl) bool {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil {
		return false
	}
	for _, p := range fn.Type.Params.List {
		// *dst.SelectorExpr: context.Context
		if sel, ok := p.Type.(*dst.SelectorExpr); ok {
			if ident, ok := sel.X.(*dst.Ident); ok {
				if ident.Name == "context" && sel.Sel.Name == "Context" {
					return true
				}
			}
		}
	}
	return false
}

// isContextType returns true if the go/types type string represents context.Context.
// Accepts the canonical form "context.Context" (non-nil).
func isContextType(typeStr string) bool {
	if typeStr == "" {
		return false
	}
	// types.Type.String() returns e.g. "context.Context" for the interface.
	return typeStr == "context.Context" || strings.HasSuffix(typeStr, " context.Context")
}

// fileRelPath returns a reasonable file identifier for {{.File}} templates.
// Uses the AST file name if present; empty otherwise.
func fileRelPath(file *dst.File) string {
	if file == nil || file.Name == nil {
		return ""
	}
	return file.Name.Name
}
