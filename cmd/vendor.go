package cmd

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/whatap/go-api-inst/config"
)

// §188: vendor project detection — matches Go command's auto-vendor logic
// (cmd/go/internal/modload/init.go setDefaultBuildMod)
//
// Go 1.14+ auto-vendor conditions (ALL must be true):
//   1. -mod flag NOT explicitly set (neither CLI nor GOFLAGS)
//   2. go.mod "go" directive >= 1.14
//   3. vendor/ directory exists
//   4. vendor/modules.txt exists

// isVendorProject detects if the project uses vendor mode.
// buildArgs: user's go build arguments (e.g., ["-o", "myapp", "./..."])
func isVendorProject(projectDir string, buildArgs []string) bool {
	// 1. Check explicit -mod flag (GOFLAGS + CLI args)
	explicitMod := getExplicitMod(buildArgs)
	switch explicitMod {
	case "mod", "readonly":
		return false // explicitly NOT vendor
	case "vendor":
		return true // explicitly vendor
	}

	// 2. Check vendor/modules.txt exists
	modulesFile := filepath.Join(projectDir, "vendor", "modules.txt")
	if _, err := os.Stat(modulesFile); err != nil {
		return false
	}

	// 3. Check go.mod "go" directive >= 1.14
	goVersion := parseGoDirective(filepath.Join(projectDir, "go.mod"))
	if goVersion == "" || !isGoVersionAtLeast(goVersion, 1, 14) {
		return false
	}

	return true
}

// getExplicitMod extracts -mod= value from GOFLAGS and CLI build args.
// Returns "" if not set, or "vendor"/"mod"/"readonly".
func getExplicitMod(buildArgs []string) string {
	// 1. Check GOFLAGS (higher priority — Go command processes GOFLAGS first)
	goflags := os.Getenv("GOFLAGS")
	if mod := parseModFlag(goflags); mod != "" {
		return mod
	}

	// 2. Check CLI build args
	for _, arg := range buildArgs {
		if mod := parseModFlag(arg); mod != "" {
			return mod
		}
	}

	return ""
}

// parseModFlag extracts -mod= value from a flag string.
// Handles: "-mod=vendor", "-mod=mod", "-mod=readonly"
// Also handles GOFLAGS with multiple flags: "-count=1 -mod=vendor"
func parseModFlag(s string) string {
	// Split by whitespace (GOFLAGS can have multiple flags)
	for _, field := range strings.Fields(s) {
		if v, ok := strings.CutPrefix(field, "-mod="); ok {
			return v
		}
	}
	return ""
}

// parseGoDirective extracts the "go X.Y" version from go.mod.
// Returns "" if go.mod doesn't exist or has no go directive.
func parseGoDirective(goModPath string) string {
	f, err := os.Open(goModPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Match "go 1.21" or "go 1.21.5" (not "go-api" or "golang.org")
		if strings.HasPrefix(line, "go ") && !strings.HasPrefix(line, "go.") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

// isGoVersionAtLeast checks if version string >= major.minor.
// version: "1.21", "1.14", "1.21.5", etc.
func isGoVersionAtLeast(version string, major, minor int) bool {
	// Strip toolchain suffix (e.g., "1.21rc1" → "1.21")
	// Only compare major.minor
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}

	vmajor, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}

	// Minor might have suffix like "21rc1" — extract leading digits
	minorStr := parts[1]
	vminor := 0
	for i, c := range minorStr {
		if c < '0' || c > '9' {
			minorStr = minorStr[:i]
			break
		}
	}
	vminor, err = strconv.Atoi(minorStr)
	if err != nil {
		return false
	}

	if vmajor != major {
		return vmajor > major
	}
	return vminor >= minor
}

// excludePatternsWithoutVendor returns DefaultExcludePatterns with "vendor/**" removed.
// Used in toolexec vendor mode — vendor/ files are filtered by isInVendor() instead.
func excludePatternsWithoutVendor() []string {
	var filtered []string
	for _, p := range config.DefaultExcludePatterns {
		if p != "vendor/**" {
			filtered = append(filtered, p)
		}
	}
	return filtered
}
