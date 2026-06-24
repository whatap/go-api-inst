package custom

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/whatap/go-api-inst/config"
)

// packageToPath converts a Go package path ("pkg/user") to a filesystem path
// using the OS separator. (Inlined after §227 Step 5 deleted util.go.)
func packageToPath(pkgPath string) string {
	return strings.ReplaceAll(pkgPath, "/", string(filepath.Separator))
}

// ApplyAddRules creates new files from add rules. `append: true` was removed
// in v0.5.5 — the yaml loader rejects it, so rules reaching this point are
// always new-file creations. baseDir is the base directory for relative
// paths (config.BaseDir).
func ApplyAddRules(dstDir, baseDir string, rules []config.AddRule) error {
	for _, rule := range rules {
		filePath := getAddFilePath(dstDir, rule)

		// Determine content
		content := rule.Content
		if rule.ContentFile != "" {
			// Resolve content_file path relative to baseDir
			contentFilePath := resolveRelativePath(baseDir, rule.ContentFile)
			data, err := os.ReadFile(contentFilePath)
			if err != nil {
				return err
			}
			content = string(data)
		}

		// Skip if content is empty
		if content == "" {
			continue
		}

		// Create directory
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return err
		}

		// Write file
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// resolveRelativePath resolves a relative path based on baseDir
// Returns the path as-is if it's an absolute path
func resolveRelativePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

// getAddFilePath returns the file path for an add rule
func getAddFilePath(dstDir string, rule config.AddRule) string {
	if rule.Package == "main" || rule.Package == "." || rule.Package == "" {
		return filepath.Join(dstDir, rule.File)
	}
	pkgPath := packageToPath(rule.Package)
	return filepath.Join(dstDir, pkgPath, rule.File)
}

// RemoveAddRules deletes files created by add rules.
func RemoveAddRules(dstDir string, rules []config.AddRule) error {
	for _, rule := range rules {
		pkgPath := packageToPath(rule.Package)
		filePath := filepath.Join(dstDir, pkgPath, rule.File)
		os.Remove(filePath)
	}
	return nil
}
