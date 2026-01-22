package cmd

import (
	"fmt"
	"os"

	"go-api-inst/ast"

	"github.com/spf13/cobra"
)

var (
	injectSrc           string
	injectOutput        string
	injectErrorTracking bool
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

		if !quiet {
			fmt.Printf("Source path: %s\n", injectSrc)
			fmt.Printf("Output path: %s\n", injectOutput)
			fmt.Println()
		}

		// ErrorTracking: Use CLI flag if explicitly set, otherwise use config file value
		errorTracking := cfg.Instrumentation.ErrorTracking
		if cmd.Flags().Changed("error-tracking") {
			errorTracking = injectErrorTracking
		}

		injector := ast.NewInjector()
		injector.ErrorTrackingEnabled = errorTracking
		injector.EnabledPackages = cfg.GetEnabledPackages()
		injector.Config = cfg // For custom rules access (§4)

		if cfg.Instrumentation.Debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] ErrorTracking: %v\n", errorTracking)
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
	injectCmd.Flags().StringVarP(&injectOutput, "output", "o", "./output", "Output directory")
	injectCmd.Flags().BoolVar(&injectErrorTracking, "error-tracking", false, "Enable error tracking code injection (adds trace.Error to if err != nil patterns)")
	rootCmd.AddCommand(injectCmd)
}
