package cmd

import (
	"fmt"
	"os"

	"github.com/whatap/go-api-inst/ast"
	"github.com/whatap/go-api-inst/report"

	"github.com/spf13/cobra"
)

var (
	injectSrc string
)

var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Inject monitoring code into source code",
	Long: `Automatically injects whatap/go-api monitoring code into target Go source code.

Injected content:
  - trace.Init/Shutdown initialization code
  - Web framework middleware (Gin, Echo, etc.)
  - HTTP client tracing code
  - Database tracing code`,
	Run: func(cmd *cobra.Command, args []string) {
		// Verify source path
		info, err := os.Stat(injectSrc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Source path not found: %s\n", injectSrc)
			os.Exit(1)
		}

		// Reload config based on srcDir (search for go.mod location)
		cfg := ReloadConfigWithProjectDir(injectSrc)

		// Initialize report
		InitReport("inject")

		// Load dependencies from go.mod
		r := report.Get()
		r.SetDirs(injectSrc, outputDir)
		loadDependencies(r, cfg.BaseDir)

		// Use rootCmd flags (outputDir, errorTracking)
		// Default output directory for inject is "./output"
		injectOutput := outputDir
		if injectOutput == "" {
			injectOutput = "./output"
		}

		if !quiet {
			fmt.Printf("Source path: %s\n", injectSrc)
			fmt.Printf("Output path: %s\n", injectOutput)
			fmt.Println()
		}

		// ErrorTracking: Use CLI flag if explicitly set, otherwise use config file value
		errorTrackingEnabled := cfg.Instrumentation.ErrorTracking
		if cmd.Root().PersistentFlags().Changed("error-tracking") {
			errorTrackingEnabled = errorTracking
		}

		injector := ast.NewInjector()
		injector.ErrorTrackingEnabled = errorTrackingEnabled
		injector.EnabledPackages = cfg.GetEnabledPackages()
		injector.Config = cfg // For custom rules access (§4)

		if cfg.Instrumentation.Debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] ErrorTracking: %v\n", errorTrackingEnabled)
		}

		if info.IsDir() {
			// Directory processing
			if err := injector.InjectDir(injectSrc, injectOutput); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Single file processing
			if err := injector.InjectFile(injectSrc, injectOutput); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

		// Finalize report
		FinalizeReport()
	},
}

func init() {
	injectCmd.Flags().StringVarP(&injectSrc, "src", "s", ".", "Source code path")
	// Add -o shorthand for output (--output is inherited from rootCmd PersistentFlags)
	injectCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory (default: ./output)")
	rootCmd.AddCommand(injectCmd)
}
