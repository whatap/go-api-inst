package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/whatap/go-api-inst/ast"
	"github.com/whatap/go-api-inst/ast/common"
	"github.com/whatap/go-api-inst/config"

	"github.com/spf13/cobra"
)

var goCmd = &cobra.Command{
	Use:   "go",
	Short: "Wrap go commands with automatic instrumentation code injection",
	Long: `Wraps go commands to automatically inject monitoring code during build.

Usage:
  whatap-go-inst go build ./...
  whatap-go-inst go build -o myapp .
  whatap-go-inst go run .
  whatap-go-inst go test ./...
  whatap-go-inst go --wrap build ./...
  whatap-go-inst go --output ./instrumented build ./...

Prerequisites (fast mode):
  whatap-go-inst init  # Run once

Options:
  --error-tracking    Enable error tracking code injection (adds trace.Error to if err != nil patterns)
  --config <path>     Specify config file path
  --wrap              Wrap mode (no init required, preserves 100% original)
  --output, -O <dir>  Path to save instrumented source

Internal behavior (default fast mode):
  1. Include tool.go with -tags whatap_tools
  2. Inject instrumentation code at compile time via toolexec
  * Prerequisite: tool.go must be created via init

Internal behavior (--wrap mode):
  1. Copy source to temporary directory
  2. Inject instrumentation code (inject)
  3. Run go get github.com/whatap/go-api@latest
  4. Run go mod tidy
  5. Execute build
  6. Save transformed source to whatap-instrumented/`,
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Parse flags (--error-tracking, --config, --wrap, --output)
		errorTrackingCLI := false
		errorTrackingSet := false
		wrapMode := false
		var goConfigPath string
		var outputDir string
		var filteredArgs []string

		for i := 0; i < len(args); i++ {
			arg := args[i]
			if arg == "--error-tracking" {
				errorTrackingCLI = true
				errorTrackingSet = true
			} else if arg == "--wrap" {
				wrapMode = true
			} else if arg == "--config" && i+1 < len(args) {
				goConfigPath = args[i+1]
				i++ // Skip next argument
			} else if strings.HasPrefix(arg, "--config=") {
				goConfigPath = strings.TrimPrefix(arg, "--config=")
			} else if arg == "--output" && i+1 < len(args) {
				outputDir = args[i+1]
				i++ // Skip next argument
			} else if strings.HasPrefix(arg, "--output=") {
				outputDir = strings.TrimPrefix(arg, "--output=")
			} else if arg == "-O" && i+1 < len(args) {
				// Short form: -O <dir>
				outputDir = args[i+1]
				i++
			} else {
				filteredArgs = append(filteredArgs, arg)
			}
		}
		args = filteredArgs

		// Load config
		loader := config.NewLoader()
		loader.ConfigPath = goConfigPath
		cfg, err := loader.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config file: %v\n", err)
			os.Exit(1)
		}

		// ErrorTracking: Use CLI flag if explicitly set, otherwise use config file value
		errorTracking := cfg.Instrumentation.ErrorTracking
		if errorTrackingSet {
			errorTracking = errorTrackingCLI
		}

		// Debug mode output
		if cfg.Instrumentation.Debug {
			if path := loader.GetConfigPath(); path != "" {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] Config file: %s\n", path)
			}
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] ErrorTracking: %v\n", errorTracking)
		}

		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Error: Please specify a go command")
			fmt.Fprintln(os.Stderr, "Example: whatap-go-inst go build ./...")
			os.Exit(1)
		}

		goSubCmd := args[0]
		goArgs := args[1:]

		// Apply inject only to build, run, test, install commands
		if shouldApplyInject(goSubCmd) {
			// Find go.mod location from build target
			projectDir := findProjectDirFromArgs(goArgs)
			if projectDir != "" {
				loader.ProjectDir = projectDir
				cfg, err = loader.Load()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to load config file: %v\n", err)
					os.Exit(1)
				}
				// Reapply ErrorTracking
				if errorTrackingSet {
					cfg.Instrumentation.ErrorTracking = errorTrackingCLI
				}
				errorTracking = cfg.Instrumentation.ErrorTracking

				if cfg.Instrumentation.Debug {
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] ProjectDir: %s\n", projectDir)
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] BaseDir: %s\n", cfg.BaseDir)
				}
			}

			// Apply CLI --output flag to config
			if outputDir != "" {
				cfg.Instrumentation.OutputDir = outputDir
			}

			if wrapMode {
				// --wrap flag: wrap mode (temporary dependencies)
				runWithInjectWithConfig(goSubCmd, goArgs, cfg, errorTracking)
			} else {
				// Default: fast mode (toolexec)
				// Show message and exit if tool.go doesn't exist
				if !hasToolFile(projectDir) {
					printInitRequiredMessage()
					os.Exit(1)
				}
				runFastBuild(goSubCmd, goArgs, cfg, errorTracking)
			}
		} else {
			// Pass other commands through as-is
			runGoCommand(goSubCmd, goArgs)
		}
	},
}

