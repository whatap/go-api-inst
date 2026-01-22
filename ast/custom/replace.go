package custom

import (
	"go-api-inst/ast/common"
	"go-api-inst/config"

	"github.com/dave/dst"
)

// ApplyReplaceRules applies function call replacement rules
// e.g., sql.Open → whatapsql.Open
func ApplyReplaceRules(file *dst.File, rules []config.ReplaceRule) error {
	for _, rule := range rules {
		// Check if target package is imported
		alias := getImportAlias(file, rule.Package)
		if alias == "" {
			continue
		}

		// Parse replacement package.function
		newPkg, newFunc := parseQualifiedName(rule.With)
		if newPkg == "" {
			// Replace only function name if no "." (same package)
			newFunc = rule.With
			newPkg = alias
		}

		// Add new imports
		for _, imp := range rule.Imports {
			common.AddImport(file, `"`+imp+`"`)
		}

		// Find and replace function calls in the entire file
		dst.Inspect(file, func(n dst.Node) bool {
			call, ok := n.(*dst.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*dst.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := sel.X.(*dst.Ident)
			if !ok {
				return true
			}

			// Match pkg.Func pattern
			if ident.Name == alias && sel.Sel.Name == rule.Function {
				// Replace
				ident.Name = newPkg
				if newFunc != "" && newFunc != rule.Function {
					sel.Sel.Name = newFunc
				}
			}

			return true
		})
	}

	return nil
}

// RemoveReplaceRules removes function call replacements (restores original)
// e.g., whatapsql.Open → sql.Open
func RemoveReplaceRules(file *dst.File, rules []config.ReplaceRule) error {
	for _, rule := range rules {
		// Parse replaced package.function
		replacedPkg, replacedFunc := parseQualifiedName(rule.With)
		if replacedPkg == "" {
			continue
		}

		// Check if replaced package is imported
		if !hasImportPath(file, rule.Imports[0]) {
			continue
		}

		// Determine original package alias
		originalAlias := getImportAlias(file, rule.Package)
		if originalAlias == "" {
			// Use last path component if original package is not imported
			parts := splitPath(rule.Package)
			originalAlias = parts[len(parts)-1]
		}

		// Find and restore replaced calls to original in the entire file
		dst.Inspect(file, func(n dst.Node) bool {
			call, ok := n.(*dst.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*dst.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := sel.X.(*dst.Ident)
			if !ok {
				return true
			}

			// Match replaced pattern
			if ident.Name == replacedPkg {
				funcToMatch := replacedFunc
				if funcToMatch == "" {
					funcToMatch = rule.Function
				}
				if sel.Sel.Name == funcToMatch {
					// Restore to original
					ident.Name = originalAlias
					sel.Sel.Name = rule.Function
				}
			}

			return true
		})

		// Remove unused whatap imports
		for _, imp := range rule.Imports {
			common.RemoveUnusedImport(file, imp)
		}
	}

	return nil
}

// splitPath splits a path by "/"
func splitPath(path string) []string {
	return splitString(path, "/")
}

// splitString splits a string by delimiter
func splitString(s, sep string) []string {
	var result []string
	for _, part := range splitByDelimiter(s, sep) {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// splitByDelimiter internal split function
func splitByDelimiter(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}
