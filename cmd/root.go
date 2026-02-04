package cmd

import (
	"fmt"
	"os"

	"github.com/whatap/go-api-inst/ast/common"
	"github.com/whatap/go-api-inst/config"
	"github.com/whatap/go-api-inst/report"

	"github.com/spf13/cobra"
)

var (
	// configPath --config flag value
	configPath string

	// verbose --verbose flag
	verbose bool

	// quiet --quiet flag
	quiet bool

	// reportPath --report flag
	reportPath string

	// outputDir --output flag (instrumented source output directory)
	outputDir string

	// noOutput --no-output flag (do not save instrumented source)
	noOutput bool

	// errorTracking --error-tracking flag
	errorTracking bool

	// fastMode --fast flag (use toolexec, requires init)
	fastMode bool

	// globalConfig loaded configuration (used by subcommands)
	globalConfig *config.Config

	// configLoader configuration loader
	configLoader *config.Loader
)

var rootCmd = &cobra.Command{
	Use:   "whatap-go-inst",
	Short: "Go AST-based automatic instrumentation code injection/removal tool",
	Long: `whatap-go-inst is a CLI tool that automatically injects or removes
whatap/go-api monitoring code into Go source code at build time.

Usage examples:
  whatap-go-inst inject --src ./myapp --output ./instrumented
  whatap-go-inst remove --src ./myapp --output ./clean

Configuration:
  Default options can be set in .whatap/config.yaml or whatap-inst.yaml.
  Priority: CLI options > environment variables > config file > defaults`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// go command loads config on its own (due to DisableFlagParsing)
		if cmd.Name() == "go" {
			return
		}

		// toolexec mode: use GO_API_PROJECT_DIR environment variable (§72 fix)
		// This is set by 'whatap-go-inst go build' to pass project directory
		projectDir := os.Getenv("GO_API_PROJECT_DIR")
		if projectDir != "" {
			loadConfigWithProjectDir(projectDir)
			return
		}

		loadConfig()
	},
}

// loadConfig loads configuration
func loadConfig() {
	loadConfigWithProjectDir("")
}

// loadConfigWithProjectDir loads configuration based on project directory
// projectDir: path to start go.mod search (empty string for cwd)
func loadConfigWithProjectDir(projectDir string) {
	configLoader = config.NewLoader()
	configLoader.ConfigPath = configPath
	configLoader.ProjectDir = projectDir

	var err error
	globalConfig, err = configLoader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config file: %v\n", err)
		os.Exit(1)
	}

	// Debug mode output
	if globalConfig.Instrumentation.Debug {
		if path := configLoader.GetConfigPath(); path != "" {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Config file: %s\n", path)
		}
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] BaseDir: %s\n", globalConfig.BaseDir)
	}
}

// GetConfig returns the current configuration
func GetConfig() *config.Config {
	if globalConfig == nil {
		return config.NewConfig()
	}
	return globalConfig
}

// ReloadConfigWithProjectDir reloads configuration based on project directory
// Called from inject, remove commands after srcDir is determined
func ReloadConfigWithProjectDir(projectDir string) *config.Config {
	loadConfigWithProjectDir(projectDir)
	return globalConfig
}

func Execute() {
	// TraverseChildren allows parsing persistent flags even for commands with DisableFlagParsing
	rootCmd.TraverseChildren = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Config file path (default: .whatap/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output (include transformation details)")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Summary only output")
	rootCmd.PersistentFlags().StringVar(&reportPath, "report", "", "Report file path (JSON format)")
	rootCmd.PersistentFlags().StringVar(&outputDir, "output", "", "Instrumented source output directory (default: whatap-instrumented/)")
	rootCmd.PersistentFlags().BoolVar(&noOutput, "no-output", false, "Do not save instrumented source")
	rootCmd.PersistentFlags().BoolVar(&errorTracking, "error-tracking", false, "Enable error tracking code injection")
	rootCmd.PersistentFlags().BoolVar(&fastMode, "fast", false, "Fast mode (use toolexec, requires init)")
}

// InitReport initializes report (called from subcommands)
func InitReport(command string) {
	report.Init(command)

	// Set log level
	if quiet {
		report.SetLevel(report.LogQuiet)
	} else if verbose {
		report.SetLevel(report.LogVerbose)
	} else {
		report.SetLevel(report.LogNormal)
	}
}

// FinalizeReport finalizes report (called from subcommands)
func FinalizeReport() {
	r := report.Get()
	r.PrintSummary()

	if reportPath != "" {
		if err := r.SaveJSON(reportPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save report: %v\n", err)
		}
	}
}

// loadDependencies loads go.mod dependencies and matches against transformers
func loadDependencies(r *report.Report, baseDir string) {
	// Get all registered transformers
	transformers := common.GetAllTransformers()

	// Convert to TransformerInfo
	infos := make([]report.TransformerInfo, len(transformers))
	for i, t := range transformers {
		infos[i] = report.TransformerInfo{
			Name:       t.Name(),
			ImportPath: t.ImportPath(),
		}
	}

	// Load dependencies
	if err := r.LoadDependenciesFromDir(baseDir, infos); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: Failed to load go.mod: %v\n", err)
		}
	}
}