// findProjectDirFromArgs finds project directory from build arguments
// Searches for go.mod location from build target file/directory
func findProjectDirFromArgs(args []string) string {
	// Find build target from args (non-flag items)
	for _, arg := range args {
		// Skip flags
		if strings.HasPrefix(arg, "-") {
			continue
		}

		// Handle ./... format
		target := arg
		if strings.HasSuffix(target, "/...") {
			target = strings.TrimSuffix(target, "/...")
		}
		if target == "." || target == "./" {
			// Based on cwd
			cwd, _ := os.Getwd()
			return config.FindGoModDir(cwd)
		}

		// Handle absolute/relative paths
		absPath, err := filepath.Abs(target)
		if err != nil {
			continue
		}

		// Check if file or directory
		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			return config.FindGoModDir(absPath)
		} else {
			// If file, search from parent directory
			return config.FindGoModDir(filepath.Dir(absPath))
		}
	}

	// If target not found, use cwd
	cwd, _ := os.Getwd()
	return config.FindGoModDir(cwd)
}

// shouldApplyInject checks if inject should be applied to this command
func shouldApplyInject(subCmd string) bool {
	switch subCmd {
	case "build", "run", "test", "install":
		return true
	default:
		return false
	}
}

// convertBuildTargets converts build target paths relative to tmpDir
// Since we copied based on srcDir, convert all paths to relative paths based on tmpDir
// Example: testapps/custom-app/... → ./...
//
//	testapps/custom-app/main.go → ./main.go
func convertBuildTargets(args []string, srcDir, cwd string) []string {
	absSrcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return args
	}

	result := make([]string, len(args))
	for i, arg := range args {
		// Keep flags as-is
		if strings.HasPrefix(arg, "-") {
			result[i] = arg
			continue
		}

		// Handle /... suffix
		suffix := ""
		target := arg
		if strings.HasSuffix(target, "/...") {
			suffix = "/..."
			target = strings.TrimSuffix(target, "/...")
		}

		// Keep . or ./ as-is
		if target == "." || target == "./" {
			result[i] = "./" + suffix
			continue
		}

		// Handle absolute/relative paths
		absTarget, err := filepath.Abs(target)
		if err != nil {
			result[i] = arg
			continue
		}

		// Check if path is inside srcDir
		relPath, err := filepath.Rel(absSrcDir, absTarget)
		if err != nil || strings.HasPrefix(relPath, "..") {
			// If outside srcDir, keep as-is (external package)
			result[i] = arg
			continue
		}

		// Convert to relative path based on srcDir
		// If relPath is ".", use "./", otherwise "./" + relPath
		if relPath == "." {
			if suffix != "" {
				result[i] = "." + suffix // ./... (suffix already has /)
			} else {
				result[i] = "./"
			}
		} else {
			result[i] = "./" + filepath.ToSlash(relPath) + suffix
		}
	}
	return result
}

