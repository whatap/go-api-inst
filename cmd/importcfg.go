package cmd

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// parseImportCfgPath extracts the -importcfg file path from compile arguments.
// Returns empty string if not found.
func parseImportCfgPath(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "-importcfg" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(args[i], "-importcfg=") {
			return strings.TrimPrefix(args[i], "-importcfg=")
		}
	}
	return ""
}

// readImportCfg reads an importcfg file and returns the existing packagefile mappings.
// Format: "packagefile <import-path>=<archive-path>"
func readImportCfg(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "packagefile ") {
			continue
		}
		rest := strings.TrimPrefix(line, "packagefile ")
		idx := strings.Index(rest, "=")
		if idx < 0 {
			continue
		}
		importPath := rest[:idx]
		archivePath := rest[idx+1:]
		entries[importPath] = archivePath
	}

	return entries, scanner.Err()
}

// appendToImportCfg appends new packagefile entries to an importcfg file.
// Only adds entries whose import path is not already present.
func appendToImportCfg(path string, newEntries map[string]string) error {
	if len(newEntries) == 0 {
		return nil
	}

	// Read existing to avoid duplicates.
	existing, err := readImportCfg(path)
	if err != nil {
		return fmt.Errorf("read importcfg for append: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open importcfg for append: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for importPath, archivePath := range newEntries {
		if _, exists := existing[importPath]; exists {
			continue
		}
		fmt.Fprintf(w, "packagefile %s=%s\n", importPath, archivePath)
	}
	return w.Flush()
}

// extractImportsFromGoFile parses a Go file (imports only) and returns
// the list of import paths.
func extractImportsFromGoFile(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	var imports []string
	for _, imp := range f.Imports {
		// imp.Path.Value is quoted, e.g., `"fmt"`.
		p := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, p)
	}
	return imports, nil
}

// findNewImports compares before and after import lists and returns
// the imports that are new (present in after but not in before).
func findNewImports(before, after []string) []string {
	set := make(map[string]struct{}, len(before))
	for _, p := range before {
		set[p] = struct{}{}
	}

	var newImports []string
	for _, p := range after {
		if _, exists := set[p]; !exists {
			newImports = append(newImports, p)
		}
	}
	return newImports
}

