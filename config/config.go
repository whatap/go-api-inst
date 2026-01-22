package config

// Preset is the instrumentation preset type
type Preset string

const (
	// PresetFull enables all packages (default)
	PresetFull Preset = "full"
	// PresetMinimal enables trace only (minimal configuration)
	PresetMinimal Preset = "minimal"
	// PresetWeb enables web frameworks only
	PresetWeb Preset = "web"
	// PresetDatabase enables databases only
	PresetDatabase Preset = "database"
	// PresetExternal enables external services only (Redis, Kafka, gRPC, etc.)
	PresetExternal Preset = "external"
	// PresetLog enables log libraries only
	PresetLog Preset = "log"
	// PresetCustom uses custom configuration (EnabledPackages/DisabledPackages)
	PresetCustom Preset = "custom"
)

// DefaultExcludePatterns are the default file/directory patterns to skip
// These are auto-generated files or directories that should not be instrumented
var DefaultExcludePatterns = []string{
	// Generated files (proto, grpc, connect, etc.)
	"**/*.pb.go",
	"**/*.pb.gw.go",
	"**/*_grpc.pb.go",
	"**/*.connect.go",
	"**/*_generated.go",
	"**/*_gen.go",
	// Test files
	"**/*_test.go",
	// Directories
	"vendor/**",
	".git/**",
	"node_modules/**",
	"whatap-instrumented/**",
	// Whatap go-api library (prevent self-instrumentation)
	"**/github.com/whatap/go-api/**",
	// Local go-api development paths (for replace directive)
	"**/go-api/agent/**",
	"**/go-api/docs/**",
	"**/go-api/httpc/**",
	"**/go-api/instrumentation/**",
	"**/go-api/logs/**",
	"**/go-api/logsink/**",
	"**/go-api/method/**",
	"**/go-api/sql/**",
	"**/go-api/trace/**",
}

// SystemSkipPaths are environment variable names for paths that should always be skipped
// These are Go system directories that should never be instrumented
var SystemSkipPaths = []string{
	"GOROOT",
	"GOMODCACHE",
}

// PresetPackages is the list of packages included in each preset
var PresetPackages = map[Preset][]string{
	PresetMinimal: {},
	PresetWeb: {
		"gin", "echo", "fiber", "chi", "gorilla", "nethttp", "fasthttp",
	},
	PresetDatabase: {
		"sql", "sqlx", "gorm", "jinzhugorm",
	},
	PresetExternal: {
		"redigo", "goredis", "mongo", "aerospike", "sarama", "grpc", "k8s",
	},
	PresetLog: {
		"fmt", "log", "logrus", "zap",
	},
	PresetFull: {
		// Web frameworks
		"gin", "echo", "fiber", "chi", "gorilla", "nethttp", "fasthttp",
		// Databases
		"sql", "sqlx", "gorm", "jinzhugorm",
		// External services
		"redigo", "goredis", "mongo", "aerospike", "sarama", "grpc", "k8s",
		// Log libraries
		"fmt", "log", "logrus", "zap",
	},
}

// Config is the whatap-go-inst configuration struct
// Loaded from CLI options, environment variables, and config files
type Config struct {
	// Instrumentation contains instrumentation-related settings
	Instrumentation InstrumentationConfig `yaml:"instrumentation"`

	// Custom contains user-defined instrumentation rules (§4)
	Custom CustomConfig `yaml:"custom"`

	// Exclude is file patterns to exclude
	Exclude []string `yaml:"exclude"`

	// BaseDir is the base directory for all relative paths (not loaded from yaml)
	// If config file is .whatap/config.yaml, BaseDir is the parent of .whatap/
	// e.g., /myapp/.whatap/config.yaml → BaseDir = /myapp/
	BaseDir string `yaml:"-"`
}

// CustomConfig is user-defined instrumentation settings (§4)
// Execution order: Add → Inject → Replace → Hook → Transform
type CustomConfig struct {
	// Add creates new files/functions (executed first)
	Add []AddRule `yaml:"add"`

	// Inject inserts code inside function definitions
	Inject []InjectRule `yaml:"inject"`

	// Replace replaces function calls
	Replace []ReplaceRule `yaml:"replace"`

	// Hook inserts code before/after function calls
	Hook []HookRule `yaml:"hook"`

	// Transform replaces code patterns with templates (universal)
	Transform []TransformRule `yaml:"transform"`
}

