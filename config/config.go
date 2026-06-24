package config

import "gopkg.in/yaml.v3"

// DefaultCopyExcludeDirs are the default directories to skip when copying source files
var DefaultCopyExcludeDirs = []string{
	".git",
	".idea",
	".vscode",
	".github",
	"whatap-instrumented", // default output directory
	"node_modules",        // large, not needed for Go build
}

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

// Config is the whatap-go-inst configuration struct
// Loaded from CLI options, environment variables, and config files
type Config struct {
	// Version is the rules schema version (§227 Step 4). Currently only 1
	// is recognised. Optional — absence is tolerated for forward compat but
	// the legacy CustomConfig path no longer exists (removed in §227 Step 5).
	Version int `yaml:"version,omitempty"`

	// Instrumentation contains instrumentation-related settings
	Instrumentation InstrumentationConfig `yaml:"instrumentation"`

	// Add is the top-level `add:` array (§227 Step 5). Engine 밖 처리 —
	// `ast/custom/add.go` 가 소비. 다른 user 규칙은 unified `rules:` 배열
	// 에 들어간다.
	Add []AddRule `yaml:"add,omitempty"`

	// Imports lists file-global imports for the new unified rules (§227 Step 4).
	// Merged with rule-level imports inside ast.LoadCustomRules.
	Imports []string `yaml:"imports,omitempty"`

	// ImportAliases is the file-global alias→path map for the new unified rules.
	// Merged with rule-level importAliases inside ast.LoadCustomRules.
	ImportAliases map[string]string `yaml:"importAliases,omitempty"`

	// Rules is the unified Rule list (§227 Step 4). Each element is a yaml.Node
	// so that rule decoding (type discriminator + per-type fields) lives in the
	// ast package — keeps the config package free of an ast import cycle.
	Rules []yaml.Node `yaml:"rules,omitempty"`

	// Exclude is file patterns to exclude from instrumentation
	Exclude []string `yaml:"exclude"`

	// CopyExclude is directories to exclude when copying source files
	// These are added to DefaultCopyExcludeDirs
	CopyExclude []string `yaml:"copy_exclude"`

	// ExternalModules is external modules (in GOMODCACHE) to instrument (§138)
	// These modules are copied from GOMODCACHE, injected, and linked via replace directive
	// e.g., ["gitrepo.xlaxiata.id/go-module/goutils/v3", "mycompany.com/internal/lib"]
	ExternalModules []string `yaml:"external_modules"`

	// BaseDir is the base directory for all relative paths (not loaded from yaml)
	// If config file is .whatap/config.yaml, BaseDir is the parent of .whatap/
	// e.g., /myapp/.whatap/config.yaml → BaseDir = /myapp/
	BaseDir string `yaml:"-"`
}

// InstrumentationConfig is instrumentation options
type InstrumentationConfig struct {
	// ErrorTracking enables error tracking code injection (--error-tracking)
	ErrorTracking bool `yaml:"error_tracking"`

	// Debug enables debug mode (GO_API_AST_DEBUG=1)
	Debug bool `yaml:"debug"`

	// OutputDir is the output directory for instrumented source (GO_API_AST_OUTPUT_DIR)
	OutputDir string `yaml:"output_dir"`

	// §242 — EnabledPackages opts in to rules whose `optin: true` is set
	// (e.g. `fmt`). DisabledPackages removes rules whose package path is in
	// the list. Both use full Rule.Target package paths:
	//   - stdlib:  "fmt", "database/sql", "log"
	//   - modules: "github.com/gin-gonic/gin", "github.com/labstack/echo/v4"
	// Matching is exact — "github.com/labstack/echo" does NOT match v4.
	EnabledPackages  []string `yaml:"enabled_packages"`
	DisabledPackages []string `yaml:"disabled_packages"`

	// §271 — SkipReplacedModules controls whether the v2 engine skips Rules
	// whose target module appears in a go.mod `replace` directive. Default
	// (nil pointer) is true — preserves §205 behaviour (safer: forks may
	// have incompatible APIs that break the whatap wrap). Set to false only
	// when you know your replace target is signature-compatible with the
	// original and you want instrumentation applied anyway.
	SkipReplacedModules *bool `yaml:"skip_replaced_modules,omitempty"`
}

// ShouldSkipReplacedModules returns the effective value of
// SkipReplacedModules with nil treated as the default (true).
func (c InstrumentationConfig) ShouldSkipReplacedModules() bool {
	if c.SkipReplacedModules == nil {
		return true
	}
	return *c.SkipReplacedModules
}

