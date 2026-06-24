// Package ast provides the v2 engine with go/types-based matching.
// match_filters.go: Optional filter chain (Steps 3-4 through 3-10 of matching-layer.md).
// Each filter returns true if the rule passes, false if it should be skipped.
// All filters are optional — nil rule fields mean PASS (skip the check).
package ast

import (
	"go/types"
	"strings"

	"github.com/dave/dst"
	"github.com/whatap/go-api-inst/ast/common"
)

// validateRule runs the optional filter chain (Steps 3-4 through 3-10).
// Returns true if all filters pass, false if any filter rejects.
func validateRule(ctx *MatchContext, rule *Rule) bool {
	// Step 3-4/3-5/3-6/3-8: Signature (param count + param types + return types)
	if rule.Signature != nil {
		if !matchSignature(ctx, rule.Signature) {
			return false
		}
	}
	// Step 3-7: Receiver type
	if rule.Receiver != nil {
		if !matchReceiver(ctx, rule.Receiver) {
			return false
		}
	}
	// Step 3-9: Struct fields
	if len(rule.Fields) > 0 {
		if !matchFields(ctx, rule.Fields) {
			return false
		}
	}
	// Step 3-10: Custom condition
	if rule.Condition != nil {
		if !rule.Condition(ctx) {
			return false
		}
	}
	return true
}

// matchSignature validates function call argument count and types.
// Step 3-4: ParamCount (MinArgs/MaxArgs)
// Step 3-5: ParamTypes (Args)
// Step 3-6: ReturnTypes (Results)
// Step 3-8: MethodSig (same Signature applied to method calls)
func matchSignature(ctx *MatchContext, sig *FuncSignature) bool {
	// Step 3-4: Argument count check (works without go/types)
	if ctx.Call != nil {
		argCount := len(ctx.Call.Args)

		minArgs := sig.MinArgs
		maxArgs := sig.MaxArgs

		// MinArgs == -1: derive from len(Args) if Args is set
		if minArgs == -1 && sig.Args != nil {
			minArgs = len(sig.Args)
		}

		// Check minimum (only if explicitly set or derived)
		if minArgs >= 0 && argCount < minArgs {
			return false
		}

		// Check maximum (MaxArgs == -1 means unlimited/variadic)
		if maxArgs >= 0 && argCount > maxArgs {
			return false
		}
	}

	// Step 3-5: Parameter type check (requires go/types)
	if sig.Args != nil && common.HasTypeInfo() {
		if !matchParamTypes(ctx, sig.Args) {
			return false
		}
	}
	// go/types 없으면 → PASS (안전 방향, matching-layer.md Step 3-5)

	// Step 3-6: Return type check (requires go/types)
	if sig.Results != nil && common.HasTypeInfo() {
		if !matchReturnTypes(ctx, sig.Results) {
			return false
		}
	}
	// go/types 없으면 → PASS (안전 방향, matching-layer.md Step 3-6)

	return true
}

// resolveFuncSignature extracts a *types.Signature from a call's SelectorExpr.
// Returns nil if type info is unavailable or the expression doesn't resolve to a function.
func resolveFuncSignature(call *dst.CallExpr) *types.Signature {
	if call == nil {
		return nil
	}
	sel, ok := call.Fun.(*dst.SelectorExpr)
	if !ok {
		return nil
	}
	// ResolveType on sel (SelectorExpr) resolves the selector's Sel ident → *types.Func
	t := common.ResolveType(sel)
	if t == nil {
		return nil
	}
	sig, _ := t.(*types.Signature)
	return sig
}

// matchParamTypes checks parameter types using go/types.
func matchParamTypes(ctx *MatchContext, expected []TypeName) bool {
	sig := resolveFuncSignature(ctx.Call)
	if sig == nil {
		return true // can't resolve → PASS (safe direction)
	}

	params := sig.Params()
	if params.Len() != len(expected) {
		return false
	}

	for i := 0; i < len(expected); i++ {
		if !typeNameMatches(expected[i], params.At(i).Type()) {
			return false
		}
	}
	return true
}

// matchReturnTypes checks return types using go/types.
func matchReturnTypes(ctx *MatchContext, expected []TypeName) bool {
	sig := resolveFuncSignature(ctx.Call)
	if sig == nil {
		return true // can't resolve → PASS
	}

	results := sig.Results()
	if results.Len() != len(expected) {
		return false
	}

	for i := 0; i < len(expected); i++ {
		if !typeNameMatches(expected[i], results.At(i).Type()) {
			return false
		}
	}
	return true
}

// typeNameMatches checks if a go/types.Type matches a TypeName.
func typeNameMatches(expected TypeName, actual types.Type) bool {
	actualStr := actual.String()

	// Handle pointer types
	if strings.HasPrefix(expected.Name, "*") {
		expectedName := expected.Name[1:]
		if expected.ImportPath != "" {
			return actualStr == "*"+expected.ImportPath+"."+expectedName
		}
		return actualStr == "*"+expectedName
	}

	if expected.ImportPath != "" {
		return actualStr == expected.ImportPath+"."+expected.Name
	}
	return actualStr == expected.Name
}

// matchReceiver validates the receiver type for method calls (Step 3-7).
// Returns false if go/types is unavailable — safe direction per design.
func matchReceiver(ctx *MatchContext, expected *TypeName) bool {
	if ctx.Sel == nil {
		return true // not a selector expression
	}

	// go/types 없으면 → SKIP (변환 안 함)
	// matching-layer.md Step 3-7: "go/types 없음 → SKIP (수신자 검증 불가 → 변환 안 함)"
	if !common.HasTypeInfo() {
		return false
	}

	return common.IsReceiverOfType(ctx.Sel.X, expected.ImportPath, expected.Name)
}

// matchFields validates struct field existence in CompositeLit (Step 3-9).
func matchFields(ctx *MatchContext, fields []FieldMatch) bool {
	if ctx.Lit == nil {
		return true // not a composite literal
	}

	for _, f := range fields {
		found := hasField(ctx.Lit, f.Name)
		if f.Required && !found {
			return false // Required field missing
		}
		if !f.Required && found {
			return false // Field must not exist but does
		}
	}
	return true
}

// hasField checks if a CompositeLit has a field with the given name.
func hasField(lit *dst.CompositeLit, fieldName string) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*dst.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*dst.Ident)
		if !ok {
			continue
		}
		if ident.Name == fieldName {
			return true
		}
	}
	return false
}