// InstrumentationConfig is instrumentation options
type InstrumentationConfig struct {
	// ErrorTracking enables error tracking code injection (--error-tracking)
	ErrorTracking bool `yaml:"error_tracking"`

	// Debug enables debug mode (GO_API_AST_DEBUG=1)
	Debug bool `yaml:"debug"`

	// OutputDir is the output directory for instrumented source (GO_API_AST_OUTPUT_DIR)
	OutputDir string `yaml:"output_dir"`

	// Preset is the instrumentation preset (full, minimal, web, database, external, custom)
	// Default: full (all packages enabled)
	Preset Preset `yaml:"preset"`

	// EnabledPackages is the list of packages to enable (preset=custom or additional)
	// e.g., ["gin", "sql", "redis"]
	EnabledPackages []string `yaml:"enabled_packages"`

	// DisabledPackages is the list of packages to disable (excluded from preset)
	// e.g., ["grpc", "k8s"]
	DisabledPackages []string `yaml:"disabled_packages"`
}

// AddRule is a rule for creating new files/functions
type AddRule struct {
	// Package is the target package path (e.g., "myapp/helper")
	Package string `yaml:"package"`

	// File is the filename to create/modify (e.g., "whatap_helper.go")
	File string `yaml:"file"`

	// Content is inline code
	Content string `yaml:"content"`

	// ContentFile is the code file path (use instead of Content)
	ContentFile string `yaml:"content_file"`

	// Append: true appends to existing file, false creates new file
	Append bool `yaml:"append"`
}

// InjectRule is a rule for inserting code inside function definitions
// Note: Can only target user-defined functions (within current module)
type InjectRule struct {
	// Package is the Go import path (e.g., "myapp/service")
	Package string `yaml:"package"`

	// File is the file path pattern (use instead of Package, e.g., "internal/handler/*.go")
	File string `yaml:"file"`

	// Function is the function name pattern (e.g., "*", "Handle*")
	Function string `yaml:"function"`

	// Start is the code to insert at function start (defer pattern recommended)
	Start string `yaml:"start"`

	// End is the code to insert at function end (not executed on mid-return, use defer)
	End string `yaml:"end"`

	// Imports are import paths to add
	Imports []string `yaml:"imports"`
}

// ReplaceRule is a rule for replacing function calls
type ReplaceRule struct {
	// Package is the original package path (e.g., "database/sql")
	Package string `yaml:"package"`

	// Function is the original function name (e.g., "Open")
	Function string `yaml:"function"`

	// With is the replacement package.function (e.g., "whatapsql.Open")
	With string `yaml:"with"`

	// Imports are import paths to add
	Imports []string `yaml:"imports"`
}

// HookRule is a rule for inserting code before/after function calls
// Can use Before or After alone
type HookRule struct {
	// Package is the target package path (e.g., "mycompany/mydb")
	Package string `yaml:"package"`

	// Function is the target function name (e.g., "Query")
	Function string `yaml:"function"`

	// Before is the code to insert before call (optional)
	Before string `yaml:"before"`

	// After is the code to insert after call (optional)
	After string `yaml:"after"`

	// Imports are import paths to add
	Imports []string `yaml:"imports"`
}

// TransformRule is a rule for code pattern → template replacement (universal)
type TransformRule struct {
	// Package is the target package path (e.g., "github.com/gin-gonic/gin")
	Package string `yaml:"package"`

	// Function is the target function name (e.g., "Default")
	Function string `yaml:"function"`

	// Template is the inline template (uses {{.Original}}, {{.Var}}, {{.Args}}, etc.)
	Template string `yaml:"template"`

	// TemplateFile is the template file path
	TemplateFile string `yaml:"template_file"`

	// Imports are import paths to add
	Imports []string `yaml:"imports"`

	// Vars are user-defined template variables
	Vars map[string]interface{} `yaml:"vars"`
}

