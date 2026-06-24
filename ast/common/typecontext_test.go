package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dave/dst"
)

func TestSetTypeContext(t *testing.T) {
	// Initially no type info
	if HasTypeInfo() {
		t.Error("HasTypeInfo should be false initially")
	}

	// GetIdentPath with nil should return ""
	if path := GetIdentPath(nil); path != "" {
		t.Errorf("GetIdentPath(nil) = %q, want empty", path)
	}

	// GetIdentPath without type context returns "" (no TypesInfo.Uses available)
	ident := &dst.Ident{Name: "sql"}
	if path := GetIdentPath(ident); path != "" {
		t.Errorf("GetIdentPath without type context = %q, want empty", path)
	}

	// ResolveType without type context returns nil
	if typ := ResolveType(ident); typ != nil {
		t.Error("ResolveType should return nil without type context")
	}

	// ClearTypeContext is safe to call multiple times
	ClearTypeContext()
	ClearTypeContext()
	if HasTypeInfo() {
		t.Error("HasTypeInfo should be false after clear")
	}
}

func TestTrySetupTypeContext_InvalidDir(t *testing.T) {
	tc := NewTypeChecker()

	// Non-existent directory should return nil (fallback)
	file := TrySetupTypeContext(tc, "/nonexistent/path/main.go")
	if file != nil {
		t.Error("TrySetupTypeContext should return nil for invalid path")
	}
	if HasTypeInfo() {
		t.Error("HasTypeInfo should be false after failed setup")
	}
}

func TestTrySetupTypeContext_ValidPackage(t *testing.T) {
	tc := NewTypeChecker()

	// Use the common package itself as test target
	// typecontext.go is in this package directory
	file := TrySetupTypeContext(tc, "typecontext.go")
	if file == nil {
		// This may fail if packages.Load can't resolve (e.g., missing deps)
		// In that case, it's a valid fallback — skip the test
		t.Skip("TrySetupTypeContext returned nil (packages.Load may have failed)")
	}

	if !HasTypeInfo() {
		t.Error("HasTypeInfo should be true after successful setup")
	}

	// Verify the file has content
	if file.Name == nil || file.Name.Name == "" {
		t.Error("Decorated file should have package name")
	}

	// Verify GetIdentPath works for package references.
	// typecontext.go imports "go/ast", "go/types", etc.
	// Find an identifier whose GetIdentPath returns a non-empty path.
	foundPath := false
	dst.Inspect(file, func(n dst.Node) bool {
		if ident, ok := n.(*dst.Ident); ok {
			if p := GetIdentPath(ident); p != "" {
				foundPath = true
				return false
			}
		}
		return true
	})

	if !foundPath {
		t.Error("Expected at least one Ident with non-empty GetIdentPath")
	}

	ClearTypeContext()
}

func TestIsReceiverOfType_NoTypeInfo(t *testing.T) {
	// Without type info, IsReceiverOfType always returns false
	ClearTypeContext()
	ident := &dst.Ident{Name: "mux"}
	if IsReceiverOfType(ident, "net/http", "ServeMux") {
		t.Error("IsReceiverOfType should return false without type info")
	}
}

func TestNamedTypeOf_NoTypeInfo(t *testing.T) {
	// Without type info, NamedTypeOf returns ok=false
	ClearTypeContext()
	if _, _, ok := NamedTypeOf(&dst.Ident{Name: "x"}); ok {
		t.Error("NamedTypeOf should return ok=false without type info")
	}
}

