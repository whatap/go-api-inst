package common

import (
	"go/token"
	"strconv"
	"testing"

	"github.com/dave/dst"
)

// makeFile creates a dst.File with the given import paths for testing.
func makeFile(imports ...string) *dst.File {
	specs := make([]dst.Spec, len(imports))
	impSpecs := make([]*dst.ImportSpec, len(imports))
	for i, imp := range imports {
		spec := &dst.ImportSpec{
			Path: &dst.BasicLit{
				Kind:  token.STRING,
				Value: strconv.Quote(imp),
			},
		}
		specs[i] = spec
		impSpecs[i] = spec
	}
	return &dst.File{
		Decls: []dst.Decl{
			&dst.GenDecl{
				Tok:   token.IMPORT,
				Specs: specs,
			},
		},
		Imports: impSpecs,
	}
}

func TestHasSupportedImport_Echo(t *testing.T) {
	prefix := "github.com/labstack/echo"
	supported := []string{"", "v4"}

	tests := []struct {
		name       string
		importPath string
		want       bool
	}{
		// 지원 버전
		{"echo (no version)", "github.com/labstack/echo", true},
		{"echo/v4", "github.com/labstack/echo/v4", true},
		{"echo/v4/middleware", "github.com/labstack/echo/v4/middleware", true},
		{"echo/middleware (no version sub-pkg)", "github.com/labstack/echo/middleware", true},

		// 미지원 버전
		{"echo/v5", "github.com/labstack/echo/v5", false},
		{"echo/v5/middleware", "github.com/labstack/echo/v5/middleware", false},
		{"echo/v6", "github.com/labstack/echo/v6", false},

		// 오탐 방지 (다른 패키지)
		{"echolab (different pkg)", "github.com/labstack/echolab", false},
		{"echov4 (no slash)", "github.com/labstack/echov4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := makeFile(tt.importPath)
			got := HasSupportedImport(file, prefix, supported)
			if got != tt.want {
				t.Errorf("HasSupportedImport(%q, %q, %v) = %v, want %v",
					tt.importPath, prefix, supported, got, tt.want)
			}
		})
	}
}

func TestHasSupportedImport_Fiber(t *testing.T) {
	prefix := "github.com/gofiber/fiber"
	supported := []string{"v2"}

	tests := []struct {
		name       string
		importPath string
		want       bool
	}{
		// 지원 버전
		{"fiber/v2", "github.com/gofiber/fiber/v2", true},
		{"fiber/v2/middleware/cors", "github.com/gofiber/fiber/v2/middleware/cors", true},

		// 미지원 버전
		{"fiber (no version, v1)", "github.com/gofiber/fiber", false},
		{"fiber/v3", "github.com/gofiber/fiber/v3", false},
		{"fiber/v3/middleware", "github.com/gofiber/fiber/v3/middleware", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := makeFile(tt.importPath)
			got := HasSupportedImport(file, prefix, supported)
			if got != tt.want {
				t.Errorf("HasSupportedImport(%q, %q, %v) = %v, want %v",
					tt.importPath, prefix, supported, got, tt.want)
			}
		})
	}
}

func TestHasSupportedImport_Chi(t *testing.T) {
	prefix := "github.com/go-chi/chi"
	supported := []string{"", "v5"}

	tests := []struct {
		name       string
		importPath string
		want       bool
	}{
		// 지원 버전
		{"chi (no version, v4.x)", "github.com/go-chi/chi", true},
		{"chi/v5", "github.com/go-chi/chi/v5", true},
		{"chi/v5/middleware", "github.com/go-chi/chi/v5/middleware", true},
		{"chi/middleware (no version sub-pkg)", "github.com/go-chi/chi/middleware", true},

		// 미지원 버전
		{"chi/v6", "github.com/go-chi/chi/v6", false},
		{"chi/v6/middleware", "github.com/go-chi/chi/v6/middleware", false},

		// 오탐 방지
		{"chi-render (different pkg)", "github.com/go-chi/chi-render", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := makeFile(tt.importPath)
			got := HasSupportedImport(file, prefix, supported)
			if got != tt.want {
				t.Errorf("HasSupportedImport(%q, %q, %v) = %v, want %v",
					tt.importPath, prefix, supported, got, tt.want)
			}
		})
	}
}

