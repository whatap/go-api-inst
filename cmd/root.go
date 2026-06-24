package cmd

import (
	"fmt"
	"os"

	"github.com/whatap/go-api-inst/ast"
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

	// outputDir --output flag (instrumented source output directory).
	// §234: empty = do not save; "--output" with no value = NoOptDefVal fallback
	// ("whatap-instrumented"); "--output <dir>" = user path.
	outputDir string

	// errorTracking --error-tracking flag
	errorTracking bool

	// fastMode --fast flag (legacy no-op; fast is the only mode since §234).
	// Kept so existing Dockerfiles passing --fast still parse.
	fastMode bool

	// externalModules --external-module flag (external modules to instrument, §138)
	externalModules []string

	// globalConfig loaded configuration (used by subcommands)
	globalConfig *config.Config

	// configLoader configuration loader
	configLoader *config.Loader
)

var rootCmd = &cobra.Command{
	Use:   "whatap-go-inst",
	Short: "Go AST-based automatic instrumentation code injection/removal tool",
	Long: `whatap-go-inst is a CLI tool that automatically injects
whatap/go-api monitoring code into Go source code at build time
(toolexec pipeline) and removes it when needed.

Usage examples:
  whatap-go-inst go build ./...
  whatap-go-inst --output go build ./...              # persist to whatap-instrumented/
  whatap-go-inst --output ./out go build ./...        # persist to ./out/
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
	// §234: --output without a value falls back to "whatap-instrumented"
	// (NoOptDefVal). Empty string means "do not save instrumented source".
	rootCmd.PersistentFlags().StringVar(&outputDir, "output", "", "Instrumented source output directory (use --output with no value for 'whatap-instrumented/', or --output <dir>)")
	rootCmd.PersistentFlags().Lookup("output").NoOptDefVal = "whatap-instrumented"
	rootCmd.PersistentFlags().BoolVar(&errorTracking, "error-tracking", false, "Enable error tracking code injection")
	// §234: --fast is now the only build mode. Flag preserved as a no-op for
	// backward compatibility with Dockerfiles / scripts that still pass it.
	rootCmd.PersistentFlags().BoolVar(&fastMode, "fast", false, "Fast (toolexec) build mode — default and only mode since v0.5.5")
	_ = rootCmd.PersistentFlags().MarkHidden("fast")
	rootCmd.PersistentFlags().StringSliceVar(&externalModules, "external-module", nil, "External module to instrument from GOMODCACHE (repeatable, comma-separated)")
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

// loadDependencies loads go.mod dependencies and matches against the same
// rule set the runtime engine will actually register.
//
// §240: Previously depended on `common.GetAllTransformers()` which returned an
// empty slice after §227 Step 5 removed the v1 transformer packages (the
// `common.Register(...)` init-time calls disappeared with them). Every dependency
// was being reported as `supported=false` even though the v2 engine was
// instrumenting them correctly.
//
// Rule source is the injector's own registry — same path the engine uses at
// run time (buildRegistry: built-in rules honouring $WHATAP_RULES_YAML dev
// override, filtered by EnabledPackages + DisabledPackages for §242
// opt-in / exclusion, plus unfiltered user rules from cfg.Rules). No
// reimplementation — callers go through `ast.NewInjector() + SetConfig` so
// report and engine stay in lock step by construction.
func loadDependencies(r *report.Report, baseDir string) {
	inj := ast.NewInjector()
	if globalConfig != nil {
		inj.EnabledPackages = globalConfig.Instrumentation.EnabledPackages
		inj.DisabledPackages = globalConfig.Instrumentation.DisabledPackages
		inj.SetConfig(globalConfig)
	}

	// Dedup by package path. Multiple rules targeting the same package
	// (e.g. gin has WrapEngine + Engine.Use) collapse to a single entry —
	// the report key is the full import path, not the former Name CSV.
	infos := make([]report.TransformerInfo, 0)
	seen := make(map[string]bool)
	for _, rule := range inj.Rules() {
		pkg := ast.ExtractRulePackage(rule.Target)
		if pkg == "" || seen[pkg] {
			continue
		}
		seen[pkg] = true
		infos = append(infos, report.TransformerInfo{ImportPath: pkg})
	}

	if err := r.LoadDependenciesFromDir(baseDir, infos); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: Failed to load go.mod: %v\n", err)
		}
	}
}

// extractRulePackage moved to ast.ExtractRulePackage (§242).
