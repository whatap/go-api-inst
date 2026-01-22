package custom

import (
	"os"
	"path/filepath"

	"github.com/whatap/go-api-inst/config"
)

// ApplyAddRules applies new file creation rules (excluding append mode)
// Must be executed before file processing at InjectDir level
// baseDir: base directory for relative paths (config.BaseDir)
func ApplyAddRules(dstDir, baseDir string, rules []config.AddRule) error {
	for _, rule := range rules {
		// Process append mode later (after file copy)
		if rule.Append {
			continue
		}

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

// ApplyAppendRules appends code to existing files (called after file copy)
// baseDir: base directory for relative paths (config.BaseDir)
func ApplyAppendRules(dstDir, baseDir string, rules []config.AddRule) error {
	for _, rule := range rules {
		// Process append mode only
		if !rule.Append {
			continue
		}

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

		// Read existing file
		existing, err := os.ReadFile(filePath)
		if err != nil {
			// Create new file if it doesn't exist
			if os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
					return err
				}
				return os.WriteFile(filePath, []byte(content), 0644)
			}
			return err
		}

		// Merge existing content + new content
		newContent := string(existing) + "\n\n" + content

		// Write file
		if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
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

// RemoveAddRules removes created files
func RemoveAddRules(dstDir string, rules []config.AddRule) error {
	for _, rule := range rules {
		// Only delete entire file if not in append mode
		if !rule.Append {
			pkgPath := packageToPath(rule.Package)
			filePath := filepath.Join(dstDir, pkgPath, rule.File)
			os.Remove(filePath)
		}
		// Append mode requires markers for accurate removal
	}
	return nil
}
