package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/whatap/go-api-inst/ast"
	"github.com/whatap/go-api-inst/ast/common"
	"github.com/whatap/go-api-inst/report"

	"github.com/spf13/cobra"
)

var toolexecCmd = &cobra.Command{
	Use:   "toolexec",
	Short: "Inject code via compiler pipeline in toolexec mode",
	Long: `Use with go build -toolexec flag to inject monitoring code at build time.

Usage:
  go build -toolexec="whatap-go-inst toolexec" ./...
  go build -toolexec="whatap-go-inst toolexec" -o myapp .

This method injects instrumentation code into the build output without modifying the original source code.`,
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Debug: show all environment variables related to whatap (§72 debug)
		if os.Getenv("GO_API_AST_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] toolexec Run: GO_API_PROJECT_DIR from os.Getenv = %q\n", os.Getenv("GO_API_PROJECT_DIR"))
		}
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Error: toolexec mode must be used with go build -toolexec")
			os.Exit(1)
		}

		// First argument is the tool to execute (compile, asm, link, etc.)
		tool := args[0]
		toolArgs := args[1:]

		// Perform AST transformation only for compile tool
		if isCompileTool(tool) {
			toolArgs = processCompileArgs(toolArgs)
		} else if isLinkTool(tool) {
			toolArgs = processLinkArgs(toolArgs)
		}

		// Execute the original tool
		execTool(tool, toolArgs)
	},
}

// isCompileTool checks if the tool is a compile tool
func isCompileTool(tool string) bool {
	base := filepath.Base(tool)
	return base == "compile" || base == "compile.exe"
}

// isLinkTool checks if the tool is a link tool
func isLinkTool(tool string) bool {
	base := filepath.Base(tool)
	return base == "link" || base == "link.exe"
}

// processLinkArgs patches the linker's importcfg with all resolved whatap packages.
// The linker needs to know about packages that were invisibly added by toolexec
// during compilation (the Go build system doesn't know about them).
func processLinkArgs(args []string) []string {
	debug := os.Getenv("GO_API_AST_DEBUG") != ""

	cachePath := os.Getenv("GO_API_RESOLVE_CACHE")
	if cachePath == "" {
		return args
	}

	resolveCache, err := readCacheFile(cachePath)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] link: failed to read resolve cache: %v\n", err)
		}
		return args
	}

	importCfgPath := parseImportCfgPath(args)
	if importCfgPath == "" {
		return args
	}

	// Add all resolved packages to the linker importcfg.
	// appendToImportCfg skips duplicates, so this is safe.
	if len(resolveCache.Packages) > 0 {
		if err := appendToImportCfg(importCfgPath, resolveCache.Packages); err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] link: importcfg append failed: %v\n", err)
			}
			return args
		}
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] link: added whatap packages to linker importcfg\n")
		}
	}

	return args
}

