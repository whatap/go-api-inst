package report

import (
	"testing"
)

// §146 / §242: matchTransformer — version filtering works AND returns the
// matched supported package path (§242 removed the Name field, so matches
// now identify themselves by ImportPath).
func TestMatchTransformer_VersionFiltering(t *testing.T) {
	supportedPaths := map[string]transformerEntry{
		"github.com/labstack/echo": {
			supportedVersions: []string{"v4"},
		},
		"github.com/gofiber/fiber": {
			supportedVersions: []string{"v2"},
		},
		"github.com/redis/go-redis": {
			supportedVersions: []string{"v9"},
		},
		"github.com/gin-gonic/gin": {
			supportedVersions: nil, // no version filtering
		},
	}

	tests := []struct {
		name     string
		depPath  string
		wantOK   bool
		wantPath string
	}{
		// Supported versions
		{"echo v4 supported", "github.com/labstack/echo/v4", true, "github.com/labstack/echo"},
		{"fiber v2 supported", "github.com/gofiber/fiber/v2", true, "github.com/gofiber/fiber"},
		{"goredis v9 supported", "github.com/redis/go-redis/v9", true, "github.com/redis/go-redis"},

		// Unsupported versions — §146 핵심: prefix만으로 매칭하면 안 됨
		{"echo v5 unsupported", "github.com/labstack/echo/v5", false, ""},
		{"echo v6 unsupported", "github.com/labstack/echo/v6", false, ""},
		{"fiber v3 unsupported", "github.com/gofiber/fiber/v3", false, ""},
		{"goredis v10 unsupported", "github.com/redis/go-redis/v10", false, ""},

		// Subpackages of supported versions — should match
		{"echo v4 middleware", "github.com/labstack/echo/v4/middleware", true, "github.com/labstack/echo"},
		{"fiber v2 utils", "github.com/gofiber/fiber/v2/utils", true, "github.com/gofiber/fiber"},

		// Subpackages of unsupported versions — should NOT match
		{"echo v5 middleware", "github.com/labstack/echo/v5/middleware", false, ""},
		{"fiber v3 utils", "github.com/gofiber/fiber/v3/utils", false, ""},

		// No version filtering (nil) — all versions match
		{"gin no version", "github.com/gin-gonic/gin", true, "github.com/gin-gonic/gin"},
		{"gin subpackage", "github.com/gin-gonic/gin/binding", true, "github.com/gin-gonic/gin"},

		// Completely unrelated
		{"unrelated package", "github.com/stretchr/testify", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := matchTransformer(tt.depPath, supportedPaths)
			if ok != tt.wantOK {
				t.Errorf("§146/§242: matchTransformer(%q) supported = %v, want %v", tt.depPath, ok, tt.wantOK)
			}
			if got != tt.wantPath {
				t.Errorf("§146/§242: matchTransformer(%q) = %q, want %q", tt.depPath, got, tt.wantPath)
			}
		})
	}
}

// §146: extractVersionFromPath
func TestExtractVersionFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"github.com/labstack/echo/v4", "v4"},
		{"github.com/gofiber/fiber/v2", "v2"},
		{"github.com/redis/go-redis/v9", "v9"},
		{"github.com/gin-gonic/gin", ""},
		{"github.com/gorilla/mux", ""},
		{"github.com/labstack/echo/v4/middleware", ""}, // middleware is not version
		{"github.com/gofiber/fiber/v2/utils", ""},      // utils is not version
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractVersionFromPath(tt.path)
			if got != tt.want {
				t.Errorf("extractVersionFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// §146: isVersionSuffix (report 패키지 내부)
func TestIsVersionSuffix_Report(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"v2", true},
		{"v4", true},
		{"v9", true},
		{"v10", true},
		{"v0", true},
		{"slog", false},
		{"middleware", false},
		{"utils", false},
		{"", false},
		{"v", false},
		{"vx", false},
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
