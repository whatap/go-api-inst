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
	Short: "Clean up manually written whatap/go-api code from a source tree",
	Long: `Removes hand-written whatap/go-api imports and calls from a Go source tree.

Primary use case: migrating from manual SDK usage to auto-instrumentation
(toolexec build wrapper). The build wrapper rewrites code in $WORK only, so
its output never reaches your source tree — stopping the wrapper is enough
to revert auto-injected changes. 'remove' exists to clean up the *manually
written* whatap calls and imports you may have left in the tree from an
earlier manual integration.

Removed content:
  - whatap/go-api imports
  - trace.Init / trace.Shutdown initialization (in main)
  - Standalone statements: trace.Step / trace.End / defer trace.End / logsink.* / etc.
  - AddHook(whatap…) method calls
  - Error-tracing scaffolding added by previous '--error-tracking' runs
  - Wrapped function calls — restored to their originals (e.g. whatapsql.Open(...) → sql.Open(...)).
    Covers 25 ReplaceFunction patterns across sql / sqlx / gorm / go-redis / redigo / mongo / fmt.

Patterns kept (with warnings) for manual review:
  - Variable assignments whose RHS is a whatap call (e.g. ctx, _ := trace.Start(...))
  - Struct field assignments using whatap values
  - Closures / factories (whatapsql.Wrap, *.NewClient) where removing the wrapper
    changes program semantics

See §272 for the design rationale.`,
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
			fmt.Println()
		}

		// Verify source path
		info, err := os.Stat(removeSrc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Source path not found: %s\n", removeSrc)
			os.Exit(1)
		}

		// §272 — manual pattern removal is the default; removeAll is a no-op
		// but we pass true so any internal RemoveAll-gated logic in older
		// code paths stays inert with the same behaviour.
		_ = removeAll
		remover := ast.NewRemover(true)

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
	removeCmd.Flags().BoolVar(&removeAll, "all", false, "DEPRECATED — manual pattern removal is the default since §272; this flag is a no-op")
	if err := removeCmd.Flags().MarkDeprecated("all", "manual pattern removal is the default since §272; this flag is a no-op (will be removed in a future version)"); err != nil {
		// Cobra returns an error only if the flag name doesn't exist; ignore safely.
		_ = err
	}
	rootCmd.AddCommand(removeCmd)
}