// processCompileArgs processes compile arguments and transforms source
func processCompileArgs(args []string) []string {
	debug := os.Getenv("GO_API_AST_DEBUG") != ""

	var goFiles []string
	var otherArgs []string
	var outputFile string

	// Parse arguments
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "-o" && i+1 < len(args) {
			outputFile = args[i+1]
			otherArgs = append(otherArgs, arg, args[i+1])
			i++
			continue
		}

		if strings.HasSuffix(arg, ".go") {
			// §191: Validate that the argument is an actual .go file, not a module path
			// ending with ".go" (e.g., github.com/nats-io/nats.go is a module, not a file).
			if info, err := os.Stat(arg); err == nil && !info.IsDir() {
				goFiles = append(goFiles, arg)
			} else {
				otherArgs = append(otherArgs, arg)
			}
		} else {
			otherArgs = append(otherArgs, arg)
		}
	}

	// Return as-is if no Go files
	if len(goFiles) == 0 {
		return args
	}

	// Parse -importcfg path and load resolve cache for importcfg modification
	importCfgPath := parseImportCfgPath(args)
	var resolveCache *ResolveCache
	cachePath := os.Getenv("GO_API_RESOLVE_CACHE")
	if cachePath != "" {
		var err error
		resolveCache, err = readCacheFile(cachePath)
		if err != nil && debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: failed to read resolve cache: %v\n", err)
		}
	}

	// §234: Use the compiler's $WORK directory (derived from -o path) instead of
	// our own tmpDir. This avoids tmp accumulation (no RemoveAll was in place),
	// lets `go build -work` expose the instrumented sources for diff, and aligns
	// with orchestrion's approach. outputFile is always present for toolexec
	// compile invocations (`-o $WORK/bXXX/_pkg_.a`); if it is missing we bail
	// out rather than invent a fallback path (safety first).
	if outputFile == "" {
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] no -o in args; skipping transformation\n")
		}
		return args
	}
	workDir := filepath.Dir(outputFile)

	// Directory to save transformed files (specified via environment variable)
	saveDir := os.Getenv("GO_API_AST_OUTPUT_DIR")

	// Transformed files
	injector := ast.NewInjector()
	// §208: Pass config to injector for custom rules and preset filtering.
	// §227 Step 5: SetConfig() rebuilds the registry so user-defined rules
	// from cfg.Rules get registered alongside built-ins. A bare assignment
	// would leave the registry frozen at NewInjector() time (built-ins only).
	if globalConfig != nil {
		injector.EnabledPackages = globalConfig.Instrumentation.EnabledPackages
		injector.DisabledPackages = globalConfig.Instrumentation.DisabledPackages
		injector.SetConfig(globalConfig)
	}
	// §205: Pass replaced modules to injector so transformers skip replaced modules
	if resolveCache != nil && len(resolveCache.ReplacedModules) > 0 {
		injector.ReplacedModules = resolveCache.ReplacedModules
	}
	var transformedFiles []string

	// Get project root for relative path calculation
	projectRoot := os.Getenv("GO_API_PROJECT_DIR")
	if projectRoot == "" {
		projectRoot, _ = os.Getwd()
	}

	// §234: go.mod/go.sum symlinks removed. They were placed into the old
	// tmpDir for module resolution that packages.Load once required. Fast
	// mode switched to importer.ForCompiler in §174, which reads .a archives
	// from importcfg and does not need go.mod at all.

	// Collect new whatap imports across all transformed files for importcfg modification.
	allNewWhatapImports := make(map[string]struct{})

	// Parse external-module list for GOMODCACHE filtering (§174).
	// Only project source and external-module packages are transformed.
	// Other GOMODCACHE packages (gin, protobuf, etc.) pass through unchanged
	// to preserve fingerprints and avoid pre-resolve mismatch.
	externalModules := parseExternalModules(os.Getenv("GO_API_EXTERNAL_MODULES"))

	// §179: Use TOOLEXEC_IMPORTPATH for accurate package identification.
	// File path matching cannot distinguish major versions (e.g., /v8 vs /v9).
	importPath := os.Getenv("TOOLEXEC_IMPORTPATH")

	// §174: Pre-load type info from importcfg for all transformable goFiles.
	// Uses importer.ForCompiler (reads .a archives) instead of packages.Load (triggers go list → panic).
	// Only load for files that will actually be transformed (skip GOROOT, skip non-external GOMODCACHE).
	if importCfgPath != "" {
		var transformableFiles []string
		for _, goFile := range goFiles {
			if shouldSkipFile(goFile) {
				continue
			}
			if isInGOMODCACHE(goFile) && !matchesExternalModule(importPath, externalModules) {
				continue
			}
			transformableFiles = append(transformableFiles, goFile)
		}
		if len(transformableFiles) > 0 {
			importcfgEntries, cfgErr := readImportCfg(importCfgPath)
			if cfgErr == nil {
				if err := common.SetupImportcfgTypeInfo(importcfgEntries, transformableFiles, importPath, debug); err != nil {
					if debug {
						fmt.Fprintf(os.Stderr, "[whatap-go-inst] importcfg typecheck setup failed (non-fatal): %v\n", err)
					}
				}
				defer common.ClearImportcfgTypeCache()
			}
		}
	}

	for _, goFile := range goFiles {
		// Skip standard library and toolchain (never save — not part of user project)
		if shouldSkipFile(goFile) {
			transformedFiles = append(transformedFiles, goFile)
			continue
		}

		// §174: GOMODCACHE files pass through unchanged unless registered as external-module.
		// This prevents transformer from modifying third-party libraries (e.g., fmt transformer
		// wrapping fmt.Println inside gin's recovery.go), which would change their fingerprint
		// and cause linker mismatch with pre-resolved whatap packages.
		// §188: vendor/ files are treated the same as GOMODCACHE — external packages.
		if (isInGOMODCACHE(goFile) || isInVendor(goFile)) && !matchesExternalModule(importPath, externalModules) {
			transformedFiles = append(transformedFiles, goFile)
			// §234 step 5/7: copy vendor originals into saveDir so --output
			// produces a buildable source tree. GOMODCACHE (non-external)
			// stays out — the module cache is outside the project.
			if saveDir != "" && isInVendor(goFile) {
				saveInstrumentedFile(goFile, goFile, saveDir)
			}
			continue
		}

		// Capture imports before transformation (for importcfg diff).
		var importsBefore []string
		if importCfgPath != "" && resolveCache != nil {
			importsBefore, _ = extractImportsFromGoFile(goFile)
		}

		// §234: Place transformed files under $WORK/bXXX/whatap/src/<importPath>/
		// (orchestrion layout). The compiler picks them up via the replaced args
		// slice; Go reclaims $WORK automatically unless `-work` is passed.
		// `//line` directives from InjectFile preserve the original path so
		// diagnostics still point at the user's source.
		//
		// §83 (go:embed) is naturally preserved because the embedcfg file holds
		// absolute paths to resource files — compilation reads resources from
		// the user's project directly, so we no longer need to duplicate embed
		// assets next to the transformed source.
		dstDir := filepath.Join(workDir, "whatap", "src", importPath)
		tmpFile := filepath.Join(dstDir, filepath.Base(goFile))
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			transformedFiles = append(transformedFiles, goFile)
			if saveDir != "" {
				saveInstrumentedFile(goFile, goFile, saveDir)
			}
			continue
		}

		// Attempt AST transformation (recover from panics in packages.Load etc.)
		var injectErr error
		var injectPanicked bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					injectErr = fmt.Errorf("panic: %v", r)
					injectPanicked = true
					if debug {
						fmt.Fprintf(os.Stderr, "[whatap-go-inst] recovered panic in InjectFile(%s): %v\n", goFile, r)
					}
				}
			}()
			injectErr = injector.InjectFile(goFile, tmpFile)
		}()
		if injectErr != nil {
			// Use original on transformation failure
			transformedFiles = append(transformedFiles, goFile)
			// §239: surface injection failures in --report. InjectFile itself
			// already records StatusError for its internal error paths (read,
			// parse). Panics never reach its AddFile call, so record here —
			// guarded by injectPanicked to avoid double-counting.
			if injectPanicked {
				report.Get().AddFile(report.FileReport{
					Path:   goFile,
					Status: report.StatusError,
					Error:  injectErr.Error(),
					Reason: "copied as-is",
				})
			}
			// §234 step 5: ensure --output still contains this file as the
			// original source so the emitted tree compiles end-to-end even
			// when individual files fail injection.
			if saveDir != "" {
				saveInstrumentedFile(goFile, goFile, saveDir)
			}
			continue
		}

		transformedFiles = append(transformedFiles, tmpFile)

		if debug {
			fmt.Fprintf(os.Stderr, "✅ %s\n", goFile)
		}

		// Detect new imports added by transformation.
		if importCfgPath != "" && resolveCache != nil {
			importsAfter, _ := extractImportsFromGoFile(tmpFile)
			for _, imp := range findNewImports(importsBefore, importsAfter) {
				allNewWhatapImports[imp] = struct{}{}
			}
		}

		// Save transformed file (if saveDir is set)
		if saveDir != "" {
			saveInstrumentedFile(goFile, tmpFile, saveDir)
		}
	}

	// Append new whatap package entries to importcfg so the compiler can find them.
	if importCfgPath != "" && resolveCache != nil && len(allNewWhatapImports) > 0 {
		newEntries := make(map[string]string)
		for imp := range allNewWhatapImports {
			archivePath := resolveCache.lookupArchive(imp)
			if archivePath != "" {
				newEntries[imp] = archivePath
			} else if debug {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: no archive for %s\n", imp)
			}
		}
		if len(newEntries) > 0 {
			if debug {
				for imp, arch := range newEntries {
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] importcfg: %s -> %s\n", imp, arch)
				}
			}
			if err := appendToImportCfg(importCfgPath, newEntries); err != nil {
				// Fallback: don't break the build, just warn.
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: importcfg append failed: %v\n", err)
				// §240: surface the failure in --report so remote diagnosis
				// can see when linker-visible packages went missing.
				report.Get().IncImportCfgFail()
			} else if debug {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] Added %d entries to importcfg\n", len(newEntries))
			}
		}
	}

	// Debug output
	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] output: %s\n", outputFile)
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] transformed: %v\n", transformedFiles)
	}

	// §239: Persist this child's per-file report as a fragment JSON so the
	// parent can aggregate it. Only active when --report is set (parent creates
	// the fragment dir and passes it via GO_API_REPORT_FRAG_DIR). No fragment
	// is written when there were no file-level records (e.g. all goFiles were
	// GOROOT/GOMODCACHE-passthroughs).
	if fragDir := os.Getenv("GO_API_REPORT_FRAG_DIR"); fragDir != "" {
		r := report.Get()
		if r != nil && len(r.Files) > 0 {
			safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(importPath)
			if safe == "" {
				safe = "unknown"
			}
			fragPath := filepath.Join(fragDir,
				fmt.Sprintf("%s-%d-%d.json", safe, os.Getpid(), time.Now().UnixNano()))
			if err := r.SaveJSONQuiet(fragPath); err != nil && debug {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] §239: save fragment %s failed: %v\n", fragPath, err)
			}
		}
	}

	// Construct new argument list
	newArgs := append(otherArgs, transformedFiles...)
	return newArgs
}