// runWithInjectWithConfig builds using Config
func runWithInjectWithConfig(subCmd string, args []string, cfg *config.Config, errorTracking bool) {
	debug := cfg.Instrumentation.Debug
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot find current directory: %v\n", err)
		os.Exit(1)
	}

	// Instrumented source output directory (config > env var > default)
	instrumentedOutputDir := cfg.Instrumentation.OutputDir
	if instrumentedOutputDir == "" {
		instrumentedOutputDir = os.Getenv("GO_API_AST_OUTPUT_DIR")
	}
	if instrumentedOutputDir == "" {
		instrumentedOutputDir = filepath.Join(cwd, "whatap-instrumented")
	}

	// 1. Create temporary directory
	tmpDir, err := os.MkdirTemp("", "whatap-go-inst-build-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temporary directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Temporary directory: %s\n", tmpDir)
	}

	// 2. Copy source (based on BaseDir)
	srcDir := cfg.BaseDir
	if srcDir == "" {
		srcDir = cwd
	}
	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Copying source... (from: %s)\n", srcDir)
	}
	if err := copySourceFiles(srcDir, tmpDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to copy source: %v\n", err)
		os.Exit(1)
	}

	// 3. Run inject
	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Injecting instrumentation code...\n")
	}
	if err := injectDir(tmpDir, errorTracking, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to inject instrumentation code: %v\n", err)
		os.Exit(1)
	}

	// 4. Install go-api latest version and run go mod tidy
	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Running go get github.com/whatap/go-api@latest...\n")
	}
	getCmd := exec.Command("go", "get", "github.com/whatap/go-api@latest")
	getCmd.Dir = tmpDir
	if debug {
		getCmd.Stdout = os.Stderr
		getCmd.Stderr = os.Stderr
	}
	getCmd.Run() // Ignore error (go.mod might not exist)

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Running go mod tidy...\n")
	}
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if debug {
		tidyCmd.Stdout = os.Stderr
		tidyCmd.Stderr = os.Stderr
	}
	tidyCmd.Run() // Ignore error (go.mod might not exist)

	// 5. Parse and convert output file path
	outputFile, newArgs := parseOutputFlag(args)
	if outputFile != "" && !filepath.IsAbs(outputFile) {
		// Convert relative path to absolute path based on original directory
		outputFile = filepath.Join(cwd, outputFile)
	}

	// 5.5. Convert build target paths (srcDir-based → tmpDir-based)
	// testapps/custom-app/... → ./...
	newArgs = convertBuildTargets(newArgs, srcDir, cwd)

	// 6. Execute build
	var buildArgs []string
	buildArgs = append(buildArgs, subCmd)
	if outputFile != "" {
		buildArgs = append(buildArgs, "-o", outputFile)
	}
	buildArgs = append(buildArgs, newArgs...)

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] go %s\n", strings.Join(buildArgs, " "))
	}

	buildCmd := exec.Command("go", buildArgs...)
	buildCmd.Dir = tmpDir
	buildCmd.Stdin = os.Stdin
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

	// 7. Copy instrumented source (always runs, default: whatap-instrumented/)
	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Copying instrumented source: %s\n", instrumentedOutputDir)
	}
	if err := copyInstrumentedSource(tmpDir, instrumentedOutputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to copy instrumented source: %v\n", err)
	}
}

// printInitRequiredMessage prints init required message
func printInitRequiredMessage() {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "❌ whatap_inst.tool.go not found.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Please run init first (in the go.mod directory):")
	fmt.Fprintln(os.Stderr, "  whatap-go-inst init")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Or build with wrap mode:")
	fmt.Fprintln(os.Stderr, "  whatap-go-inst go --wrap build ./...")
	fmt.Fprintln(os.Stderr, "")
}

// hasToolFile checks if whatap_inst.tool.go file exists
func hasToolFile(projectDir string) bool {
	toolFilePath := filepath.Join(projectDir, "whatap_inst.tool.go")
	_, err := os.Stat(toolFilePath)
	return err == nil
}

