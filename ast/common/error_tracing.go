// Package common provides shared utilities for AST transformations.
package common

import (
	"fmt"
	"go/token"
	"os"

	"github.com/dave/dst"
)

// ErrorTracer injects error tracing code
type ErrorTracer struct {
	injected bool // whether error tracing code has been injected
}

// NewErrorTracer creates a new error tracer
func NewErrorTracer() *ErrorTracer {
	return &ErrorTracer{}
}

// Inject injects error tracing code into the file
// Finds if err != nil { return ... } patterns and adds trace.Error(context.Background(), err) before return
func (et *ErrorTracer) Inject(file *dst.File) {
	et.injected = false
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		// Skip main function (error should not occur before trace.Init)
		if fn.Name.Name == "main" && fn.Recv == nil {
			continue
		}

		// Traverse all if statements in the function body
		fn.Body.List = et.processStmts(fn.Body.List, nil)
	}

	// Add context import if error tracing was injected
	if et.injected {
		AddImport(file, "context")
	}
}

// Remove removes error tracing code from the file
func (et *ErrorTracer) Remove(file *dst.File) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		et.removeFromBlock(fn.Body)
	}

	// Remove unused context import
	RemoveUnusedImport(file, "context")
}

// processStmts processes error check patterns in a statement list
func (et *ErrorTracer) processStmts(stmts []dst.Stmt, isWhatapCall func(dst.Stmt) bool) []dst.Stmt {
	var newStmts []dst.Stmt
	var prevStmt dst.Stmt

	for i, stmt := range stmts {
		newStmts = append(newStmts, stmt)

		// Skip if err != nil right after whatap package call
		skipErrorTracing := false
		if prevStmt != nil && isWhatapCall != nil && isWhatapCall(prevStmt) {
			if _, ok := stmt.(*dst.IfStmt); ok {
				skipErrorTracing = true
				if os.Getenv("GO_API_AST_DEBUG") != "" {
					fmt.Println("[DEBUG] Skipping trace.Error injection after whatap package call")
				}
			}
		}

		if !skipErrorTracing {
			et.processStmt(stmt, isWhatapCall)
		}

		// Process if err == nil { return } pattern
		if ifStmt, ok := stmt.(*dst.IfStmt); ok {
			// Skip if right after whatap call
			if prevStmt != nil && isWhatapCall != nil && isWhatapCall(prevStmt) {
				prevStmt = stmt
				continue
			}

			errVarName := et.getErrEqualNilVarName(ifStmt.Cond)
			if errVarName != "" && et.blockHasReturn(ifStmt.Body) {
				// If if block has return, insert trace.Error after the if statement
				if i < len(stmts)-1 {
					traceErrorStmt := et.createTraceErrorStmt(errVarName)
					newStmts = append(newStmts, traceErrorStmt)
					et.injected = true
				}
			}
		}

		prevStmt = stmt
	}

	return newStmts
}

// processStmt processes error check patterns in a single statement
func (et *ErrorTracer) processStmt(stmt dst.Stmt, isWhatapCall func(dst.Stmt) bool) {
	switch s := stmt.(type) {
	case *dst.IfStmt:
		// Check if it's an if err != nil pattern
		errVarName := et.getErrCheckVarName(s.Cond)
		if errVarName != "" {
			et.insertTraceErrorBeforeReturn(s.Body, errVarName)
		}

		// Process if err == nil { } else { return } pattern
		errEqualNilVarName := et.getErrEqualNilVarName(s.Cond)
		if errEqualNilVarName != "" && s.Else != nil {
			if elseBlock, ok := s.Else.(*dst.BlockStmt); ok {
				et.insertTraceErrorBeforeReturn(elseBlock, errEqualNilVarName)
			}
		}

		// Recursively process else block
		if s.Else != nil {
			if elseBlock, ok := s.Else.(*dst.BlockStmt); ok {
				elseBlock.List = et.processStmts(elseBlock.List, isWhatapCall)
			} else if elseIf, ok := s.Else.(*dst.IfStmt); ok {
				et.processStmt(elseIf, isWhatapCall)
			}
		}

		// Process nested if statements inside the if block
		s.Body.List = et.processStmts(s.Body.List, isWhatapCall)

	case *dst.BlockStmt:
		s.List = et.processStmts(s.List, isWhatapCall)

	case *dst.ForStmt:
		if s.Body != nil {
			s.Body.List = et.processStmts(s.Body.List, isWhatapCall)
		}

	case *dst.RangeStmt:
		if s.Body != nil {
			s.Body.List = et.processStmts(s.Body.List, isWhatapCall)
		}

	case *dst.SwitchStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CaseClause); ok {
					cc.Body = et.processStmts(cc.Body, isWhatapCall)
				}
			}
		}

	case *dst.SelectStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CommClause); ok {
					cc.Body = et.processStmts(cc.Body, isWhatapCall)
				}
			}
		}
	}
}

// removeFromBlock removes trace.Error calls from a block
func (et *ErrorTracer) removeFromBlock(block *dst.BlockStmt) {
	var newList []dst.Stmt
	for _, stmt := range block.List {
		if et.isTraceErrorCall(stmt) {
			continue
		}
		newList = append(newList, stmt)

		// Process nested blocks
		et.removeFromStmt(stmt)
	}
	block.List = newList
}

