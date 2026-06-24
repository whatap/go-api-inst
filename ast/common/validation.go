// Package common provides shared AST utilities.
// validation.go: Call signature validation for argument count matching (§158).
package common

import "github.com/dave/dst"

// CallSignature defines the expected argument count range for a function call.
// Use FixedArgs, Variadic, or RangeArgs constructors to create valid signatures.
// The zero value (all fields false/zero) means "no validation" for backward compatibility.
type CallSignature struct {
	MinArgs int  // minimum argument count (0-based)
	MaxArgs int  // maximum argument count (-1 = variadic/unlimited)
	defined bool // true if explicitly created via constructor
}

// FixedArgs returns a CallSignature for exactly n arguments.
func FixedArgs(n int) CallSignature {
	return CallSignature{MinArgs: n, MaxArgs: n, defined: true}
}

// Variadic returns a CallSignature for min or more arguments (no upper limit).
func Variadic(min int) CallSignature {
	return CallSignature{MinArgs: min, MaxArgs: -1, defined: true}
}

// RangeArgs returns a CallSignature for min to max arguments (inclusive).
func RangeArgs(min, max int) CallSignature {
	return CallSignature{MinArgs: min, MaxArgs: max, defined: true}
}

// ValidateArgCount checks if a call expression has the expected number of arguments.
// Returns true if the call matches the signature, false otherwise.
// If sig was not created via a constructor (zero value), always returns true (backward compatibility).
func ValidateArgCount(call *dst.CallExpr, sig CallSignature) bool {
	// Not explicitly defined = no validation (backward compatibility)
	if !sig.defined {
		return true
	}

	argCount := len(call.Args)

	// Check minimum
	if argCount < sig.MinArgs {
		return false
	}

	// Check maximum (-1 = unlimited)
	if sig.MaxArgs >= 0 && argCount > sig.MaxArgs {
		return false
	}

	return true
}