// TestNamedTypeOf_GenericReceiver verifies that NamedTypeOf drops the type
// arguments of an instantiated generic receiver, returning the origin type's
// package path and name (§282). This is the shared helper behind
// resolveMethodTarget and the 3-7 receiver filter (IsReceiverOfType).
func TestNamedTypeOf_GenericReceiver(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module gen\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	src := `package main

type Chain[I any, O any] struct{}

func (c *Chain[I, O]) Append(x any) *Chain[I, O] { return c }

func NewChain[I any, O any]() *Chain[I, O] { return &Chain[I, O]{} }

type Box struct{}

func (b *Box) Open() {}

func main() {
	ch := NewChain[string, int]()
	ch.Append(nil)
	box := &Box{}
	box.Open()
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	tc := NewTypeChecker()
	file := TrySetupTypeContext(tc, filepath.Join(tmpDir, "main.go"))
	if file == nil {
		t.Skip("TrySetupTypeContext returned nil (packages.Load may not work in test env)")
	}
	defer ClearTypeContext()

	type res struct {
		pkg, name string
		ok        bool
	}
	got := map[string]res{}
	dst.Inspect(file, func(n dst.Node) bool {
		call, ok := n.(*dst.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*dst.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "Append" || sel.Sel.Name == "Open" {
			p, nm, ok := NamedTypeOf(sel.X)
			got[sel.Sel.Name] = res{p, nm, ok}
		}
		return true
	})

	// Generic receiver: name must be "Chain" with NO bracket/type args.
	g := got["Append"]
	if !g.ok || g.pkg != "gen" || g.name != "Chain" {
		t.Errorf("generic NamedTypeOf = %+v, want {pkg:gen name:Chain ok:true}", g)
	}
	// Non-generic receiver: regression guard.
	b := got["Open"]
	if !b.ok || b.pkg != "gen" || b.name != "Box" {
		t.Errorf("non-generic NamedTypeOf = %+v, want {pkg:gen name:Box ok:true}", b)
	}

	// IsReceiverOfType (3-7 filter) must also accept the generic receiver.
	dst.Inspect(file, func(n dst.Node) bool {
		call, ok := n.(*dst.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*dst.SelectorExpr); ok && sel.Sel.Name == "Append" {
			if !IsReceiverOfType(sel.X, "gen", "Chain") {
				t.Error("IsReceiverOfType should match generic receiver gen.Chain")
			}
		}
		return true
	})
}

func TestIsReceiverOfType_NilExpr(t *testing.T) {
	// nil expression should return false
	ClearTypeContext()
	if IsReceiverOfType(nil, "net/http", "ServeMux") {
		t.Error("IsReceiverOfType should return false for nil expr")
	}
}

func TestMatchIdentPkg_NoTypeInfo(t *testing.T) {
	// Without type info, MatchIdentPkg falls back to ident.Name == pkgName
	ClearTypeContext()

	ident := &dst.Ident{Name: "sql"}
	if !MatchIdentPkg(ident, "sql", "database/sql") {
		t.Error("MatchIdentPkg should match by name without type info")
	}
	if MatchIdentPkg(ident, "http", "net/http") {
		t.Error("MatchIdentPkg should not match different name without type info")
	}
}

func TestMatchIdentPkg_VersionedImport(t *testing.T) {
	// Without type info, versioned prefix doesn't matter — falls back to name match
	ClearTypeContext()

	ident := &dst.Ident{Name: "echo"}
	if !MatchIdentPkg(ident, "echo", "github.com/labstack/echo") {
		t.Error("MatchIdentPkg should match by name for versioned import without type info")
	}
}

func TestMatchCallPkg_BasicCall(t *testing.T) {
	// Test pkg.Func() pattern matching without type info
	ClearTypeContext()

	call := &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   &dst.Ident{Name: "sql"},
			Sel: &dst.Ident{Name: "Open"},
		},
	}

	ident, funcName, matched := MatchCallPkg(call, "sql", "database/sql")
	if !matched {
		t.Error("MatchCallPkg should match sql.Open")
	}
	if ident == nil || ident.Name != "sql" {
		t.Error("MatchCallPkg should return ident with name 'sql'")
	}
	if funcName != "Open" {
		t.Errorf("MatchCallPkg funcName = %q, want 'Open'", funcName)
	}

	// Non-matching call
	_, _, matched = MatchCallPkg(call, "http", "net/http")
	if matched {
		t.Error("MatchCallPkg should not match sql.Open with pkgName 'http'")
	}
}

func TestMatchCallPkg_NonSelectorExpr(t *testing.T) {
	// Test non-SelectorExpr pattern (e.g., func())
	ClearTypeContext()

	call := &dst.CallExpr{
		Fun: &dst.Ident{Name: "myFunc"},
	}

	ident, funcName, matched := MatchCallPkg(call, "sql", "database/sql")
	if matched {
		t.Error("MatchCallPkg should not match non-SelectorExpr")
	}
	if ident != nil {
		t.Error("MatchCallPkg should return nil ident for non-SelectorExpr")
	}
	if funcName != "" {
		t.Errorf("MatchCallPkg funcName should be empty, got %q", funcName)
	}
}

// §180: MatchIdentPkg with go/types must reject stdlib subpackages.
// e.g., "log/slog" must NOT match importPrefix "log".
func TestMatchIdentPkg_WithTypeInfo_StdlibSubpackages(t *testing.T) {
	// Create temp project with both "log" and "log/slog"
	tmpDir := t.TempDir()

	goMod := "module testpkg\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	src := `package main

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
)

