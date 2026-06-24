package ast

import (
	"strings"
	"testing"
)

// ruleForTarget returns the builtin Rule whose Target matches, or nil.
func ruleForTarget(target string) *Rule {
	for _, r := range AllRules() {
		if r.Target == target {
			return r
		}
	}
	return nil
}

// TestGrpcChainInterceptor_Server is the §294 regression: the grpc.NewServer
// rule must inject the *Chain* interceptor options (additive), never the
// single-slot grpc.UnaryInterceptor/StreamInterceptor — otherwise a server that
// already configured its own interceptor panics at runtime
// ("the unary server interceptor was already set and may not be reset", seaweedfs).
func TestGrpcChainInterceptor_Server(t *testing.T) {
	r := ruleForTarget("google.golang.org/grpc.NewServer")
	if r == nil {
		t.Fatal("no rule for grpc.NewServer")
	}
	a, ok := r.Advice.(*ArgInsert)
	if !ok {
		t.Fatalf("grpc.NewServer advice is %T, want *ArgInsert", r.Advice)
	}

	src := `package p
func f() { grpc.NewServer(opts...) }
`
	file := parseTestFile(t, src)
	call := findFirstCall(file)
	ctx := &MatchContext{File: file, Call: call, FuncName: "NewServer", PkgName: "grpc", Mode: ModeInject}
	a.Apply(ctx)

	got := fileToString(t, file)
	if !strings.Contains(got, "grpc.ChainUnaryInterceptor(whatapgrpc.UnaryServerInterceptor())") {
		t.Errorf("expected ChainUnaryInterceptor injection, got:\n%s", got)
	}
	if !strings.Contains(got, "grpc.ChainStreamInterceptor(whatapgrpc.StreamServerInterceptor())") {
		t.Errorf("expected ChainStreamInterceptor injection, got:\n%s", got)
	}
	// Must NOT emit the single-slot options (would panic when user already set one).
	if strings.Contains(got, "grpc.UnaryInterceptor(") || strings.Contains(got, "grpc.StreamInterceptor(") {
		t.Errorf("§294 regression: single-slot interceptor option emitted (panics if user already set one):\n%s", got)
	}
}

// TestGrpcChainInterceptor_Client is the §294 regression for client dial rules:
// grpc.Dial/DialContext/NewClient must inject WithChain* (additive) so a user's
// existing WithUnaryInterceptor is not silently overwritten.
func TestGrpcChainInterceptor_Client(t *testing.T) {
	for _, target := range []string{
		"google.golang.org/grpc.Dial",
		"google.golang.org/grpc.DialContext",
		"google.golang.org/grpc.NewClient",
	} {
		r := ruleForTarget(target)
		if r == nil {
			t.Fatalf("no rule for %s", target)
		}
		a, ok := r.Advice.(*ArgInsert)
		if !ok {
			t.Fatalf("%s advice is %T, want *ArgInsert", target, r.Advice)
		}

		src := `package p
func f() { grpc.NewClient(addr, opts...) }
`
		file := parseTestFile(t, src)
		call := findFirstCall(file)
		ctx := &MatchContext{File: file, Call: call, FuncName: "NewClient", PkgName: "grpc", Mode: ModeInject}
		a.Apply(ctx)

		got := fileToString(t, file)
		if !strings.Contains(got, "grpc.WithChainUnaryInterceptor(whatapgrpc.UnaryClientInterceptor())") {
			t.Errorf("%s: expected WithChainUnaryInterceptor injection, got:\n%s", target, got)
		}
		if !strings.Contains(got, "grpc.WithChainStreamInterceptor(whatapgrpc.StreamClientInterceptor())") {
			t.Errorf("%s: expected WithChainStreamInterceptor injection, got:\n%s", target, got)
		}
	}
}