// runFastBuild fast build mode (using toolexec)
func runFastBuild(subCmd string, args []string, cfg *config.Config, errorTracking bool) {
	debug := cfg.Instrumentation.Debug

	// Project directory (cfg.BaseDir or cwd)
	projectDir := cfg.BaseDir
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	// Convert to absolute path for toolexec (§72 fix)
	if !filepath.IsAbs(projectDir) {
		absProjectDir, err := filepath.Abs(projectDir)
		if err == nil {
			projectDir = absProjectDir
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Build mode: fast (toolexec)\n")
	}

	// 1. Find current executable path
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot find executable path: %v\n", err)
		os.Exit(1)
	}

	// 2. Configure toolexec flag
	toolexecFlag := fmt.Sprintf("-toolexec=%s toolexec", execPath)
	if errorTracking {
		toolexecFlag = fmt.Sprintf("-toolexec=%s toolexec --error-tracking", execPath)
	}

	// 3. Convert build target paths (when running from outside)
	cwd, _ := os.Getwd()
	convertedArgs := convertBuildTargets(args, projectDir, cwd)

	// 4. Set instrumented source output directory
	instrumentedOutputDir := cfg.Instrumentation.OutputDir
	if instrumentedOutputDir == "" {
		instrumentedOutputDir = os.Getenv("GO_API_AST_OUTPUT_DIR")
	}
	if instrumentedOutputDir == "" {
		instrumentedOutputDir = filepath.Join(cwd, "whatap-instrumented")
	}
	// Convert to absolute path
	if !filepath.IsAbs(instrumentedOutputDir) {
		instrumentedOutputDir = filepath.Join(cwd, instrumentedOutputDir)
	}

	// 5. Configure go command
	var buildArgs []string
	buildArgs = append(buildArgs, subCmd)
	buildArgs = append(buildArgs, "-tags", "whatap_tools") // Include tool.go created by init
	buildArgs = append(buildArgs, toolexecFlag)
	buildArgs = append(buildArgs, convertedArgs...)

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] ProjectDir: %s\n", projectDir)
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] InstrumentedOutput: %s\n", instrumentedOutputDir)
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] go %s\n", strings.Join(buildArgs, " "))
	}

	// 6. Execute go command (in project directory)
	// Pass save path to toolexec via GO_API_AST_OUTPUT_DIR environment variable
	// Pass project directory to toolexec via GO_API_PROJECT_DIR environment variable (§72 fix)
	buildCmd := exec.Command("go", buildArgs...)
	buildCmd.Dir = projectDir
	// Build environment with our variables, filtering out any existing duplicates
	env := os.Environ()
	env = filterEnvVar(env, "GO_API_AST_OUTPUT_DIR")
	env = filterEnvVar(env, "GO_API_PROJECT_DIR")
	env = append(env,
		"GO_API_AST_OUTPUT_DIR="+instrumentedOutputDir,
		"GO_API_PROJECT_DIR="+projectDir,
	)
	buildCmd.Env = env
	buildCmd.Stdin = os.Stdin
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	buildErr := buildCmd.Run()

	// Copy go.mod, go.sum regardless of build success/failure
	copyProjectFiles(projectDir, instrumentedOutputDir)
	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] go.mod, go.sum copied: %s\n", instrumentedOutputDir)
	}

	if buildErr != nil {
		if exitErr, ok := buildErr.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

// copyProjectFiles copies project files like go.mod, go.sum
func copyProjectFiles(srcDir, dstDir string) {
	files := []string{"go.mod", "go.sum"}
	for _, file := range files {
		srcPath := filepath.Join(srcDir, file)
		dstPath := filepath.Join(dstDir, file)

		data, err := os.ReadFile(srcPath)
		if err != nil {
			continue // Skip if file doesn't exist
		}

		os.MkdirAll(dstDir, 0755)
		os.WriteFile(dstPath, data, 0644)
	}
}

// copyInstrumentedSource copies instrumented source to output directory
func copyInstrumentedSource(src, dst string) error {
	// Remove existing directory and recreate
	os.RemoveAll(dst)
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		// Skip directories (build artifacts, previous instrumentation results)
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "instrumented" || base == "whatap-instrumented" || base == "output" || base == "cleaned" {
				return filepath.SkipDir
			}
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy only .go, go.mod, go.sum files
		ext := filepath.Ext(path)
		base := filepath.Base(path)
		if ext == ".go" || base == "go.mod" || base == "go.sum" {
			return copyFile(path, dstPath)
		}

		return nil
	})
}

// copySourceFiles copies source files to temporary directory
func copySourceFiles(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Ignore error and continue
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		// Skip directories
		if info.IsDir() {
			base := filepath.Base(path)
			// Skip build artifacts, instrumentation results, IDE settings, etc.
			if base == ".git" || base == "node_modules" || base == ".idea" || base == ".vscode" ||
				base == "instrumented" || base == "whatap-instrumented" || base == "output" || base == "cleaned" {
				return filepath.SkipDir
			}
			// Copy vendor (include dependencies)
			return os.MkdirAll(dstPath, info.Mode())
		}

		// File extensions to copy
		ext := filepath.Ext(path)
		base := filepath.Base(path)

		// go.mod needs replace relative path adjustment
		if base == "go.mod" {
			return copyGoMod(path, dstPath, src, dst)
		}

		// Copy go.sum, *.go files
		if base == "go.sum" || ext == ".go" {
			return copyFile(path, dstPath)
		}

		return nil
	})
}