func main() {
	// log package calls
	log.SetOutput(os.Stderr)
	log.Println("hello")

	// log/slog calls — must NOT be matched by log transformer
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	logger.Info("hi")

	// net/http calls
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	// net/http/pprof — must NOT be matched by net/http transformer
	_ = pprof.Handler("goroutine")

	// database/sql calls
	db, _ := sql.Open("sqlite3", ":memory:")
	_ = db

	// database/sql/driver — must NOT be matched by sql transformer
	var _ driver.Driver
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	tc := NewTypeChecker()
	file := TrySetupTypeContext(tc, filepath.Join(tmpDir, "main.go"))
	if file == nil {
		t.Skip("TrySetupTypeContext failed (packages.Load may not work in test env)")
	}
	defer ClearTypeContext()

	if !HasTypeInfo() {
		t.Fatal("HasTypeInfo should be true")
	}

	// Collect all pkg.Func() calls and their resolved import paths
	type callInfo struct {
		pkgIdent    *dst.Ident
		funcName    string
		identPath   string
	}
	var calls []callInfo

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
		calls = append(calls, callInfo{
			pkgIdent:  ident,
			funcName:  sel.Sel.Name,
			identPath: GetIdentPath(ident),
		})
		return true
	})

	// Verify: log transformer (importPrefix="log") must match log.* but NOT slog.*
	t.Run("log_vs_slog", func(t *testing.T) {
		for _, c := range calls {
			matched := MatchIdentPkg(c.pkgIdent, "log", "log")
			if c.identPath == "log" && !matched {
				t.Errorf("MatchIdentPkg should match log.%s (identPath=%q)", c.funcName, c.identPath)
			}
			if c.identPath == "log/slog" && matched {
				t.Errorf("MatchIdentPkg must NOT match slog.%s (identPath=%q) for importPrefix 'log'", c.funcName, c.identPath)
			}
		}
	})

	// Verify: net/http transformer (importPrefix="net/http") must match http.* but NOT pprof.*
	t.Run("http_vs_pprof", func(t *testing.T) {
		for _, c := range calls {
			matched := MatchIdentPkg(c.pkgIdent, "http", "net/http")
			if c.identPath == "net/http" && !matched {
				t.Errorf("MatchIdentPkg should match http.%s (identPath=%q)", c.funcName, c.identPath)
			}
			if c.identPath == "net/http/pprof" && matched {
				t.Errorf("MatchIdentPkg must NOT match pprof.%s (identPath=%q) for importPrefix 'net/http'", c.funcName, c.identPath)
			}
		}
	})

	// Verify: database/sql transformer (importPrefix="database/sql") must match sql.* but NOT driver.*
	t.Run("sql_vs_driver", func(t *testing.T) {
		for _, c := range calls {
			matched := MatchIdentPkg(c.pkgIdent, "sql", "database/sql")
			if c.identPath == "database/sql" && !matched {
				t.Errorf("MatchIdentPkg should match sql.%s (identPath=%q)", c.funcName, c.identPath)
			}
			if c.identPath == "database/sql/driver" && matched {
				t.Errorf("MatchIdentPkg must NOT match driver.%s (identPath=%q) for importPrefix 'database/sql'", c.funcName, c.identPath)
			}
		}
	})

	// Verify: fmt transformer — should match fmt.* calls
	t.Run("fmt", func(t *testing.T) {
		for _, c := range calls {
			if c.identPath == "fmt" {
				matched := MatchIdentPkg(c.pkgIdent, "fmt", "fmt")
				if !matched {
					t.Errorf("MatchIdentPkg should match fmt.%s", c.funcName)
				}
			}
		}
	})
}