// AddRule is a rule for creating a new file in a target package.
//
// Content must be a complete Go source file — `package <name>` declaration,
// any `import` statements, and the declarations themselves. The tool writes
// the content verbatim; there is no auto-wrapping.
//
// File must not match the default exclude patterns (`**/*_generated.go`,
// `**/*_gen.go`, `**/*_test.go`, etc.) or fast mode's toolexec will skip
// instrumentation of the file. Naming convention: `whatap_*.go` or
// `<yourmod>_ext.go`.
//
// Existing files are never overwritten — the build aborts with an error if
// the target path already exists.
type AddRule struct {
	// Package is the target package path relative to the project root
	// (e.g., "pkg/user", "main", or "."). "main"/"."/"" map to the project
	// root.
	Package string `yaml:"package"`

	// File is the filename to create (e.g., "whatap_helper.go").
	File string `yaml:"file"`

	// Content is the inline Go source. Must start with `package <name>` —
	// see type doc for contract.
	Content string `yaml:"content"`

	// ContentFile is a filesystem path (relative to the config file's
	// directory) whose contents are used instead of Content.
	ContentFile string `yaml:"content_file"`

	// Append was removed in v0.5.5. Presence of `append:` in yaml triggers a
	// migration error at load time. DO NOT use — this field exists only so
	// the yaml decoder can catch deprecated rules and emit a clear message.
	//
	// Deprecated: migrate to a new-file add rule in the same package.
	Append bool `yaml:"append,omitempty"`
}

// NewConfig creates a Config with default settings
func NewConfig() *Config {
	return &Config{
		Instrumentation: InstrumentationConfig{
			ErrorTracking: false,
			Debug:         false,
			OutputDir:     "",
		},
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

// GetCopyExcludeDirs returns directories to exclude when copying
// Returns DefaultCopyExcludeDirs + user-defined CopyExclude
func (c *Config) GetCopyExcludeDirs() []string {
	// Start with default list
	result := make([]string, len(DefaultCopyExcludeDirs))
	copy(result, DefaultCopyExcludeDirs)

	// Add user-defined excludes
	for _, dir := range c.CopyExclude {
		if !contains(result, dir) {
			result = append(result, dir)
		}
	}

	return result
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
	if len(other.Instrumentation.EnabledPackages) > 0 {
		c.Instrumentation.EnabledPackages = append(c.Instrumentation.EnabledPackages, other.Instrumentation.EnabledPackages...)
	}
	if len(other.Instrumentation.DisabledPackages) > 0 {
		c.Instrumentation.DisabledPackages = append(c.Instrumentation.DisabledPackages, other.Instrumentation.DisabledPackages...)
	}
	// §271 — SkipReplacedModules merge: only overwrite when the other source
	// set it explicitly (pointer non-nil). Default (nil) leaves the current
	// value untouched so layered configs (defaults → file → CLI) compose.
	if other.Instrumentation.SkipReplacedModules != nil {
		v := *other.Instrumentation.SkipReplacedModules
		c.Instrumentation.SkipReplacedModules = &v
	}


	// Merge Exclude (add)
	if len(other.Exclude) > 0 {
		c.Exclude = append(c.Exclude, other.Exclude...)
	}

	// Merge CopyExclude (add)
	if len(other.CopyExclude) > 0 {
		c.CopyExclude = append(c.CopyExclude, other.CopyExclude...)
	}

	// Merge ExternalModules (add, deduplicate)
	for _, mod := range other.ExternalModules {
		if !contains(c.ExternalModules, mod) {
			c.ExternalModules = append(c.ExternalModules, mod)
		}
	}

	// Merge new unified Rules schema (§227 Step 4). Behaviour is intentionally
	// permissive: later sources append rules, override the version field, and
	// extend imports / importAliases.
	if other.Version != 0 {
		c.Version = other.Version
	}
	if len(other.Imports) > 0 {
		c.Imports = append(c.Imports, other.Imports...)
	}
	if len(other.ImportAliases) > 0 {
		if c.ImportAliases == nil {
			c.ImportAliases = make(map[string]string, len(other.ImportAliases))
		}
		for k, v := range other.ImportAliases {
			c.ImportAliases[k] = v
		}
	}
	if len(other.Rules) > 0 {
		c.Rules = append(c.Rules, other.Rules...)
	}
	if len(other.Add) > 0 {
		c.Add = append(c.Add, other.Add...)
	}
}

// HasExternalModules returns true if external module instrumentation is configured
func (c *Config) HasExternalModules() bool {
	return len(c.ExternalModules) > 0
}

// GetExternalModules returns configured external modules to instrument
func (c *Config) GetExternalModules() []string {
	return c.ExternalModules
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