// copyFile copies a file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create directory
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// copyGoMod copies go.mod file while adjusting replace relative paths
func copyGoMod(src, dst, srcDir, dstDir string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create directory
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// replace statement pattern: replace module => path or replace module path
	// Relative paths start with ./ or ../
	replacePattern := regexp.MustCompile(`^(\s*replace\s+\S+\s+=>\s+)(\.\.?/.*)$`)
	replaceBlockPattern := regexp.MustCompile(`^(\s*)(\.\.?/\S+)(\s+.*)$`)

	scanner := bufio.NewScanner(srcFile)
	writer := bufio.NewWriter(dstFile)
	defer writer.Flush()

	inReplaceBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		// replace ( block start
		if strings.HasPrefix(strings.TrimSpace(line), "replace (") {
			inReplaceBlock = true
			writer.WriteString(line + "\n")
			continue
		}

		// replace block end
		if inReplaceBlock && strings.TrimSpace(line) == ")" {
			inReplaceBlock = false
			writer.WriteString(line + "\n")
			continue
		}

		// Single replace statement processing
		if matches := replacePattern.FindStringSubmatch(line); matches != nil {
			prefix := matches[1]
			relativePath := matches[2]
			adjustedPath := adjustRelativePath(relativePath, srcDir, dstDir)
			writer.WriteString(prefix + adjustedPath + "\n")
			continue
		}

		// replace block internal processing
		if inReplaceBlock {
			if matches := replaceBlockPattern.FindStringSubmatch(line); matches != nil {
				indent := matches[1]
				relativePath := matches[2]
				suffix := matches[3]
				adjustedPath := adjustRelativePath(relativePath, srcDir, dstDir)
				writer.WriteString(indent + adjustedPath + suffix + "\n")
				continue
			}
		}

		writer.WriteString(line + "\n")
	}

	return scanner.Err()
}

// adjustRelativePath adjusts relative path from srcDir-based to dstDir-based
func adjustRelativePath(relativePath, srcDir, dstDir string) string {
	// Return as-is if not a relative path
	if !strings.HasPrefix(relativePath, ".") {
		return relativePath
	}

	// Calculate absolute path based on srcDir
	srcGoModDir := srcDir
	absPath := filepath.Join(srcGoModDir, relativePath)
	absPath = filepath.Clean(absPath)

	// Calculate relative path from dstDir to absPath
	newRelPath, err := filepath.Rel(dstDir, absPath)
	if err != nil {
		// Return original on error
		return relativePath
	}

	// Convert Windows path to Unix style (go.mod uses Unix style)
	newRelPath = filepath.ToSlash(newRelPath)

	return newRelPath
}

// injectDir injects instrumentation code into all Go files in directory
func injectDir(dir string, errorTracking bool, cfg *config.Config) error {
	injector := ast.NewInjector()
	injector.ErrorTrackingEnabled = errorTracking
	if cfg != nil {
		injector.EnabledPackages = cfg.GetEnabledPackages()
		injector.Config = cfg // For custom rules access (§4)
	}

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Get exclude patterns from config
		var excludePatterns []string
		if cfg != nil {
			excludePatterns = cfg.GetExcludePatterns()
		}

		if info.IsDir() {
			if common.ShouldSkipDirectory(path, dir, excludePatterns) {
				return filepath.SkipDir
			}
			return nil
		}

		// Process only Go files
		if filepath.Ext(path) != ".go" {
			return nil
		}

		// Skip files based on exclude patterns
		if common.ShouldSkipFile(path, dir, excludePatterns) {
			return nil
		}

		// Transform to temporary file
		tmpFile := path + ".tmp"
		if err := injector.InjectFile(path, tmpFile); err != nil {
			// Skip on transformation failure
			os.Remove(tmpFile)
			return nil
		}

		// Replace original
		if err := os.Rename(tmpFile, path); err != nil {
			os.Remove(tmpFile)
			return err
		}

		return nil
	})
}

// parseOutputFlag parses -o flag
func parseOutputFlag(args []string) (string, []string) {
	var outputFile string
	var newArgs []string

	for i := 0; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			outputFile = args[i+1]
			i++ // Skip argument after -o
			continue
		}
		if strings.HasPrefix(args[i], "-o=") {
			outputFile = strings.TrimPrefix(args[i], "-o=")
			continue
		}
		newArgs = append(newArgs, args[i])
	}

	return outputFile, newArgs
}

// runGoCommand executes go command
func runGoCommand(subCmd string, args []string) {
	newArgs := append([]string{subCmd}, args...)
	cmd := exec.Command("go", newArgs...)
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

// filterEnvVar removes environment variables with the given key from the list
// This is used to ensure we don't have duplicate environment variables (§72 fix)
func filterEnvVar(env []string, key string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}

func init() {
	rootCmd.AddCommand(goCmd)
}