// §180: MatchIdentPkg with go/types must allow versioned imports.
// e.g., "github.com/labstack/echo/v4" must match importPrefix "github.com/labstack/echo".
func TestMatchIdentPkg_WithTypeInfo_VersionedImports(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := `module testpkg

go 1.21

require github.com/labstack/echo/v4 v4.13.3
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	src := `package main

import (
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	_ = e
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// go mod tidy to download deps
	// Skip if network or deps unavailable
	tc := NewTypeChecker()
	file := TrySetupTypeContext(tc, filepath.Join(tmpDir, "main.go"))
	if file == nil {
		t.Skip("TrySetupTypeContext failed (deps may not be available)")
	}
	defer ClearTypeContext()

	// Find echo.New() call
	found := false
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
		identPath := GetIdentPath(ident)
		if identPath == "github.com/labstack/echo/v4" {
			// Must match importPrefix "github.com/labstack/echo" (version suffix)
			if !MatchIdentPkg(ident, "echo", "github.com/labstack/echo") {
				t.Errorf("MatchIdentPkg should match echo/v4 with prefix 'github.com/labstack/echo'")
			}
			found = true
			return false
		}
		return true
	})

	if !found {
		t.Log("echo.New() call not found — deps may not have resolved")
	}
}

// §180: Fallback path (no go/types) — GetPackageNameForImport vs GetPackageNameForImportPrefix
// for stdlib packages with subpackages.
func TestMatchIdentPkg_Fallback_StdlibSubpackages(t *testing.T) {
	ClearTypeContext()

	tests := []struct {
		name         string
		identName    string
		pkgName      string
		importPrefix string
		want         bool
	}{
		// log transformer: must match "log" but NOT "slog"
		{"log.Println matches", "log", "log", "log", true},
		{"slog must not match log", "slog", "log", "log", false},

		// net/http transformer: must match "http" but NOT "pprof"/"httptest"
		{"http.HandleFunc matches", "http", "http", "net/http", true},
		{"pprof must not match http", "pprof", "http", "net/http", false},
		{"httptest must not match http", "httptest", "http", "net/http", false},

		// database/sql transformer: must match "sql" but NOT "driver"
		{"sql.Open matches", "sql", "sql", "database/sql", true},
		{"driver must not match sql", "driver", "sql", "database/sql", false},

		// fmt transformer: must match "fmt"
		{"fmt.Println matches", "fmt", "fmt", "fmt", true},

		// Versioned packages: name-based matching
		{"echo matches echo", "echo", "echo", "github.com/labstack/echo", true},
		{"redis matches redis", "redis", "redis", "github.com/redis/go-redis", true},
		{"fiber matches fiber", "fiber", "fiber", "github.com/gofiber/fiber", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ident := &dst.Ident{Name: tt.identName}
			got := MatchIdentPkg(ident, tt.pkgName, tt.importPrefix)
			if got != tt.want {
				t.Errorf("MatchIdentPkg(ident=%q, pkgName=%q, prefix=%q) = %v, want %v",
					tt.identName, tt.pkgName, tt.importPrefix, got, tt.want)
			}
		})
	}
}

