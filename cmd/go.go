package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
  whatap-go-inst --output go build ./...              # persist to whatap-instrumented/ (NoOptDefVal)
  whatap-go-inst --output ./instrumented go build ./...

Global options (before 'go'):
  --error-tracking    Enable error tracking code injection
  --config <path>     Specify config file path
  --output [<dir>]    Persist instrumented source tree. No value → whatap-instrumented/.
                      Omit flag → no save. Emitted tree is standalone-buildable
                      (originals + transformed + _modules/ + go.mod replace).
  --external-module   Module(s) from GOMODCACHE to instrument (repeatable, comma-separated)

Internal behavior (§234 — toolexec only, post v0.5.5):
  1. Auto-add github.com/whatap/go-api dependency (if not in go.mod) + go mod tidy
  2. Pre-resolve whatap package archives via go list
  3. Spawn 'go build' with -toolexec=whatap-go-inst; transformed .go files are
     written directly under $WORK/bXXX/whatap/src/<importpath>/ so the compiler
     picks them up without an intermediate tmpDir
  4. Patch importcfg with resolved archive paths for new whatap imports
  5. If --output is set, copy the resulting source tree (originals + transformed
     + go.mod/go.sum + _modules/ for external modules) for standalone reproduction`,
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		// Use global flags from rootCmd (errorTracking, outputDir, externalModules,
		// configPath). --fast is retained as a hidden no-op for backward compat.
		// Load config
		loader := config.NewLoader()
		loader.ConfigPath = configPath
		cfg, err := loader.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config file: %v\n", err)
			os.Exit(1)
		}

		// ErrorTracking: Use CLI flag if explicitly set, otherwise use config file value
		errorTrackingEnabled := cfg.Instrumentation.ErrorTracking
		if cmd.Root().PersistentFlags().Changed("error-tracking") {
			errorTrackingEnabled = errorTracking
		}

		// Debug mode output
		if cfg.Instrumentation.Debug {
			if path := loader.GetConfigPath(); path != "" {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] Config file: %s\n", path)
			}
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] ErrorTracking: %v\n", errorTrackingEnabled)
			if cfg.HasExternalModules() {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] ExternalModules: %v\n", cfg.ExternalModules)
			}
		}

		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Error: Please specify a go command")
			fmt.Fprintln(os.Stderr, "Example: whatap-go-inst go build ./...")
			os.Exit(1)
		}

		// args are go subcommand and its arguments (e.g., ["build", "./..."])
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
				// Reapply ErrorTracking from CLI flag
				if cmd.Root().PersistentFlags().Changed("error-tracking") {
					errorTrackingEnabled = errorTracking
				} else {
					errorTrackingEnabled = cfg.Instrumentation.ErrorTracking
				}

				if cfg.Instrumentation.Debug {
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] ProjectDir: %s\n", projectDir)
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] BaseDir: %s\n", cfg.BaseDir)
				}
			}

			// Apply CLI --output flag to config
			if outputDir != "" {
				cfg.Instrumentation.OutputDir = outputDir
			}

			// Apply CLI --external-module flag to config (§138)
			if cmd.Root().PersistentFlags().Changed("external-module") {
				for _, mod := range externalModules {
					found := false
					for _, existing := range cfg.ExternalModules {
						if existing == mod {
							found = true
							break
						}
					}
					if !found {
						cfg.ExternalModules = append(cfg.ExternalModules, mod)
					}
				}
			}

			// §234 step 10: fast (toolexec) is the only build mode. The legacy
			// wrap/inject/generate paths were removed after §195 promoted fast
			// to default (2026-03-19). --fast is retained as a no-op for
			// backward compatibility. See runFastBuild in go_fast.go.
			runFastBuild(goSubCmd, goArgs, cfg, errorTrackingEnabled)
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

// convertBuildTargets rewrites build target paths so `go build` is invoked from
// the project root (srcDir = go.mod dir). When the user runs whatap-go-inst
// from a sub-directory, targets like `.` or `./...` need to be translated to
// the equivalent path relative to srcDir.
// Example (cwd=project/cmd/app, srcDir=project): "." → "./cmd/app"
func convertBuildTargets(args []string, srcDir, cwd string) []string {
	absSrcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return args
	}

	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		absCwd = cwd
	}

	// Calculate relative path from srcDir to cwd (for . target conversion)
	cwdRelPath, _ := filepath.Rel(absSrcDir, absCwd)
	if cwdRelPath == "" {
		cwdRelPath = "."
	}

	// Flags that take a value as the next argument (not combined with =)
	// go build flags
	flagsWithValue := map[string]bool{
		"-o": true, "-p": true, "-asmflags": true, "-buildmode": true,
		"-compiler": true, "-gccgoflags": true, "-gcflags": true,
		"-installsuffix": true, "-ldflags": true, "-mod": true,
		"-modfile": true, "-overlay": true, "-pkgdir": true,
		"-tags": true, "-toolexec": true,
		// go test flags
		"-bench": true, "-benchtime": true, "-blockprofile": true,
		"-blockprofilerate": true, "-count": true, "-covermode": true,
		"-coverpkg": true, "-coverprofile": true, "-cpu": true,
		"-cpuprofile": true, "-exec": true, "-memprofile": true,
		"-memprofilerate": true, "-mutexprofile": true,
		"-mutexprofilefraction": true, "-outputdir": true,
		"-parallel": true, "-run": true, "-shuffle": true,
		"-timeout": true, "-trace": true,
	}

	var result []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Keep flags and their values as-is
		if strings.HasPrefix(arg, "-") {
			result = append(result, arg)

			// Check if this flag takes a value as the next argument
			// Extract flag name (handle -flag=value format)
			flagName := arg
			if idx := strings.Index(arg, "="); idx != -1 {
				flagName = arg[:idx]
			}

			// If flag takes a value and it's not in -flag=value format, skip next arg
			if takesValue, ok := flagsWithValue[flagName]; ok && takesValue && !strings.Contains(arg, "=") {
				if i+1 < len(args) {
					i++
					result = append(result, args[i])
				}
			}
			continue
		}

		// Handle /... suffix
		suffix := ""
		target := arg
		if strings.HasSuffix(target, "/...") {
			suffix = "/..."
			target = strings.TrimSuffix(target, "/...")
		}

		// Convert . or ./ to relative path from srcDir to cwd
		// e.g., if cwd=project/cmd/app and srcDir=project, "." becomes "./cmd/app"
		if target == "." || target == "./" {
			if cwdRelPath == "." {
				result = append(result, "./"+suffix)
			} else {
				result = append(result, "./"+filepath.ToSlash(cwdRelPath)+suffix)
			}
			continue
		}

		// Handle absolute/relative paths
		absTarget, err := filepath.Abs(target)
		if err != nil {
			result = append(result, arg)
			continue
		}

		// Check if path is inside srcDir
		relPath, err := filepath.Rel(absSrcDir, absTarget)
		if err != nil || strings.HasPrefix(relPath, "..") {
			// If outside srcDir, keep as-is (external package)
			result = append(result, arg)
			continue
		}

		// Convert to relative path based on srcDir
		// If relPath is ".", use "./", otherwise "./" + relPath
		if relPath == "." {
			if suffix != "" {
				result = append(result, "."+suffix) // ./... (suffix already has /)
			} else {
				result = append(result, "./")
			}
		} else {
			result = append(result, "./"+filepath.ToSlash(relPath)+suffix)
		}
	}
	return result
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
