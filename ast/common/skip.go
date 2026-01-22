package common

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/whatap/go-api-inst/config"
)

// ShouldSkipPath checks if a path should be skipped based on patterns and system paths
// Parameters:
//   - path: absolute or relative file/directory path
//   - basePath: base directory for relative pattern matching
//   - excludePatterns: glob patterns to exclude (use config.DefaultExcludePatterns if nil)
func ShouldSkipPath(path string, basePath string, excludePatterns []string) bool {
	// Check system paths (GOROOT, GOMODCACHE)
	if isSystemPath(path) {
		return true
	}

	// Use default patterns if not specified
	if excludePatterns == nil {
		excludePatterns = config.DefaultExcludePatterns
	}

	// Get relative path for pattern matching
	relPath := path
	if basePath != "" && filepath.IsAbs(path) {
		if rel, err := filepath.Rel(basePath, path); err == nil {
			relPath = rel
		}
	}

	// Normalize path separators for cross-platform matching
	relPath = filepath.ToSlash(relPath)

	// Check exclude patterns
	for _, pattern := range excludePatterns {
		matched, err := doublestar.Match(pattern, relPath)
		if err == nil && matched {
			return true
		}

		// Also check against the filename only for file patterns
		filename := filepath.Base(path)
		if strings.Contains(pattern, "*") && !strings.Contains(pattern, "/") {
			matched, err = doublestar.Match(pattern, filename)
			if err == nil && matched {
				return true
			}
		}
	}

	return false
}

// ShouldSkipFile checks if a file should be skipped (convenience wrapper)
func ShouldSkipFile(filePath string, basePath string, excludePatterns []string) bool {
	return ShouldSkipPath(filePath, basePath, excludePatterns)
}

// ShouldSkipDirectory checks if a directory should be skipped
func ShouldSkipDirectory(dirPath string, basePath string, excludePatterns []string) bool {
	// Use default patterns if not specified
	if excludePatterns == nil {
		excludePatterns = config.DefaultExcludePatterns
	}

	base := filepath.Base(dirPath)

	// Check common skip directories (quick check before glob)
	skipDirs := []string{"vendor", ".git", "node_modules", "whatap-instrumented"}
	for _, dir := range skipDirs {
		if base == dir {
			return true
		}
	}

	return ShouldSkipPath(dirPath, basePath, excludePatterns)
}

// isSystemPath checks if the path is under a Go system directory
func isSystemPath(path string) bool {
	for _, envName := range config.SystemSkipPaths {
		var skipPath string
		switch envName {
		case "GOROOT":
			skipPath = os.Getenv("GOROOT")
			if skipPath == "" {
				// Try runtime.GOROOT() fallback by checking common patterns
				continue
			}
		case "GOMODCACHE":
			skipPath = os.Getenv("GOMODCACHE")
			if skipPath == "" {
				// Fallback: GOPATH/pkg/mod
				if gopath := os.Getenv("GOPATH"); gopath != "" {
					skipPath = filepath.Join(gopath, "pkg", "mod")
				} else {
					// Default GOPATH
					if home, err := os.UserHomeDir(); err == nil {
						skipPath = filepath.Join(home, "go", "pkg", "mod")
					}
				}
			}
		}

		if skipPath != "" {
			// Normalize paths for comparison
			skipPath = filepath.Clean(skipPath)
			cleanPath := filepath.Clean(path)

			if strings.HasPrefix(cleanPath, skipPath) {
				return true
			}
		}
	}

	return false
}
