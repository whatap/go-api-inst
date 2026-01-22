package custom

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"go-api-inst/ast/common"
	"go-api-inst/config"

	"github.com/dave/dst"
)

// TransformContext holds template variables
type TransformContext struct {
	Original string // Full original code (e.g., "gin.Default()")
	Var      string // Assigned variable name (e.g., "r")
	Args     string // Full function arguments
	Arg0     string // First argument
	Arg1     string // Second argument
	Arg2     string // Third argument
	FuncName string // Function name (e.g., "Default")
	PkgName  string // Package name (e.g., "gin")
}

// ApplyTransformRules applies template-based code transformation
// baseDir: base directory for relative paths (config.BaseDir)
func ApplyTransformRules(file *dst.File, baseDir string, rules []config.TransformRule) error {
	for _, rule := range rules {
		// Check if target package is imported
		alias := getImportAlias(file, rule.Package)
		if alias == "" {
			continue
		}

		// Prepare template
		tmplStr := rule.Template
		if rule.TemplateFile != "" {
			// Resolve template_file path relative to baseDir
			templateFilePath := resolveRelativePath(baseDir, rule.TemplateFile)
			data, err := os.ReadFile(templateFilePath)
			if err != nil {
				return err
			}
			tmplStr = string(data)
		}

		if tmplStr == "" {
			continue
		}

		tmpl, err := template.New("transform").Parse(tmplStr)
		if err != nil {
			return err
		}

		// Add new imports
		for _, imp := range rule.Imports {
			common.AddImport(file, `"`+imp+`"`)
		}

		// Traverse all function declarations
		for _, decl := range file.Decls {
			fn, ok := decl.(*dst.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			// Transform statement list
			newList, err := transformStmts(fn.Body.List, alias, rule.Function, tmpl)
			if err != nil {
				return err
			}
			fn.Body.List = newList
		}
	}

	return nil
}

// transformStmts finds and transforms target calls in each statement
func transformStmts(stmts []dst.Stmt, alias, funcName string, tmpl *template.Template) ([]dst.Stmt, error) {
	var newList []dst.Stmt

	for _, stmt := range stmts {
		// Find target call in assignment statement
		if assign, ok := stmt.(*dst.AssignStmt); ok && len(assign.Rhs) > 0 {
			if call, ok := assign.Rhs[0].(*dst.CallExpr); ok {
				if isTargetCall(call, alias, funcName) {
					// Build transform context
					ctx := buildTransformContext(assign, call, alias)

					// Execute template
					var buf bytes.Buffer
					if err := tmpl.Execute(&buf, ctx); err != nil {
						return nil, err
					}

					// Parse and insert result
					resultStmts, err := parseTransformResult(buf.String(), assign)
					if err != nil {
						// Keep original on parse failure
						newList = append(newList, stmt)
						continue
					}

					newList = append(newList, resultStmts...)
					continue
				}
			}
		}

		// Find target call in expression statement
		if exprStmt, ok := stmt.(*dst.ExprStmt); ok {
			if call, ok := exprStmt.X.(*dst.CallExpr); ok {
				if isTargetCall(call, alias, funcName) {
					ctx := &TransformContext{
						Original: nodeToString(call),
						FuncName: funcName,
						PkgName:  alias,
						Args:     argsToString(call.Args),
					}
					if len(call.Args) > 0 {
						ctx.Arg0 = nodeToString(call.Args[0])
					}

					var buf bytes.Buffer
					if err := tmpl.Execute(&buf, ctx); err != nil {
						return nil, err
					}

					resultStmts, err := parseCodeBlock(buf.String())
					if err != nil {
						newList = append(newList, stmt)
						continue
					}

					newList = append(newList, resultStmts...)
					continue
				}
			}
		}

		newList = append(newList, stmt)
	}

	return newList, nil
}

// isTargetCall checks if it's a target function call
func isTargetCall(call *dst.CallExpr, alias, funcName string) bool {
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	return ident.Name == alias && sel.Sel.Name == funcName
}

// buildTransformContext builds a transform context
func buildTransformContext(assign *dst.AssignStmt, call *dst.CallExpr, alias string) *TransformContext {
	ctx := &TransformContext{
		Original: nodeToString(call),
		PkgName:  alias,
	}

	// Extract function name
	if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
		ctx.FuncName = sel.Sel.Name
	}

	// Extract variable name
	if len(assign.Lhs) > 0 {
		if ident, ok := assign.Lhs[0].(*dst.Ident); ok {
			ctx.Var = ident.Name
		}
	}

	// Extract arguments
	ctx.Args = argsToString(call.Args)
	if len(call.Args) > 0 {
		ctx.Arg0 = nodeToString(call.Args[0])
	}
	if len(call.Args) > 1 {
		ctx.Arg1 = nodeToString(call.Args[1])
	}
	if len(call.Args) > 2 {
		ctx.Arg2 = nodeToString(call.Args[2])
	}

	return ctx
}

