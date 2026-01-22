package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go-api-inst/ast"

	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Inject code when called from go:generate",
	Long: `Called by go generate to inject monitoring code into Go files in the current package.

This command is not executed directly but through go:generate directive:
  //go:generate whatap-go-inst generate`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check current file from GOFILE environment variable
		gofile := os.Getenv("GOFILE")
		gopackage := os.Getenv("GOPACKAGE")

		if gofile == "" {
			// Process current directory if executed directly
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			if err := generateDir(dir); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		fmt.Printf("Running go:generate... (package: %s, file: %s)\n", gopackage, gofile)

		// Process Go files in current directory
		if err := generateDir("."); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// generateDir injects monitoring code into Go files in directory (in-place)
func generateDir(dir string) error {
	injector := ast.NewInjector()

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Skip vendor, .git, etc.
			base := filepath.Base(path)
			if base == "vendor" || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Process only Go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip generated files, test files
		if strings.HasSuffix(path, "_test.go") ||
			strings.HasSuffix(path, "_generate.go") ||
			strings.Contains(path, "generated") {
			return nil
		}

		// Transform to temporary file
		tmpFile := path + ".tmp"
		if err := injector.InjectFile(path, tmpFile); err != nil {
			// Skip on transformation failure (already transformed or cannot transform)
			os.Remove(tmpFile)
			return nil
		}

		// Replace if there are changes compared to original
		origContent, _ := os.ReadFile(path)
		newContent, _ := os.ReadFile(tmpFile)

		if string(origContent) != string(newContent) {
			fmt.Printf("Transformed: %s\n", path)
			if err := os.Rename(tmpFile, path); err != nil {
				os.Remove(tmpFile)
				return err
			}
		} else {
			os.Remove(tmpFile)
		}

		return nil
	})
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
