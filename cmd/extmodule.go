package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/whatap/go-api-inst/config"
)

// ModuleDownloadInfo holds information from go mod download -json
type ModuleDownloadInfo struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
	Error   string `json:"Error,omitempty"`
}

// ExternalModuleResult holds the result of external module processing
type ExternalModuleResult struct {
	ModulePath string // original module path
	Version    string // resolved version
	LocalDir   string // local copy directory (absolute path)
}

// resolveModule resolves a module path to its GOMODCACHE location
// projectDir must contain a go.mod that requires the module
func resolveModule(modulePath, projectDir string) (*ModuleDownloadInfo, error) {
	// Resolve version: try go list -m first, then parse go.mod directly
	moduleArg := modulePath
	listCmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}@{{.Version}}", modulePath)
	listCmd.Dir = projectDir
	if listOut, err := listCmd.Output(); err == nil {
		resolved := strings.TrimSpace(string(listOut))
		if resolved != "" && strings.Contains(resolved, "@") {
			moduleArg = resolved
		}
	} else {
		// Fallback: parse version from go.mod directly
		if ver := findModuleVersionInGoMod(modulePath, projectDir); ver != "" {
			moduleArg = modulePath + "@" + ver
		}
	}

	cmd := exec.Command("go", "mod", "download", "-json", moduleArg)
	cmd.Dir = projectDir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go mod download -json %s: %w", moduleArg, err)
	}

	var info ModuleDownloadInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("parse module info %s: %w", modulePath, err)
	}
	if info.Error != "" {
		return nil, fmt.Errorf("module %s: %s", modulePath, info.Error)
	}
	if info.Dir == "" {
		return nil, fmt.Errorf("module %s: no Dir in download info", modulePath)
	}
	return &info, nil
}

// sanitizeModulePath converts a module path to a safe directory name
// e.g., "gitrepo.xlaxiata.id/go-module/goutils/v3" → "gitrepo_xlaxiata_id_go-module_goutils_v3"
func sanitizeModulePath(modulePath string) string {
	r := strings.NewReplacer("/", "_", ".", "_")
	return r.Replace(modulePath)
}

// copyDirWritable copies a directory tree, ensuring all destination files are writable
// Source files may be read-only (GOMODCACHE uses 444 permissions)
func copyDirWritable(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		// Copy file — os.Create gives 0666 (writable), regardless of source permissions
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

// prepareExternalModules resolves, copies, injects, and adds replace directives
// for external modules specified in config.
//
// srcDir: original project directory (must contain go.mod with require for target modules)
// dstDir: destination directory (tmpDir for wrap mode, outputDir for inject mode)
//
// Returns results for post-processing (adding go-api dependency after go get).
func prepareExternalModules(cfg *config.Config, srcDir, dstDir string, errorTracking bool, debug bool) ([]ExternalModuleResult, error) {
	if !cfg.HasExternalModules() {
		return nil, nil
	}

	modulesDir := filepath.Join(dstDir, "_modules")
	var results []ExternalModuleResult

	// Expand wildcard patterns (e.g., "github.com/org/*") before processing
	modules := resolveExternalModuleList(cfg.GetExternalModules(), srcDir, debug)

	for _, modulePath := range modules {
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Resolving external module: %s\n", modulePath)
		}

		// 1. Resolve module location in GOMODCACHE
		info, err := resolveModule(modulePath, srcDir)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", modulePath, err)
		}

		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst]   %s@%s → %s\n", info.Path, info.Version, info.Dir)
		}

		// 2. Copy module from GOMODCACHE to dstDir/_modules/{name}/
		localDirName := sanitizeModulePath(modulePath)
		localDir := filepath.Join(modulesDir, localDirName)

		if err := copyDirWritable(info.Dir, localDir); err != nil {
			return nil, fmt.Errorf("copy module %s: %w", modulePath, err)
		}

		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst]   Copied to: %s\n", localDir)
		}

		// 3. Inject instrumentation code into copied module
		if err := injectDir(localDir, errorTracking, cfg); err != nil {
			return nil, fmt.Errorf("inject module %s: %w", modulePath, err)
		}

		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst]   Injected: %s\n", modulePath)
		}

		// 4. Add replace directive to dstDir/go.mod
		// go mod edit -replace=module@version=./local/path
		relModulePath, err := filepath.Rel(dstDir, localDir)
		if err != nil {
			relModulePath = localDir
		}
		relModulePath = filepath.ToSlash(relModulePath) // Unix-style for go.mod

		replaceArg := fmt.Sprintf("%s@%s=./%s", modulePath, info.Version, relModulePath)
		editCmd := exec.Command("go", "mod", "edit", "-replace="+replaceArg)
		editCmd.Dir = dstDir
		if out, err := editCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("add replace for %s: %s: %w", modulePath, string(out), err)
		}

		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst]   Added replace: %s@%s => ./%s\n",
				modulePath, info.Version, relModulePath)
		}

		results = append(results, ExternalModuleResult{
			ModulePath: modulePath,
			Version:    info.Version,
			LocalDir:   localDir,
		})
	}

	return results, nil
}