// removeFromStmt removes trace.Error from nested blocks within a statement
func (et *ErrorTracer) removeFromStmt(stmt dst.Stmt) {
	switch s := stmt.(type) {
	case *dst.IfStmt:
		et.removeFromBlock(s.Body)
		if s.Else != nil {
			if elseBlock, ok := s.Else.(*dst.BlockStmt); ok {
				et.removeFromBlock(elseBlock)
			} else if elseIf, ok := s.Else.(*dst.IfStmt); ok {
				et.removeFromStmt(elseIf)
			}
		}
	case *dst.ForStmt:
		if s.Body != nil {
			et.removeFromBlock(s.Body)
		}
	case *dst.RangeStmt:
		if s.Body != nil {
			et.removeFromBlock(s.Body)
		}
	case *dst.SwitchStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CaseClause); ok {
					var newBody []dst.Stmt
					for _, bodyStmt := range cc.Body {
						if !et.isTraceErrorCall(bodyStmt) {
							newBody = append(newBody, bodyStmt)
							et.removeFromStmt(bodyStmt)
						}
					}
					cc.Body = newBody
				}
			}
		}
	case *dst.SelectStmt:
		if s.Body != nil {
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*dst.CommClause); ok {
					var newBody []dst.Stmt
					for _, bodyStmt := range cc.Body {
						if !et.isTraceErrorCall(bodyStmt) {
							newBody = append(newBody, bodyStmt)
							et.removeFromStmt(bodyStmt)
						}
					}
					cc.Body = newBody
				}
			}
		}
	}
}

// getErrCheckVarName extracts error variable name from err != nil condition
func (et *ErrorTracer) getErrCheckVarName(cond dst.Expr) string {
	binExpr, ok := cond.(*dst.BinaryExpr)
	if !ok || binExpr.Op != token.NEQ {
		return ""
	}

	if ident, ok := binExpr.Y.(*dst.Ident); !ok || ident.Name != "nil" {
		return ""
	}

	if ident, ok := binExpr.X.(*dst.Ident); ok {
		if ident.Name == "err" || ident.Name == "e" || ident.Name == "error" {
			return ident.Name
		}
	}

	return ""
}

// getErrEqualNilVarName extracts error variable name from err == nil condition
func (et *ErrorTracer) getErrEqualNilVarName(cond dst.Expr) string {
	binExpr, ok := cond.(*dst.BinaryExpr)
	if !ok || binExpr.Op != token.EQL {
		return ""
	}

	if ident, ok := binExpr.Y.(*dst.Ident); !ok || ident.Name != "nil" {
		return ""
	}

	if ident, ok := binExpr.X.(*dst.Ident); ok {
		if ident.Name == "err" || ident.Name == "e" || ident.Name == "error" {
			return ident.Name
		}
	}

	return ""
}

// blockHasReturn checks if the block contains a return statement
func (et *ErrorTracer) blockHasReturn(block *dst.BlockStmt) bool {
	for _, stmt := range block.List {
		if _, ok := stmt.(*dst.ReturnStmt); ok {
			return true
		}
	}
	return false
}

// createTraceErrorStmt creates a trace.Error(context.Background(), err) statement
func (et *ErrorTracer) createTraceErrorStmt(errVarName string) *dst.ExprStmt {
	stmt := &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent("trace"),
				Sel: dst.NewIdent("Error"),
			},
			Args: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent("context"),
						Sel: dst.NewIdent("Background"),
					},
				},
				dst.NewIdent(errVarName),
			},
		},
	}
	stmt.Decs.After = dst.NewLine
	return stmt
}

// insertTraceErrorBeforeReturn inserts trace.Error before return statements
func (et *ErrorTracer) insertTraceErrorBeforeReturn(block *dst.BlockStmt, errVarName string) {
	var newList []dst.Stmt

	for _, stmt := range block.List {
		if _, ok := stmt.(*dst.ReturnStmt); ok {
			// Check if trace.Error already exists
			if len(newList) > 0 && et.isTraceErrorCallWithVar(newList[len(newList)-1], errVarName) {
				newList = append(newList, stmt)
				continue
			}

			traceErrorStmt := et.createTraceErrorStmt(errVarName)
			newList = append(newList, traceErrorStmt, stmt)
			et.injected = true
		} else {
			newList = append(newList, stmt)
		}
	}

	block.List = newList
}

// isTraceErrorCall checks if it's a trace.Error(...) call
func (et *ErrorTracer) isTraceErrorCall(stmt dst.Stmt) bool {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
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

	return ident.Name == "trace" && sel.Sel.Name == "Error"
}

// isTraceErrorCallWithVar checks if it's a trace.Error call using a specific error variable
func (et *ErrorTracer) isTraceErrorCallWithVar(stmt dst.Stmt, errVarName string) bool {
	if !et.isTraceErrorCall(stmt) {
		return false
	}

	exprStmt := stmt.(*dst.ExprStmt)
	call := exprStmt.X.(*dst.CallExpr)

	// Check if it's trace.Error(ctx, err) format
	if len(call.Args) < 2 {
		return false
	}

	if ident, ok := call.Args[1].(*dst.Ident); ok {
		return ident.Name == errVarName
	}

	return false
}