// saveInstrumentedFile copies transformed file to save directory
func saveInstrumentedFile(originalPath, transformedPath, saveDir string) {
	// Calculate relative path from project root
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	relPath, err := filepath.Rel(cwd, originalPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		// File outside project — either GOMODCACHE (external module) or
		// some other out-of-tree source. Use TOOLEXEC_IMPORTPATH to preserve
		// package directory structure.
		//
		// §234 step 8: For external-module matches (GO_API_EXTERNAL_MODULES),
		// rewrite the save path to `_modules/<sanitized-module>/<sub-pkg>/...`
		// so that the emitted tree pairs with a `replace` directive in the
		// output go.mod. Non-matching GOMODCACHE files fall back to the
		// importpath layout (legacy behaviour).
		importPath := os.Getenv("TOOLEXEC_IMPORTPATH")
		modules := parseExternalModules(os.Getenv("GO_API_EXTERNAL_MODULES"))
		if ext := externalModuleOutputRel(importPath, filepath.Base(originalPath), modules); ext != "" {
			relPath = ext
		} else if importPath != "" {
			relPath = filepath.Join(importPath, filepath.Base(originalPath))
		} else {
			relPath = filepath.Base(originalPath)
		}
	}

	// Save path
	savePath := filepath.Join(saveDir, relPath)

	// Create directory
	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		return
	}

	// Copy file
	data, err := os.ReadFile(transformedPath)
	if err != nil {
		return
	}

	os.WriteFile(savePath, data, 0644)
}

