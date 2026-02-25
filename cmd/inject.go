package cmd

import (
	"fmt"
	"os"
	"os/exec"

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

		debug := cfg.Instrumentation.Debug

		injector := ast.NewInjector()
		injector.ErrorTrackingEnabled = errorTrackingEnabled
		injector.EnabledPackages = cfg.GetEnabledPackages()
		injector.Config = cfg // For custom rules access (§4)

		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] ErrorTracking: %v\n", errorTrackingEnabled)
			if cfg.HasExternalModules() {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] ExternalModules: %v\n", cfg.ExternalModules)
			}
		}

		if info.IsDir() {
			// Directory processing
			if err := injector.InjectDir(injectSrc, injectOutput); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Process external modules (§138)
			if cfg.HasExternalModules() {
				if debug {
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] Processing external modules...\n")
				}

				results, extErr := prepareExternalModules(cfg, injectSrc, injectOutput, errorTrackingEnabled, debug)
				if extErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: external module processing failed: %v\n", extErr)
				} else if len(results) > 0 {
					// Resolve go-api version and add require to each module's go.mod
					// Use go mod edit -require (not go get) to avoid network hangs
					goAPIVersion := resolveGoAPILatestVersion(injectSrc, debug)
					if goAPIVersion != "" {
						requireArg := fmt.Sprintf("github.com/whatap/go-api@%s", goAPIVersion)
						for _, result := range results {
							editCmd := exec.Command("go", "mod", "edit", "-require="+requireArg)
							editCmd.Dir = result.LocalDir
							if out, err := editCmd.CombinedOutput(); err != nil {
								if debug {
									fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: go mod edit -require in %s: %s\n",
										result.ModulePath, string(out))
								}
							} else if debug {
								fmt.Fprintf(os.Stderr, "[whatap-go-inst]   Added go-api %s to: %s\n",
									goAPIVersion, result.ModulePath)
							}
						}
					} else if debug {
						fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: could not resolve go-api version; "+
							"external modules will need manual go-api dependency\n")
					}

					if !quiet {
						fmt.Printf("\nExternal modules instrumented:\n")
						for _, result := range results {
							fmt.Printf("  %s@%s\n", result.ModulePath, result.Version)
						}
					}
				}
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
