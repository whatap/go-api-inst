package cmd

import (
	"fmt"
	"os"

	"github.com/whatap/go-api-inst/ast"

	"github.com/spf13/cobra"
)

var (
	removeSrc string
	removeAll bool
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove monitoring code from source code",
	Long: `Automatically removes whatap/go-api monitoring code from target Go source code.

Removed content:
  - trace.Init/Shutdown initialization code
  - Web framework middleware
  - HTTP client tracing code
  - Database tracing code

--all option:
  Also removes manually inserted patterns beyond inject patterns.
  - Standalone statements like trace.Step(), trace.Println()
  - AddHook() method calls
  - Non-removable patterns (Start/End pairs, Wrap closures) will show warnings`,
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize report
		InitReport("remove")

		// Use rootCmd flags (outputDir)
		// Default output directory for remove is "./output"
		removeOutput := outputDir
		if removeOutput == "" {
			removeOutput = "./output"
		}

		if !quiet {
			fmt.Printf("Source path: %s\n", removeSrc)
			fmt.Printf("Output path: %s\n", removeOutput)
			if removeAll {
				fmt.Println("Mode: --all (including manually inserted patterns)")
			}
			fmt.Println()
		}

		// Verify source path
		info, err := os.Stat(removeSrc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Source path not found: %s\n", removeSrc)
			os.Exit(1)
		}

		remover := ast.NewRemover(removeAll)

		if info.IsDir() {
			// Directory processing
			if err := remover.RemoveDir(removeSrc, removeOutput); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Single file processing
			if err := remover.RemoveFile(removeSrc, removeOutput); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

		// Finalize report
		FinalizeReport()
	},
}

func init() {
	removeCmd.Flags().StringVarP(&removeSrc, "src", "s", ".", "Source code path")
	// Add -o shorthand for output (--output is inherited from rootCmd PersistentFlags)
	removeCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory (default: ./output)")
	removeCmd.Flags().BoolVar(&removeAll, "all", false, "Also remove manually inserted patterns (standalone statements, AddHook, etc.)")
	rootCmd.AddCommand(removeCmd)
}
