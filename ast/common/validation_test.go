package common

import (
	"testing"

	"github.com/dave/dst"
)

// makeCall creates a CallExpr with n arguments for testing.
func makeCall(n int) *dst.CallExpr {
	args := make([]dst.Expr, n)
	for i := 0; i < n; i++ {
		args[i] = dst.NewIdent("x")
	}
	return &dst.CallExpr{
		Fun:  dst.NewIdent("f"),
		Args: args,
	}
}

func TestFixedArgs(t *testing.T) {
	sig := FixedArgs(2)
	if sig.MinArgs != 2 || sig.MaxArgs != 2 {
		t.Errorf("FixedArgs(2) = {%d, %d}, want {2, 2}", sig.MinArgs, sig.MaxArgs)
	}
}

func TestVariadic(t *testing.T) {
	sig := Variadic(1)
	if sig.MinArgs != 1 || sig.MaxArgs != -1 {
		t.Errorf("Variadic(1) = {%d, %d}, want {1, -1}", sig.MinArgs, sig.MaxArgs)
	}
}

func TestRangeArgs(t *testing.T) {
	sig := RangeArgs(1, 3)
	if sig.MinArgs != 1 || sig.MaxArgs != 3 {
		t.Errorf("RangeArgs(1, 3) = {%d, %d}, want {1, 3}", sig.MinArgs, sig.MaxArgs)
	}
}

func TestValidateArgCount_ZeroValue(t *testing.T) {
	// Zero value (not defined) = no validation, always true
	sig := CallSignature{}
	for _, n := range []int{0, 1, 5, 100} {
		if !ValidateArgCount(makeCall(n), sig) {
			t.Errorf("zero-value sig should always return true, got false for %d args", n)
		}
	}
}

func TestValidateArgCount_FixedArgs(t *testing.T) {
	tests := []struct {
		name  string
		nArgs int
		fixed int
		want  bool
	}{
		{"exact_match", 2, 2, true},
		{"too_few", 1, 2, false},
		{"too_many", 3, 2, false},
		{"zero_args_match", 0, 0, true},
		{"zero_args_reject_one", 1, 0, false},
		{"one_arg_exact", 1, 1, true},
		{"one_arg_none", 0, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := FixedArgs(tt.fixed)
			got := ValidateArgCount(makeCall(tt.nArgs), sig)
			if got != tt.want {
				t.Errorf("ValidateArgCount(%d args, FixedArgs(%d)) = %v, want %v",
					tt.nArgs, tt.fixed, got, tt.want)
			}
		})
	}
}

func TestValidateArgCount_Variadic(t *testing.T) {
	tests := []struct {
		name  string
		nArgs int
		min   int
		want  bool
	}{
		{"at_min", 2, 2, true},
		{"above_min", 5, 2, true},
		{"below_min", 1, 2, false},
		{"zero_min_zero_args", 0, 0, true},
		{"zero_min_many_args", 10, 0, true},
		{"one_min_zero_args", 0, 1, false},
		{"one_min_one_arg", 1, 1, true},
		{"one_min_many_args", 100, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := Variadic(tt.min)
			got := ValidateArgCount(makeCall(tt.nArgs), sig)
			if got != tt.want {
				t.Errorf("ValidateArgCount(%d args, Variadic(%d)) = %v, want %v",
					tt.nArgs, tt.min, got, tt.want)
			}
		})
	}
}

func TestValidateArgCount_RangeArgs(t *testing.T) {
	tests := []struct {
		name  string
		nArgs int
		min   int
		max   int
		want  bool
	}{
		{"in_range", 2, 1, 3, true},
		{"at_min", 1, 1, 3, true},
		{"at_max", 3, 1, 3, true},
		{"below_min", 0, 1, 3, false},
		{"above_max", 4, 1, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := RangeArgs(tt.min, tt.max)
			got := ValidateArgCount(makeCall(tt.nArgs), sig)
			if got != tt.want {
				t.Errorf("ValidateArgCount(%d args, RangeArgs(%d, %d)) = %v, want %v",
					tt.nArgs, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestValidateArgCount_EmptyCall(t *testing.T) {
	call := &dst.CallExpr{Fun: dst.NewIdent("f")}
	sig := FixedArgs(1)
	if ValidateArgCount(call, sig) {
		t.Error("expected false for 0-arg call with FixedArgs(1)")
	}
}
