package custom

import (
	"github.com/whatap/go-api-inst/ast/common"
	"github.com/whatap/go-api-inst/config"

	"github.com/dave/dst"
)

// ApplyInjectRules inserts code inside function definitions
// Inserts code at the start/end of user-defined functions
func ApplyInjectRules(file *dst.File, rules []config.InjectRule, srcPath string) error {
	for _, rule := range rules {
		// Match package/file pattern
		if !matchesPackageOrFile(file, rule.Package, rule.File, srcPath) {
			continue
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

			// Match function name pattern
			if !matchesFunction(fn.Name.Name, rule.Function) {
				continue
			}

			// Insert Start code (at function start)
			if rule.Start != "" {
				stmts, err := parseCodeBlock(rule.Start)
				if err != nil {
					return err
				}
				// Insert before existing statements
				fn.Body.List = append(stmts, fn.Body.List...)
			}

			// Insert End code (wrapped with defer to execute at function exit)
			// Uses defer func() { ... }() pattern
			if rule.End != "" {
				stmts, err := parseCodeBlock(rule.End)
				if err != nil {
					return err
				}
				// Create defer func() { end code }()
				deferStmt := &dst.DeferStmt{
					Call: &dst.CallExpr{
						Fun: &dst.FuncLit{
							Type: &dst.FuncType{
								Params: &dst.FieldList{},
							},
							Body: &dst.BlockStmt{
								List: stmts,
							},
						},
					},
				}
				// Insert defer right after Start code (at function start)
				if rule.Start != "" {
					// Calculate position after start code
					startStmts, _ := parseCodeBlock(rule.Start)
					insertPos := len(startStmts)
					newList := make([]dst.Stmt, 0, len(fn.Body.List)+1)
					newList = append(newList, fn.Body.List[:insertPos]...)
					newList = append(newList, deferStmt)
					newList = append(newList, fn.Body.List[insertPos:]...)
					fn.Body.List = newList
				} else {
					// Insert at function start if no start code
					fn.Body.List = append([]dst.Stmt{deferStmt}, fn.Body.List...)
				}
			}
		}
	}

	return nil
}

// RemoveInjectRules removes injected code
// Consider using marker comments for accurate removal
func RemoveInjectRules(file *dst.File, rules []config.InjectRule, srcPath string) error {
	// TODO: Implement inject removal logic
	// Currently difficult to identify injected code
	// Need to consider marker comment pattern
	return nil
}