// externalModuleOutputRel returns the path under `_modules/<sanitized>/...`
// for an importPath that matches one of the configured external modules.
// Returns "" when no external-module prefix matches.
// §234 step 8: pairs with the `replace <mod> => ./_modules/<sanitized>` entry
// that go_fast.go writes into the output go.mod.
func externalModuleOutputRel(importPath, basename string, modules []string) string {
	if importPath == "" || len(modules) == 0 {
		return ""
	}
	for _, mod := range modules {
		if importPath == mod {
			return filepath.Join("_modules", sanitizeModulePath(mod), basename)
		}
		if strings.HasPrefix(importPath, mod+"/") {
			subPath := strings.TrimPrefix(importPath, mod+"/")
			return filepath.Join("_modules", sanitizeModulePath(mod), subPath, basename)
		}
	}
	return ""
}

// shouldSkipFile checks if the file should be skipped for transformation.
// In fast (toolexec) mode, GOMODCACHE is NOT skipped — external packages are also instrumented.
// Only GOROOT (standard library) is skipped. Duplicate transaction prevention is handled at runtime (§175).
func shouldSkipFile(path string) bool {
	cleanPath := filepath.Clean(path)

	// GOROOT is always skipped (standard library)
	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		goroot = runtime.GOROOT()
	}
	if goroot != "" {
		if strings.HasPrefix(cleanPath, filepath.Clean(goroot)) {
			return true
		}
	}

	// Skip Go toolchain downloaded into GOMODCACHE (e.g., golang.org/toolchain@v0.0.1-go1.25.5.windows-amd64/src/).
	// When a project uses a newer Go version than the system Go, the toolchain (including stdlib)
	// is downloaded into GOMODCACHE. These files must NOT be instrumented.
	slashPath := filepath.ToSlash(cleanPath)
	if strings.Contains(slashPath, "golang.org/toolchain@") {
		return true
	}

	// Check exclude patterns (without system path check — GOMODCACHE is allowed)
	// §188: In vendor mode, remove "vendor/**" from exclude patterns so vendor/ files are not skipped.
	// Vendor files are then filtered by isInVendor() + matchesExternalModule() in processCompileArgs.
	basePath, _ := os.Getwd()
	var excludePatterns []string
	if os.Getenv("GO_API_VENDOR_MODE") == "true" {
		excludePatterns = excludePatternsWithoutVendor()
	}
	skip := common.ShouldSkipFileExcludeOnly(path, basePath, excludePatterns)

	debug := os.Getenv("GO_API_AST_DEBUG") != ""
	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] shouldSkipFile: path=%q basePath=%q skip=%v\n", path, basePath, skip)
	}
	return skip
}