// parseTransformResult parses the template result
func parseTransformResult(result string, originalAssign *dst.AssignStmt) ([]dst.Stmt, error) {
	// Handle multi-line result
	lines := strings.Split(strings.TrimSpace(result), "\n")

	// If first line is {{.Original}}, keep original + add following lines
	if len(lines) > 1 && strings.Contains(lines[0], "{{.Original}}") {
		// Original assignment + additional statements
		stmts := []dst.Stmt{originalAssign}
		additionalStmts, err := parseCodeBlock(strings.Join(lines[1:], "\n"))
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, additionalStmts...)
		return stmts, nil
	}

	// Replace entire with new code
	return parseCodeBlock(result)
}

// nodeToString converts a DST node to string
func nodeToString(node dst.Node) string {
	// Simple implementation - more sophisticated conversion needed in practice
	var buf bytes.Buffer

	// Wrap dst.Node in temporary file for output
	// Return name directly for Ident
	if ident, ok := node.(*dst.Ident); ok {
		return ident.Name
	}

	// Format CallExpr, SelectorExpr, etc. directly
	switch n := node.(type) {
	case *dst.CallExpr:
		fnStr := nodeToString(n.Fun)
		argsStr := argsToString(n.Args)
		return fmt.Sprintf("%s(%s)", fnStr, argsStr)
	case *dst.SelectorExpr:
		return fmt.Sprintf("%s.%s", nodeToString(n.X), n.Sel.Name)
	case *dst.BasicLit:
		return n.Value
	case *dst.UnaryExpr:
		return fmt.Sprintf("%s%s", n.Op.String(), nodeToString(n.X))
	case *dst.BinaryExpr:
		return fmt.Sprintf("%s %s %s", nodeToString(n.X), n.Op.String(), nodeToString(n.Y))
	case *dst.StarExpr:
		return fmt.Sprintf("*%s", nodeToString(n.X))
	case *dst.CompositeLit:
		if n.Type != nil {
			return fmt.Sprintf("%s{...}", nodeToString(n.Type))
		}
		return "{...}"
	case *dst.IndexExpr:
		return fmt.Sprintf("%s[%s]", nodeToString(n.X), nodeToString(n.Index))
	case *dst.FuncLit:
		return "func(){...}"
	default:
		// Return empty buffer for other nodes
		_ = buf
		return "<unknown>"
	}
}

// argsToString converts argument list to string
func argsToString(args []dst.Expr) string {
	var parts []string
	for _, arg := range args {
		parts = append(parts, nodeToString(arg))
	}
	return strings.Join(parts, ", ")
}

// RemoveTransformRules removes code transformed by transform rules
func RemoveTransformRules(file *dst.File, rules []config.TransformRule) error {
	// TODO: Implement transform removal logic
	// Markers or reverse templates needed to restore transformed code to original
	return nil
}
