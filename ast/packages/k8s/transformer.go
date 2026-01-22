// Package k8s provides the Kubernetes client-go transformer.
package k8s

import (
	"strings"

	"github.com/whatap/go-api-inst/ast/common"

	"github.com/dave/dst"
)

func init() {
	common.Register(&Transformer{})
}

// Transformer implements common.Transformer for Kubernetes client-go.
type Transformer struct {
	transformed bool // tracks if any transformation was made
}

// Name returns the transformer name.
func (t *Transformer) Name() string {
	return "k8s"
}

// ImportPath returns the original package import path.
func (t *Transformer) ImportPath() string {
	return "k8s.io/client-go/kubernetes"
}

// WhatapImport returns the whatap instrumentation import path.
func (t *Transformer) WhatapImport() string {
	return "github.com/whatap/go-api/instrumentation/k8s.io/client-go/kubernetes/whatapkubernetes"
}

// Detect checks if the file uses Kubernetes client-go.
func (t *Transformer) Detect(file *dst.File) bool {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if path == "k8s.io/client-go/kubernetes" || path == "k8s.io/client-go/rest" {
			return true
		}
	}
	return false
}

// Inject adds config.Wrap(whatapkubernetes.WrapRoundTripper()) before kubernetes.NewForConfig().
// Returns (true, nil) if transformation occurred, (false, nil) otherwise.
func (t *Transformer) Inject(file *dst.File) (bool, error) {
	t.transformed = false

	// Get the actual package name used in code (could be alias)
	pkgName := common.GetPackageNameForImportPrefix(file, t.ImportPath())
	if pkgName == "" {
		return false, nil
	}

	// Track wrapped configs to avoid duplicate wrapping
	wrappedConfigs := make(map[string]bool)

	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var newList []dst.Stmt
		for _, stmt := range fn.Body.List {
			// Check for kubernetes.NewForConfig(config) call
			configVar := t.getK8sNewForConfigArg(stmt, pkgName)
			if configVar != "" && !wrappedConfigs[configVar] {
				wrappedConfigs[configVar] = true

				// Add: config.Wrap(whatapkubernetes.WrapRoundTripper())
				wrapStmt := &dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.SelectorExpr{
							X:   dst.NewIdent(configVar),
							Sel: dst.NewIdent("Wrap"),
						},
						Args: []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.SelectorExpr{
									X:   dst.NewIdent("whatapkubernetes"),
									Sel: dst.NewIdent("WrapRoundTripper"),
								},
							},
						},
					},
				}
				wrapStmt.Decs.After = dst.NewLine
				newList = append(newList, wrapStmt)
				t.transformed = true
			}

			newList = append(newList, stmt)
		}
		fn.Body.List = newList
	}
	return t.transformed, nil
}

// Remove removes config.Wrap(whatapkubernetes.WrapRoundTripper()) statements.
func (t *Transformer) Remove(file *dst.File) error {
	for _, decl := range file.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var newList []dst.Stmt
		for _, stmt := range fn.Body.List {
			if t.isK8sWrapCall(stmt) {
				continue // Remove whatapkubernetes Wrap call
			}
			newList = append(newList, stmt)
		}
		fn.Body.List = newList
	}
	return nil
}

// getK8sNewForConfigArg returns the config variable name from kubernetes.NewForConfig(config) call.
// pkgName is the actual package name used in code (could be alias)
func (t *Transformer) getK8sNewForConfigArg(stmt dst.Stmt, pkgName string) string {
	assign, ok := stmt.(*dst.AssignStmt)
	if !ok || len(assign.Rhs) == 0 {
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
	if !ok {
		return ""
	}

	// kubernetes.NewForConfig or kubernetes.NewForConfigOrDie
	if ident.Name != pkgName {
		return ""
	}
	if sel.Sel.Name != "NewForConfig" && sel.Sel.Name != "NewForConfigOrDie" {
		return ""
	}

	// Get first argument (config)
	if len(call.Args) == 0 {
		return ""
	}

	if argIdent, ok := call.Args[0].(*dst.Ident); ok {
		return argIdent.Name
	}
	return ""
}

// isK8sWrapCall checks if stmt is config.Wrap(whatapkubernetes.WrapRoundTripper()).
func (t *Transformer) isK8sWrapCall(stmt dst.Stmt) bool {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return false
	}

	// Check for .Wrap(...) call
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok || sel.Sel.Name != "Wrap" {
		return false
	}

	// Check argument is whatapkubernetes.WrapRoundTripper()
	if len(call.Args) != 1 {
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

	return argIdent.Name == "whatapkubernetes" && argSel.Sel.Name == "WrapRoundTripper"
}