// NewConfig creates a Config with default settings
func NewConfig() *Config {
	return &Config{
		Instrumentation: InstrumentationConfig{
			ErrorTracking: false,
			Debug:         false,
			OutputDir:     "",
		},
		Custom:  CustomConfig{},
		Exclude: DefaultExcludePatterns,
	}
}

// GetExcludePatterns returns exclude patterns (DefaultExcludePatterns if empty)
func (c *Config) GetExcludePatterns() []string {
	if len(c.Exclude) == 0 {
		return DefaultExcludePatterns
	}
	return c.Exclude
}

// Merge merges non-zero values from another Config (priority: other > c)
func (c *Config) Merge(other *Config) {
	if other == nil {
		return
	}

	// Merge BaseDir (base for relative paths)
	if other.BaseDir != "" {
		c.BaseDir = other.BaseDir
	}

	// Merge Instrumentation
	if other.Instrumentation.ErrorTracking {
		c.Instrumentation.ErrorTracking = true
	}
	if other.Instrumentation.Debug {
		c.Instrumentation.Debug = true
	}
	if other.Instrumentation.OutputDir != "" {
		c.Instrumentation.OutputDir = other.Instrumentation.OutputDir
	}
	if other.Instrumentation.Preset != "" {
		c.Instrumentation.Preset = other.Instrumentation.Preset
	}
	if len(other.Instrumentation.EnabledPackages) > 0 {
		c.Instrumentation.EnabledPackages = append(c.Instrumentation.EnabledPackages, other.Instrumentation.EnabledPackages...)
	}
	if len(other.Instrumentation.DisabledPackages) > 0 {
		c.Instrumentation.DisabledPackages = append(c.Instrumentation.DisabledPackages, other.Instrumentation.DisabledPackages...)
	}

	// Merge Custom (add)
	if len(other.Custom.Add) > 0 {
		c.Custom.Add = append(c.Custom.Add, other.Custom.Add...)
	}
	if len(other.Custom.Inject) > 0 {
		c.Custom.Inject = append(c.Custom.Inject, other.Custom.Inject...)
	}
	if len(other.Custom.Replace) > 0 {
		c.Custom.Replace = append(c.Custom.Replace, other.Custom.Replace...)
	}
	if len(other.Custom.Hook) > 0 {
		c.Custom.Hook = append(c.Custom.Hook, other.Custom.Hook...)
	}
	if len(other.Custom.Transform) > 0 {
		c.Custom.Transform = append(c.Custom.Transform, other.Custom.Transform...)
	}

	// Merge Exclude (add)
	if len(other.Exclude) > 0 {
		c.Exclude = append(c.Exclude, other.Exclude...)
	}
}

// GetEnabledPackages returns the list of enabled packages
// Preset default packages + EnabledPackages - DisabledPackages
func (c *Config) GetEnabledPackages() []string {
	preset := c.Instrumentation.Preset
	if preset == "" {
		preset = PresetFull // default
	}

	// Get default package list
	var packages []string
	if preset == PresetCustom {
		// custom preset: use EnabledPackages only
		packages = make([]string, len(c.Instrumentation.EnabledPackages))
		copy(packages, c.Instrumentation.EnabledPackages)
	} else {
		// Other presets: get from PresetPackages
		base := PresetPackages[preset]
		packages = make([]string, len(base))
		copy(packages, base)

		// Add EnabledPackages
		for _, pkg := range c.Instrumentation.EnabledPackages {
			if !contains(packages, pkg) {
				packages = append(packages, pkg)
			}
		}
	}

	// Remove DisabledPackages
	if len(c.Instrumentation.DisabledPackages) > 0 {
		var filtered []string
		for _, pkg := range packages {
			if !contains(c.Instrumentation.DisabledPackages, pkg) {
				filtered = append(filtered, pkg)
			}
		}
		packages = filtered
	}

	return packages
}

// IsPackageEnabled checks if a specific package is enabled
func (c *Config) IsPackageEnabled(name string) bool {
	enabled := c.GetEnabledPackages()
	return contains(enabled, name)
}

// contains checks if slice contains the item
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
