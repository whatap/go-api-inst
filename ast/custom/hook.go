package custom

import (
	"go-api-inst/ast/common"
	"go-api-inst/config"

	"github.com/dave/dst"
)

// ApplyHookRules inserts code before/after function calls
func ApplyHookRules(file *dst.File, rules []config.HookRule) error {
	for _, rule := range rules {
		// Check target package alias
		// Treat as local function call if "main" or current file's package
		alias := getImportAlias(file, rule.Package)
		isLocalCall := rule.Package == "main" || rule.Package == file.Name.Name

		// Skip if external package and import doesn't exist
		if !isLocalCall && alias == "" {
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

			// Rebuild statement list
			newList := processStmtsForHookEx(fn.Body.List, rule, alias, isLocalCall)
			fn.Body.List = newList
		}
	}

	return nil
}

// processStmtsForHookEx checks each statement and inserts hooks (supports local/external functions)
func processStmtsForHookEx(stmts []dst.Stmt, rule config.HookRule, alias string, isLocalCall bool) []dst.Stmt {
	var newList []dst.Stmt

	for _, stmt := range stmts {
		// Check if this statement contains target function call
		var hasTarget bool
		if isLocalCall {
			hasTarget = containsLocalCall(stmt, rule.Function)
		} else {
			hasTarget = containsCall(stmt, alias, rule.Function)
		}

		if hasTarget {
			// Insert Before
			if rule.Before != "" {
				beforeStmts, _ := parseCodeBlock(rule.Before)
				newList = append(newList, beforeStmts...)
			}

			newList = append(newList, stmt)

			// Insert After
			if rule.After != "" {
				afterStmts, _ := parseCodeBlock(rule.After)
				newList = append(newList, afterStmts...)
			}
		} else {
			newList = append(newList, stmt)
		}

		// Process nested blocks (if, for, switch, etc.)
		processNestedBlocksEx(stmt, rule, alias, isLocalCall)
	}

	return newList
}

// processStmtsForHook checks each statement and inserts hooks (backward compatibility)
func processStmtsForHook(stmts []dst.Stmt, rule config.HookRule, alias string) []dst.Stmt {
	return processStmtsForHookEx(stmts, rule, alias, false)
}

// containsLocalCall checks if statement contains a direct local function call
// Supported patterns:
//   - processData()                        : ExprStmt
//   - err := processData()                 : AssignStmt
//   - if err := processData(); err != nil  : IfStmt.Init
//   - switch processData() { ... }         : SwitchStmt.Tag
func containsLocalCall(stmt dst.Stmt, funcName string) bool {
	switch s := stmt.(type) {
	case *dst.ExprStmt:
		// Simple call: processData()
		return isLocalCallExpr(s.X, funcName)

	case *dst.AssignStmt:
		// Assignment: err := processData()
		if len(s.Rhs) > 0 {
			return isLocalCallExpr(s.Rhs[0], funcName)
		}

	case *dst.IfStmt:
		// if initialization: if err := processData(); err != nil { }
		if s.Init != nil {
			if assign, ok := s.Init.(*dst.AssignStmt); ok {
				if len(assign.Rhs) > 0 {
					return isLocalCallExpr(assign.Rhs[0], funcName)
				}
			}
		}

	case *dst.SwitchStmt:
		// switch condition: switch processData() { ... }
		if s.Tag != nil {
			return isLocalCallExpr(s.Tag, funcName)
		}
	}

	return false
}

// isLocalCallExpr checks if expression is a local function call (supports wildcard pattern)
func isLocalCallExpr(expr dst.Expr, funcPattern string) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}

	if ident, ok := call.Fun.(*dst.Ident); ok {
		return matchesFunction(ident.Name, funcPattern)
	}

	return false
}

// processNestedBlocksEx processes nested blocks (supports local/external functions)
func processNestedBlocksEx(stmt dst.Stmt, rule config.HookRule, alias string, isLocalCall bool) {
	switch s := stmt.(type) {
	case *dst.IfStmt:
		if s.Body != nil {
			s.Body.List = processStmtsForHookEx(s.Body.List, rule, alias, isLocalCall)
		}
		if s.Else != nil {
			if block, ok := s.Else.(*dst.BlockStmt); ok {
				block.List = processStmtsForHookEx(block.List, rule, alias, isLocalCall)
			} else if elseIf, ok := s.Else.(*dst.IfStmt); ok {
				processNestedBlocksEx(elseIf, rule, alias, isLocalCall)
			}
		}
	case *dst.ForStmt:
		if s.Body != nil {
			s.Body.List = processStmtsForHookEx(s.Body.List, rule, alias, isLocalCall)
		}
	case *dst.RangeStmt:
		if s.Body != nil {
			s.Body.List = processStmtsForHookEx(s.Body.List, rule, alias, isLocalCall)
		}
	case *dst.SwitchStmt:
		if s.Body != nil {
			for _, c := range s.Body.List {
				if caseClause, ok := c.(*dst.CaseClause); ok {
					caseClause.Body = processStmtsForHookEx(caseClause.Body, rule, alias, isLocalCall)
				}
			}
		}
	case *dst.SelectStmt:
		if s.Body != nil {
			for _, c := range s.Body.List {
				if commClause, ok := c.(*dst.CommClause); ok {
					commClause.Body = processStmtsForHookEx(commClause.Body, rule, alias, isLocalCall)
				}
			}
		}
	}
}

// processNestedBlocks processes nested blocks (backward compatibility)
func processNestedBlocks(stmt dst.Stmt, rule config.HookRule, alias string) {
	processNestedBlocksEx(stmt, rule, alias, false)
}

// RemoveHookRules removes code inserted by hooks
func RemoveHookRules(file *dst.File, rules []config.HookRule) error {
	// TODO: Implement hook removal logic
	// Markers needed to identify inserted code
	return nil
}
