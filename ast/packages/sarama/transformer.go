// Package sarama provides the IBM/sarama (Kafka) transformer.
package sarama

import (
	"go/token"
	"strings"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for sarama.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "sarama"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "github.com/IBM/sarama"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/github.com/IBM/sarama/whatapsarama"
}

// Detect checks if the file uses sarama.
func (t *Transformer) Detect(file *dst.File) bool {
	// Check for both IBM/sarama and Shopify/sarama
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == "github.com/IBM/sarama" || path == "github.com/Shopify/sarama" {
			return true
		}
	}
	return false
}

// Inject adds whatapsarama interceptor after sarama.NewConfig().
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false

	// Get the actual package name used in code (could be alias)
	// Check both IBM/sarama and Shopify/sarama (deprecated)
	pkgName := common.GetPackageNameForImportPrefix(file, "github.com/IBM/sarama")
	if pkgName == "" {
		pkgName = common.GetPackageNameForImportPrefix(file, "github.com/Shopify/sarama")
	}
	if pkgName == "" {
		return false, nil
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		// First pass: find all NewConfig() calls in this function
		var configVars []string
		var configIndices []int
		for i, stmt := range fn.Body.List {
			if configVar := t.getSaramaNewConfigVar(stmt, pkgName); configVar != "" {
				configVars = append(configVars, configVar)
				configIndices = append(configIndices, i)
			}
		}

		// Skip if no NewConfig() calls
		if len(configVars) == 0 {
			continue
		}

		// Second pass: build new statement list
		// Add interceptor declaration once at the beginning (after first NewConfig)
		var newList []dst.Stmt
		interceptorDeclared := false

		for i, stmt := range fn.Body.List {
			newList = append(newList, stmt)

			// Check if this is a NewConfig() call
			configVar := t.getSaramaNewConfigVar(stmt, pkgName)
			if configVar == "" {
				continue
			}

			// Add interceptor declaration only once (after first NewConfig)
			if !interceptorDeclared {
				interceptorVarStmt := &dst.AssignStmt{
					Lhs: []dst.Expr{dst.NewIdent("whatapInterceptor")},
					Tok: token.DEFINE,
					Rhs: []dst.Expr{
						&dst.UnaryExpr{
							Op: token.AND,
							X: &dst.CompositeLit{
								Type: &dst.SelectorExpr{
									X:   dst.NewIdent("whatapsarama"),
									Sel: dst.NewIdent("Interceptor"),
								},
							},
						},
					},
				}
				interceptorVarStmt.Decs.After = dst.NewLine
				newList = append(newList, interceptorVarStmt)
				interceptorDeclared = true
			}

			// Add: config.Producer.Interceptors = []sarama.ProducerInterceptor{whatapInterceptor}
			producerStmt := &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.SelectorExpr{
						X: &dst.SelectorExpr{
							X:   dst.NewIdent(configVar),
							Sel: dst.NewIdent("Producer"),
						},
						Sel: dst.NewIdent("Interceptors"),
					},
				},
				Tok: token.ASSIGN,
				Rhs: []dst.Expr{
					&dst.CompositeLit{
						Type: &dst.ArrayType{
							Elt: &dst.SelectorExpr{
								X:   dst.NewIdent("sarama"),
								Sel: dst.NewIdent("ProducerInterceptor"),
							},
						},
						Elts: []dst.Expr{dst.NewIdent("whatapInterceptor")},
					},
				},
			}
			producerStmt.Decs.After = dst.NewLine
			newList = append(newList, producerStmt)

			// Add: config.Consumer.Interceptors = []sarama.ConsumerInterceptor{whatapInterceptor}
			consumerStmt := &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.SelectorExpr{
						X: &dst.SelectorExpr{
							X:   dst.NewIdent(configVar),
							Sel: dst.NewIdent("Consumer"),
						},
						Sel: dst.NewIdent("Interceptors"),
					},
				},
				Tok: token.ASSIGN,
				Rhs: []dst.Expr{
					&dst.CompositeLit{
						Type: &dst.ArrayType{
							Elt: &dst.SelectorExpr{
								X:   dst.NewIdent("sarama"),
								Sel: dst.NewIdent("ConsumerInterceptor"),
							},
						},
						Elts: []dst.Expr{dst.NewIdent("whatapInterceptor")},
					},
				},
			}
			consumerStmt.Decs.After = dst.NewLine
			newList = append(newList, consumerStmt)
			t.transformed = true
			_ = i // suppress unused variable warning
		}
		fn.Body.List = newList
	}
	return t.transformed, nil
}

// Remove removes whatapsarama interceptor statements.
func (t *Transformer) Remove(file *dst.File) error {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var newList []dst.Stmt
		for _, stmt := range fn.Body.List {
			// Skip: whatapInterceptor := &whatapsarama.Interceptor{}
			if t.isWhatapsaramaInterceptorDecl(stmt) {
				continue
			}

			// Skip: config.Producer.Interceptors = ... or config.Consumer.Interceptors = ...
			if t.isSaramaInterceptorAssign(stmt) {
				continue
			}

			newList = append(newList, stmt)
		}
		fn.Body.List = newList
	}
	return nil
}

// getSaramaNewConfigVar returns the variable name if stmt is sarama.NewConfig()
// pkgName is the actual package name used in code (could be alias)
func (t *Transformer) getSaramaNewConfigVar(stmt dst.Stmt, pkgName string) string {
	assign, ok := stmt.(*dst.AssignStmt)
	if !ok || len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
		return ""
	}

	call, ok := assign.Rhs[0].(*dst.CallExpr)
	if !ok {
		return ""
	}

	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return ""
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok || ident.Name != pkgName || sel.Sel.Name != "NewConfig" {
		return ""
	}

	if lhsIdent, ok := assign.Lhs[0].(*dst.Ident); ok {
		return lhsIdent.Name
	}
	return ""
}

// isWhatapsaramaInterceptorDecl checks if stmt is whatapInterceptor := &whatapsarama.Interceptor{}
func (t *Transformer) isWhatapsaramaInterceptorDecl(stmt dst.Stmt) bool {
	assign, ok := stmt.(*dst.AssignStmt)
	if !ok || assign.Tok != token.DEFINE {
		return false
	}

	if len(assign.Rhs) == 0 {
		return false
	}

	unary, ok := assign.Rhs[0].(*dst.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return false
	}

	composite, ok := unary.X.(*dst.CompositeLit)
	if !ok {
		return false
	}

	sel, ok := composite.Type.(*dst.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*dst.Ident)
	if !ok {
		return false
	}

	return ident.Name == "whatapsarama" && sel.Sel.Name == "Interceptor"
}

// isSaramaInterceptorAssign checks if stmt is config.*.Interceptors = [...]{whatapInterceptor}
func (t *Transformer) isSaramaInterceptorAssign(stmt dst.Stmt) bool {
	assign, ok := stmt.(*dst.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN {
		return false
	}

	if len(assign.Lhs) == 0 || len(assign.Rhs) == 0 {
		return false
	}

	// Check LHS: *.Interceptors
	sel, ok := assign.Lhs[0].(*dst.SelectorExpr)
	if !ok || sel.Sel.Name != "Interceptors" {
		return false
	}

	// Check RHS contains whatapInterceptor
	composite, ok := assign.Rhs[0].(*dst.CompositeLit)
	if !ok {
		return false
	}

	for _, elt := range composite.Elts {
		if ident, ok := elt.(*dst.Ident); ok {
			if ident.Name == "whatapInterceptor" {
				return true
			}
		}
	}

	return false
}
