package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/whatap/go-api-inst/ast"
	"github.com/whatap/go-api-inst/ast/common"

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

// processCompileArgs processes compile arguments and transforms source
func processCompileArgs(args []string) []string {
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
			goFiles = append(goFiles, arg)
		} else {
			otherArgs = append(otherArgs, arg)
		}
	}

	// Return as-is if no Go files
	if len(goFiles) == 0 {
		return args
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "whatap-go-inst-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp directory: %v\n", err)
		return args
	}

	// Directory to save transformed files (specified via environment variable)
	saveDir := os.Getenv("GO_API_AST_OUTPUT_DIR")

	// Transformed files
	injector := ast.NewInjector()
	var transformedFiles []string

	for _, goFile := range goFiles {
		// Skip standard library and external packages
		if shouldSkipFile(goFile) {
			transformedFiles = append(transformedFiles, goFile)
			continue
		}

		// Temporary file path
		tmpFile := filepath.Join(tmpDir, filepath.Base(goFile))

		// Attempt AST transformation
		err := injector.InjectFile(goFile, tmpFile)
		if err != nil {
			// Use original on transformation failure
			transformedFiles = append(transformedFiles, goFile)
			continue
		}

		transformedFiles = append(transformedFiles, tmpFile)

		// Save transformed file (if saveDir is set)
		if saveDir != "" {
			saveInstrumentedFile(goFile, tmpFile, saveDir)
		}
	}

	// Debug output
	if os.Getenv("GO_API_AST_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] output: %s\n", outputFile)
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] transformed: %v\n", transformedFiles)
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
		// Use filename only for files outside cwd
		relPath = filepath.Base(originalPath)
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

// shouldSkipFile checks if the file should be skipped for transformation
// Uses common.ShouldSkipFile for unified skip logic
func shouldSkipFile(path string) bool {
	// Get current working directory as base path for pattern matching
	basePath, _ := os.Getwd()
	skip := common.ShouldSkipFile(path, basePath, nil)

	// Debug output for skip logic
	if os.Getenv("GO_API_AST_DEBUG") != "" && !skip {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] shouldSkipFile: path=%q basePath=%q skip=%v\n", path, basePath, skip)
	}
	return skip
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

func init() {
	rootCmd.AddCommand(toolexecCmd)
}
