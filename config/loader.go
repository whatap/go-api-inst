package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config file search order (only within .whatap/ directory)
// 1. .whatap/config.yaml (recommended)
// 2. .whatap/whatap.yaml (alternative)
var configSearchPaths = []string{
	".whatap/config.yaml",
	".whatap/whatap.yaml",
}

// Loader loads Config
type Loader struct {
	// ConfigPath is the path specified via --config flag
	ConfigPath string

	// ProjectDir is the project directory (go.mod location)
	// For inject: srcDir, for go build: search from build target
	// If not set, search go.mod from cwd
	ProjectDir string

	// CLIFlags are options passed from CLI
	CLIFlags *CLIFlags
}

// CLIFlags are CLI options
type CLIFlags struct {
	ErrorTracking *bool   // nil if not specified
	Debug         *bool   // nil if not specified
	OutputDir     *string // nil if not specified
}

// NewLoader creates a Loader
func NewLoader() *Loader {
	return &Loader{
		CLIFlags: &CLIFlags{},
	}
}

// Load loads configuration (priority: CLI > env vars > config file > defaults)
func (l *Loader) Load() (*Config, error) {
	// 1. Defaults
	cfg := NewConfig()

	// 2. Load config file
	fileCfg, err := l.loadFromFile()
	if err != nil {
		return nil, err
	}
	if fileCfg != nil {
		cfg.Merge(fileCfg)
	}

	// 3. Set BaseDir (even if no config file)
	// Priority: BaseDir from config > go.mod location > ProjectDir > cwd
	if cfg.BaseDir == "" {
		cfg.BaseDir = l.computeProjectBaseDir()
	}

	// 4. Apply environment variables
	l.applyEnvVars(cfg)

	// 5. Apply CLI options (highest priority)
	l.applyCLIFlags(cfg)

	return cfg, nil
}

// computeProjectBaseDir calculates BaseDir based on go.mod
func (l *Loader) computeProjectBaseDir() string {
	startDir := l.ProjectDir
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return ""
		}
	}

	// Find go.mod location
	goModDir := FindGoModDir(startDir)
	if goModDir != "" {
		return goModDir
	}

	// Return start directory if no go.mod
	absPath, _ := filepath.Abs(startDir)
	return absPath
}

// loadFromFile loads from config file
func (l *Loader) loadFromFile() (*Config, error) {
	var configPath string

	// 1. --config flag
	if l.ConfigPath != "" {
		configPath = l.ConfigPath
	} else {
		// 2. WHATAP_INST_CONFIG environment variable
		envPath := os.Getenv("WHATAP_INST_CONFIG")
		if envPath != "" {
			configPath = envPath
		} else {
			// 3. Default search paths
			configPath = l.findConfigFile()
		}
	}

	if configPath == "" {
		return nil, nil // No config file (not an error)
	}

	cfg, err := l.parseYAMLFile(configPath)
	if err != nil {
		return nil, err
	}

	// Set BaseDir: base for all relative paths
	// If config file is in .whatap/, use parent of .whatap/
	// e.g., /myapp/.whatap/config.yaml → BaseDir = /myapp/
	cfg.BaseDir = l.computeBaseDir(configPath)

	return cfg, nil
}

// computeBaseDir calculates BaseDir from config file path
func (l *Loader) computeBaseDir(configPath string) string {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return ""
	}

	// Directory of config file
	configDir := filepath.Dir(absPath)

	// If in .whatap/, return parent directory
	if filepath.Base(configDir) == ".whatap" {
		return filepath.Dir(configDir)
	}

	// Otherwise return config file directory
	return configDir
}

// findConfigFile searches for config file based on go.mod
// Search order:
// 1. If ProjectDir is set: find go.mod from ProjectDir, use that directory
// 2. If no ProjectDir: find go.mod from cwd, use that directory
// 3. If no go.mod: use cwd
func (l *Loader) findConfigFile() string {
	// Determine search start directory
	startDir := l.ProjectDir
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return ""
		}
	}

	// Find go.mod location (project root)
	projectRoot := FindGoModDir(startDir)
	if projectRoot == "" {
		// Use start directory if no go.mod
		projectRoot = startDir
	}

	// Search for config file from project root
	for _, path := range configSearchPaths {
		fullPath := filepath.Join(projectRoot, path)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}

	return ""
}

// FindGoModDir searches upward from given path to find go.mod
// Returns directory path containing go.mod, or empty string if not found
func FindGoModDir(startPath string) string {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return ""
	}

	// Convert to directory if file
	info, err := os.Stat(absPath)
	if err != nil {
		return ""
	}
	if !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}

	dir := absPath
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, no go.mod
			return ""
		}
		dir = parent
	}
}

// parseYAMLFile parses a YAML file
func (l *Loader) parseYAMLFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyEnvVars applies environment variables
func (l *Loader) applyEnvVars(cfg *Config) {
	// GO_API_AST_DEBUG
	if os.Getenv("GO_API_AST_DEBUG") != "" {
		cfg.Instrumentation.Debug = true
	}

	// GO_API_AST_OUTPUT_DIR
	if dir := os.Getenv("GO_API_AST_OUTPUT_DIR"); dir != "" {
		cfg.Instrumentation.OutputDir = dir
	}
}

// applyCLIFlags applies CLI options (highest priority)
func (l *Loader) applyCLIFlags(cfg *Config) {
	if l.CLIFlags == nil {
		return
	}

	if l.CLIFlags.ErrorTracking != nil {
		cfg.Instrumentation.ErrorTracking = *l.CLIFlags.ErrorTracking
	}
	if l.CLIFlags.Debug != nil {
		cfg.Instrumentation.Debug = *l.CLIFlags.Debug
	}
	if l.CLIFlags.OutputDir != nil {
		cfg.Instrumentation.OutputDir = *l.CLIFlags.OutputDir
	}
}

// GetConfigPath returns the config file path used (for debugging)
func (l *Loader) GetConfigPath() string {
	if l.ConfigPath != "" {
		return l.ConfigPath
	}
	if envPath := os.Getenv("WHATAP_INST_CONFIG"); envPath != "" {
		return envPath
	}
	return l.findConfigFile()
}