// §180: MatchCallPkg fallback must reject stdlib subpackage calls.
func TestMatchCallPkg_Fallback_StdlibSubpackages(t *testing.T) {
	ClearTypeContext()

	tests := []struct {
		name         string
		pkgIdent     string
		funcName     string
		pkgName      string
		importPrefix string
		wantMatch    bool
	}{
		// log transformer
		{"log.New matches", "log", "New", "log", "log", true},
		{"slog.New must not match log", "slog", "New", "log", "log", false},

		// net/http transformer
		{"http.ListenAndServe matches", "http", "ListenAndServe", "http", "net/http", true},
		{"pprof.Handler must not match http", "pprof", "Handler", "http", "net/http", false},

		// database/sql transformer
		{"sql.Open matches", "sql", "Open", "sql", "database/sql", true},
		{"driver.Open must not match sql", "driver", "Open", "sql", "database/sql", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call := &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X:   &dst.Ident{Name: tt.pkgIdent},
					Sel: &dst.Ident{Name: tt.funcName},
				},
			}
			_, _, matched := MatchCallPkg(call, tt.pkgName, tt.importPrefix)
			if matched != tt.wantMatch {
				t.Errorf("MatchCallPkg(%s.%s, pkgName=%q, prefix=%q) = %v, want %v",
					tt.pkgIdent, tt.funcName, tt.pkgName, tt.importPrefix, matched, tt.wantMatch)
			}
		})
	}
}

// §180: isVersionSuffix must correctly identify version patterns.
func TestIsVersionSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// Valid version suffixes
		{"v2", true},
		{"v4", true},
		{"v9", true},
		{"v10", true},
		{"v100", true},

		// Invalid — NOT version suffixes
		{"slog", false},        // §180: this was the bug
		{"pprof", false},       // net/http/pprof
		{"driver", false},      // database/sql/driver
		{"httptest", false},    // net/http/httptest
		{"middleware", false},  // echo/v4/middleware
		{"logger", false},     // fiber/v2/middleware/logger
		{"internal", false},   // go-redis/v9/internal
		{"v", false},          // too short
		{"", false},           // empty
		{"v0", true},          // v0 is valid
		{"v1", true},          // v1 is valid
		{"vx", false},         // not a number
		{"v4beta", false},     // not pure digits
		{"V4", false},         // uppercase
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isVersionSuffix(tt.input)
			if got != tt.want {
				t.Errorf("isVersionSuffix(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// §166: MatchIdentPkg — 서브패키지 오인식 방지 (fiber/v2/middleware/logger vs fiber/v2)
func TestMatchIdentPkg_SubpackageFalsePositive(t *testing.T) {
	// Without type info, MatchIdentPkg relies on fallback logic
	// fiber/v2/middleware/logger should NOT match "github.com/gofiber/fiber" prefix
	ClearTypeContext()

	tests := []struct {
		name        string
		identName   string
		identPath   string // simulated import path (from file imports)
		pkgName     string
		matchPrefix string
		want        bool
	}{
		{
			name:        "fiber.New matches fiber prefix",
			identName:   "fiber",
			identPath:   "github.com/gofiber/fiber/v2",
			pkgName:     "fiber",
			matchPrefix: "github.com/gofiber/fiber",
			want:        true,
		},
		{
			name:        "logger.New should NOT match fiber prefix",
			identName:   "logger",
			identPath:   "github.com/gofiber/fiber/v2/middleware/logger",
			pkgName:     "fiber",
			matchPrefix: "github.com/gofiber/fiber",
			want:        false,
		},
		{
			name:        "middleware.X should NOT match echo prefix",
			identName:   "middleware",
			identPath:   "github.com/labstack/echo/v4/middleware",
			pkgName:     "echo",
			matchPrefix: "github.com/labstack/echo",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ident := &dst.Ident{Name: tt.identName, Path: tt.identPath}
			got := MatchIdentPkg(ident, tt.pkgName, tt.matchPrefix)
			if got != tt.want {
				t.Errorf("§166: MatchIdentPkg(%q, path=%q, pkgName=%q, prefix=%q) = %v, want %v",
					tt.identName, tt.identPath, tt.pkgName, tt.matchPrefix, got, tt.want)
			}
		})
	}
}

// §164: IsReceiverOfType — go/types 없을 때 안전하게 처리
func TestIsReceiverOfType_FallbackSafety(t *testing.T) {
	ClearTypeContext()

	// Without type info, IsReceiverOfType should return false (safe fallback)
	ident := &dst.Ident{Name: "client"}
	got := IsReceiverOfType(ident, "github.com/aerospike/aerospike-client-go", "Client")
	if got {
		t.Error("§164: IsReceiverOfType without type info should return false (safe fallback)")
	}
}
