package cmd

import (
	"encoding/json"
	"fmt"
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

// §234 step 10: prepareExternalModules / finalizeExternalModules / injectDir
// were wrap-mode exclusives. After wrap/inject/generate removal they are gone;
// persistExternalModulesForOutput (below) is the fast-mode replacement.

// persistExternalModulesForOutput finalises the _modules/ tree that toolexec
// populated under outputDir when --output is set in fast (toolexec) mode.
// §234 step 8.
//
// toolexec.saveInstrumentedFile has already placed instrumented .go files
// at <outputDir>/_modules/<sanitized>/<sub-pkg>/*.go via externalModuleOutputRel.
// This function completes the emitted tree so it is buildable standalone:
//
//  1. Copies the GOMODCACHE go.mod into <outputDir>/_modules/<sanitized>/go.mod
//     so the submodule has its own module metadata.
//  2. Injects `require github.com/whatap/go-api <version>` into that go.mod
//     (so the instrumented code can resolve whatap imports).
//  3. Adds `replace <mod> <ver> => ./_modules/<sanitized>` to the top-level
//     <outputDir>/go.mod so the main module picks up the local copy.
//
// `go mod tidy` is deliberately skipped — users can run it on the emitted
// tree themselves if they need go.sum to be re-normalised.
func persistExternalModulesForOutput(cfg *config.Config, projectDir, outputDir string, debug bool) error {
	if cfg == nil || !cfg.HasExternalModules() {
		return nil
	}

	modules := resolveExternalModuleList(cfg.GetExternalModules(), projectDir, debug)
	if len(modules) == 0 {
		return nil
	}

	// Prefer the outputDir's go.mod for the go-api version, but fall back to
	// the project's go.mod if the output copy is missing the require line.
	goAPIVersion := getGoAPIVersion(outputDir)
	if goAPIVersion == "" {
		goAPIVersion = getGoAPIVersion(projectDir)
	}

	for _, modulePath := range modules {
		info, err := resolveModule(modulePath, projectDir)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] §234 external-module %s resolve failed: %v\n", modulePath, err)
			}
			continue
		}

		localDirName := sanitizeModulePath(modulePath)
		localDir := filepath.Join(outputDir, "_modules", localDirName)

		// 1. Copy go.mod from GOMODCACHE into the output submodule.
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", localDir, err)
		}
		srcGoMod := filepath.Join(info.Dir, "go.mod")
		dstGoMod := filepath.Join(localDir, "go.mod")
		if data, rdErr := os.ReadFile(srcGoMod); rdErr == nil {
			if werr := os.WriteFile(dstGoMod, data, 0644); werr != nil {
				return fmt.Errorf("write %s: %w", dstGoMod, werr)
			}
		} else if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] §234 skip copy %s (missing go.mod: %v)\n", modulePath, rdErr)
		}

		// 2. Inject require github.com/whatap/go-api into the submodule go.mod.
		if goAPIVersion != "" {
			if _, err := os.Stat(dstGoMod); err == nil {
				requireArg := fmt.Sprintf("github.com/whatap/go-api@%s", goAPIVersion)
				editCmd := exec.Command("go", "mod", "edit", "-require="+requireArg)
				editCmd.Dir = localDir
				if out, err := editCmd.CombinedOutput(); err != nil && debug {
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] §234 add go-api require to %s: %s: %v\n", modulePath, out, err)
				}
			}
		}

		// 3. Add replace directive to <outputDir>/go.mod.
		relModulePath, err := filepath.Rel(outputDir, localDir)
		if err != nil {
			relModulePath = localDir
		}
		relModulePath = filepath.ToSlash(relModulePath)

		replaceArg := fmt.Sprintf("%s@%s=./%s", modulePath, info.Version, relModulePath)
		editCmd := exec.Command("go", "mod", "edit", "-replace="+replaceArg)
		editCmd.Dir = outputDir
		if out, err := editCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("add replace %s in %s: %s: %w", modulePath, outputDir, string(out), err)
		}

		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] §234: _modules/%s (replace %s@%s)\n",
				localDirName, modulePath, info.Version)
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