// finalizeExternalModules adds go-api dependency to each copied module's go.mod.
// Must be called AFTER go get github.com/whatap/go-api@latest on dstDir,
// so that the go-api version can be determined.
func finalizeExternalModules(results []ExternalModuleResult, dstDir string, debug bool) error {
	if len(results) == 0 {
		return nil
	}

	// Get go-api version from main module (already added by go get)
	goAPIVersion := getGoAPIVersion(dstDir)
	if goAPIVersion == "" {
		return fmt.Errorf("cannot determine go-api version from %s", dstDir)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] go-api version for external modules: %s\n", goAPIVersion)
	}

	requireArg := fmt.Sprintf("github.com/whatap/go-api@%s", goAPIVersion)

	for _, result := range results {
		editCmd := exec.Command("go", "mod", "edit", "-require="+requireArg)
		editCmd.Dir = result.LocalDir
		if out, err := editCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("add go-api require to %s: %s: %w",
				result.ModulePath, string(out), err)
		}

		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst]   Added go-api %s to: %s\n",
				goAPIVersion, result.ModulePath)
		}
	}

	return nil
}

// findModuleVersionInGoMod parses go.mod directly to find a module's version.
// This is a fallback when go list -m fails (e.g., private transitive deps).
func findModuleVersionInGoMod(modulePath, projectDir string) string {
	goModPath := filepath.Join(projectDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		// Match: modulePath vX.Y.Z or modulePath vX.Y.Z // indirect
		if strings.HasPrefix(line, modulePath+" ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

// resolveGoAPILatestVersion tries to determine the latest go-api version.
// First checks if go-api is already in the project's go.mod, then falls back to go list.
// Returns empty string if resolution fails.
func resolveGoAPILatestVersion(projectDir string, debug bool) string {
	// Try go list -m first (works if go-api is already a dependency)
	ver := getGoAPIVersion(projectDir)
	if ver != "" {
		return ver
	}

	// Try go list -m -versions to find the latest
	cmd := exec.Command("go", "list", "-m", "-versions", "github.com/whatap/go-api")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err == nil {
		line := strings.TrimSpace(string(out))
		// Format: "github.com/whatap/go-api v0.1.0 v0.2.0 v0.3.0"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			return parts[len(parts)-1] // last version is latest
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Could not resolve go-api version from %s\n", projectDir)
	}
	return ""
}

// expandWildcardModules expands wildcard pattern (e.g., "github.com/org/*")
// to matching module paths from go list -m all.
// Pattern must end with "/*" and have at least 3 depth (2 slashes).
func expandWildcardModules(pattern, projectDir string, debug bool) ([]string, error) {
	// 1. Validate: must end with /*
	if !strings.HasSuffix(pattern, "/*") {
		return nil, fmt.Errorf("invalid wildcard pattern: %s (must end with /*)", pattern)
	}

	// 2. Validate depth: prefix must have at least 2 slashes (3 depth)
	prefix := strings.TrimSuffix(pattern, "/*")
	if strings.Count(prefix, "/") < 1 {
		return nil, fmt.Errorf("wildcard pattern too broad: %s (need at least 3 depth, e.g., github.com/org/*)", pattern)
	}

	// 3. go list -m all (GOWORK=off to avoid workspace interference)
	cmd := exec.Command("go", "list", "-m", "all")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list -m all: %w", err)
	}

	// 4. Filter: prefix + "/" match (child modules) or exact match
	matchPrefix := prefix + "/"
	var matched []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		modPath := fields[0]
		if strings.HasPrefix(modPath, matchPrefix) || modPath == prefix {
			matched = append(matched, modPath)
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Wildcard %s → %d modules matched\n", pattern, len(matched))
		for _, m := range matched {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst]   - %s\n", m)
		}
	}
	return matched, nil
}

// resolveExternalModuleList expands wildcard patterns in module list.
func resolveExternalModuleList(patterns []string, projectDir string, debug bool) []string {
	var result []string
	for _, p := range patterns {
		if strings.Contains(p, "*") {
			// Normalize: "github.com/org*" → "github.com/org/*"
			normalized := p
			if !strings.HasSuffix(normalized, "/*") {
				normalized = strings.TrimSuffix(normalized, "*") + "/*"
				// Handle bare "*" → "/*"
			}
			expanded, err := expandWildcardModules(normalized, projectDir, debug)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: %v\n", err)
				continue
			}
			result = append(result, expanded...)
		} else {
			result = append(result, p)
		}
	}
	return result
}

// getGoAPIVersion reads the go-api version from go.mod in the given directory.
// Returns empty string if not found.
func getGoAPIVersion(dir string) string {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Version}}", "github.com/whatap/go-api")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