// parseExternalModules parses comma-separated module paths from GO_API_EXTERNAL_MODULES
// and expands wildcard patterns using resolveExternalModuleList (shared with wrap mode).
func parseExternalModules(envVal string) []string {
	if envVal == "" {
		return nil
	}
	var patterns []string
	for _, m := range strings.Split(envVal, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			patterns = append(patterns, m)
		}
	}
	projectDir := os.Getenv("GO_API_PROJECT_DIR")
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	debug := os.Getenv("GO_API_AST_DEBUG") != ""
	return resolveExternalModuleList(patterns, projectDir, debug)
}

// §188: isInVendor checks if the file path is inside the project's vendor/ directory.
func isInVendor(path string) bool {
	projectDir := os.Getenv("GO_API_PROJECT_DIR")
	if projectDir == "" {
		return false
	}
	vendorDir := filepath.Join(projectDir, "vendor")
	cleanPath := filepath.Clean(path)
	return strings.HasPrefix(cleanPath, filepath.Clean(vendorDir)+string(filepath.Separator))
}

// isInGOMODCACHE checks if the file path is inside GOMODCACHE.
func isInGOMODCACHE(path string) bool {
	gomodcache := os.Getenv("GOMODCACHE")
	if gomodcache == "" {
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			home, _ := os.UserHomeDir()
			gopath = filepath.Join(home, "go")
		}
		gomodcache = filepath.Join(gopath, "pkg", "mod")
	}
	cleanPath := filepath.Clean(path)
	return strings.HasPrefix(cleanPath, filepath.Clean(gomodcache))
}

// matchesExternalModule checks if the import path belongs to one of the external modules.
// Uses TOOLEXEC_IMPORTPATH (Go module import path) for exact matching,
// which correctly distinguishes major versions (e.g., /v8 vs /v9).
// Wildcards are already expanded by parseExternalModules → resolveExternalModuleList.
//
// Example: importPath="github.com/myorg/mylib", module="github.com/myorg/mylib" → match
// Example: importPath="github.com/myorg/mylib/sub", module="github.com/myorg/mylib" → match (sub-package)
// Example: importPath="github.com/redis/go-redis/v8", module="github.com/redis/go-redis/v9" → no match
func matchesExternalModule(importPath string, modules []string) bool {
	if importPath == "" || len(modules) == 0 {
		return false
	}
	for _, mod := range modules {
		if importPath == mod || strings.HasPrefix(importPath, mod+"/") {
			return true
		}
	}
	return false
}

// execTool executes the tool
func execTool(tool string, args []string) {
	cmd := exec.Command(tool, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

// §234: copyEmbedResources / copyDirForEmbed removed. The previous
// implementation duplicated //go:embed resources alongside transformed files
// in a private tmpDir. Since transformed files now live in $WORK/.../whatap/src
// and embedcfg (generated by `go build`) references resource paths as
// absolute paths into the user's project, no copy is needed.

func init() {
	rootCmd.AddCommand(toolexecCmd)
}