func TestHasSupportedImport_GoRedis(t *testing.T) {
	// New path (v9)
	t.Run("new_path", func(t *testing.T) {
		prefix := "github.com/redis/go-redis"
		supported := []string{"v9"}

		tests := []struct {
			name       string
			importPath string
			want       bool
		}{
			{"go-redis/v9", "github.com/redis/go-redis/v9", true},
			{"go-redis/v9/internal", "github.com/redis/go-redis/v9/internal", true},
			{"go-redis (no version)", "github.com/redis/go-redis", false},
			{"go-redis/v10", "github.com/redis/go-redis/v10", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				file := makeFile(tt.importPath)
				got := HasSupportedImport(file, prefix, supported)
				if got != tt.want {
					t.Errorf("HasSupportedImport(%q, %q, %v) = %v, want %v",
						tt.importPath, prefix, supported, got, tt.want)
				}
			})
		}
	})

	// Old path (v8)
	t.Run("old_path", func(t *testing.T) {
		prefix := "github.com/go-redis/redis"
		supported := []string{"v8"}

		tests := []struct {
			name       string
			importPath string
			want       bool
		}{
			{"redis/v8", "github.com/go-redis/redis/v8", true},
			{"redis/v8/internal", "github.com/go-redis/redis/v8/internal", true},
			{"redis (no version)", "github.com/go-redis/redis", false},
			{"redis/v7", "github.com/go-redis/redis/v7", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				file := makeFile(tt.importPath)
				got := HasSupportedImport(file, prefix, supported)
				if got != tt.want {
					t.Errorf("HasSupportedImport(%q, %q, %v) = %v, want %v",
						tt.importPath, prefix, supported, got, tt.want)
				}
			})
		}
	})
}

// §180: GetPackageNameForImport (exact match) vs GetPackageNameForImportPrefix (prefix match)
// for stdlib packages with subpackages.
func TestGetPackageNameForImport_StdlibExact(t *testing.T) {
	tests := []struct {
		name       string
		imports    []string
		queryPath  string
		want       string
	}{
		// log vs log/slog — exact match must return correct package
		{
			"log exact match (log first)",
			[]string{"log", "log/slog"},
			"log",
			"log",
		},
		{
			"log exact match (slog first)",
			[]string{"log/slog", "log"},
			"log",
			"log",
		},
		{
			"slog exact match",
			[]string{"log", "log/slog"},
			"log/slog",
			"slog",
		},
		{
			"log only — no slog",
			[]string{"log"},
			"log",
			"log",
		},
		{
			"slog only — log not imported",
			[]string{"log/slog"},
			"log",
			"", // must NOT match log/slog
		},

		// net/http vs net/http/pprof
		{
			"http exact match (pprof first)",
			[]string{"net/http/pprof", "net/http"},
			"net/http",
			"http",
		},
		{
			"pprof exact match",
			[]string{"net/http", "net/http/pprof"},
			"net/http/pprof",
			"pprof",
		},
		{
			"http only — pprof query returns empty",
			[]string{"net/http"},
			"net/http/pprof",
			"",
		},

		// database/sql vs database/sql/driver
		{
			"sql exact match (driver first)",
			[]string{"database/sql/driver", "database/sql"},
			"database/sql",
			"sql",
		},
		{
			"driver exact match",
			[]string{"database/sql", "database/sql/driver"},
			"database/sql/driver",
			"driver",
		},

		// fmt — no subpackages
		{
			"fmt exact",
			[]string{"fmt"},
			"fmt",
			"fmt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := makeFile(tt.imports...)
			got := GetPackageNameForImport(file, tt.queryPath)
			if got != tt.want {
				t.Errorf("GetPackageNameForImport(imports=%v, path=%q) = %q, want %q",
					tt.imports, tt.queryPath, got, tt.want)
			}
		})
	}
}

// §180: GetPackageNameForImportPrefix has a pitfall with stdlib subpackages.
// This test documents the problem that motivated using GetPackageNameForImport
// for stdlib transformers.
func TestGetPackageNameForImportPrefix_StdlibPitfall(t *testing.T) {
	tests := []struct {
		name       string
		imports    []string
		prefix     string
		wantSafe   bool   // true if result is safe (won't cause false positive)
		wantResult string // expected result
	}{
		{
			"log prefix with slog first — UNSAFE",
			[]string{"log/slog", "log"},
			"log",
			false,   // slog first → returns "slog" → log transformer matches slog.New()
			"slog",  // prefix match picks up log/slog first
		},
		{
			"log prefix with log first — safe by accident",
			[]string{"log", "log/slog"},
			"log",
			true,
			"log", // log appears first → correct, but order-dependent!
		},
		{
			"net/http prefix with pprof first — UNSAFE",
			[]string{"net/http/pprof", "net/http"},
			"net/http",
			false,
			"pprof",
		},
		{
			"database/sql prefix with driver first — UNSAFE",
			[]string{"database/sql/driver", "database/sql"},
			"database/sql",
			false,
			"driver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := makeFile(tt.imports...)
			got := GetPackageNameForImportPrefix(file, tt.prefix)
			if got != tt.wantResult {
				t.Errorf("GetPackageNameForImportPrefix(imports=%v, prefix=%q) = %q, want %q",
					tt.imports, tt.prefix, got, tt.wantResult)
			}
			if !tt.wantSafe {
				t.Logf("KNOWN PITFALL: prefix match returns %q — stdlib transformers must use GetPackageNameForImport instead", got)
			}
		})
	}
}

// §180: GetPackageNameForImportPrefix is correct for versioned packages (intended use case).
func TestGetPackageNameForImportPrefix_VersionedPackages(t *testing.T) {
	tests := []struct {
		name       string
		imports    []string
		prefix     string
		want       string
	}{
		{
			"echo/v4",
			[]string{"github.com/labstack/echo/v4"},
			"github.com/labstack/echo",
			"echo",
		},
		{
			"echo/v4 with middleware",
			[]string{"github.com/labstack/echo/v4", "github.com/labstack/echo/v4/middleware"},
			"github.com/labstack/echo",
			"echo",
		},
		{
			"go-redis/v9",
			[]string{"github.com/redis/go-redis/v9"},
			"github.com/redis/go-redis",
			"redis",
		},
		{
			"fiber/v2",
			[]string{"github.com/gofiber/fiber/v2"},
			"github.com/gofiber/fiber",
			"fiber",
		},
		{
			"chi/v5",
			[]string{"github.com/go-chi/chi/v5"},
			"github.com/go-chi/chi",
			"chi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := makeFile(tt.imports...)
			got := GetPackageNameForImportPrefix(file, tt.prefix)
			if got != tt.want {
				t.Errorf("GetPackageNameForImportPrefix(imports=%v, prefix=%q) = %q, want %q",
					tt.imports, tt.prefix, got, tt.want)
			}
		})
	}
}

// makeFileWithAlias creates a dst.File with aliased imports for testing.
func makeFileWithAlias(alias, importPath string) *dst.File {
	spec := &dst.ImportSpec{
		Name: &dst.Ident{Name: alias},
		Path: &dst.BasicLit{
			Kind:  token.STRING,
			Value: strconv.Quote(importPath),
		},
	}
	return &dst.File{
		Decls: []dst.Decl{
			&dst.GenDecl{
				Tok:   token.IMPORT,
				Specs: []dst.Spec{spec},
			},
		},
		Imports: []*dst.ImportSpec{spec},
	}
}

// §180: Aliased imports must be handled correctly by both functions.
func TestGetPackageName_Aliases(t *testing.T) {
	// import mylog "log"
	file := makeFileWithAlias("mylog", "log")

	got := GetPackageNameForImport(file, "log")
	if got != "mylog" {
		t.Errorf("GetPackageNameForImport with alias = %q, want 'mylog'", got)
	}

	got = GetPackageNameForImportPrefix(file, "log")
	if got != "mylog" {
		t.Errorf("GetPackageNameForImportPrefix with alias = %q, want 'mylog'", got)
	}

	// import h "net/http"
	file2 := makeFileWithAlias("h", "net/http")

	got = GetPackageNameForImport(file2, "net/http")
	if got != "h" {
		t.Errorf("GetPackageNameForImport with alias 'h' = %q, want 'h'", got)
	}
}

func TestHasSupportedImport_Aerospike(t *testing.T) {
	prefix := "github.com/aerospike/aerospike-client-go"
	supported := []string{"v6"}

	tests := []struct {
		name       string
		importPath string
		want       bool
	}{
		// 지원 버전
		{"aerospike/v6", "github.com/aerospike/aerospike-client-go/v6", true},
		{"aerospike/v6/types", "github.com/aerospike/aerospike-client-go/v6/types", true},

		// 미지원 버전 (v8: whatapas 패키지 미존재 §206)
		{"aerospike/v8", "github.com/aerospike/aerospike-client-go/v8", false},
		{"aerospike (no version, v1~v5)", "github.com/aerospike/aerospike-client-go", false},
		{"aerospike/v7", "github.com/aerospike/aerospike-client-go/v7", false},
		{"aerospike/v9", "github.com/aerospike/aerospike-client-go/v9", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := makeFile(tt.importPath)
			got := HasSupportedImport(file, prefix, supported)
			if got != tt.want {
				t.Errorf("HasSupportedImport(%q, %q, %v) = %v, want %v",
					tt.importPath, prefix, supported, got, tt.want)
			}
		})
	}
}
