// Package report provides instrumentation report generation.
package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogLevel represents log verbosity level
type LogLevel int

const (
	LogQuiet   LogLevel = iota // Summary only
	LogNormal                  // Default (files being processed)
	LogVerbose                 // Detailed (transformation details)
	LogDebug                   // Debug (all information)
)

// FileStatus represents file processing status
type FileStatus string

const (
	StatusInstrumented FileStatus = "instrumented" // Instrumentation code injected
	StatusSkipped      FileStatus = "skipped"      // Skipped (no target, already instrumented, etc.)
	StatusCopied       FileStatus = "copied"       // Copied as-is (non-Go files)
	StatusError        FileStatus = "error"        // Error occurred
	StatusRemoved      FileStatus = "removed"      // Instrumentation code removed
)

// DiagnosticLevel represents diagnostic severity
type DiagnosticLevel string

const (
	DiagInfo    DiagnosticLevel = "info"
	DiagWarning DiagnosticLevel = "warning"
	DiagError   DiagnosticLevel = "error"
)

// Diagnostic represents a diagnostic message for a file
type Diagnostic struct {
	Level   DiagnosticLevel `json:"level"`
	Line    int             `json:"line,omitempty"`
	Message string          `json:"message"`
	Hint    string          `json:"hint,omitempty"`
}

// Dependency represents a go.mod dependency
type Dependency struct {
	Path        string `json:"path"`                  // e.g., github.com/gin-gonic/gin
	Version     string `json:"version"`               // e.g., v1.9.1
	Indirect    bool   `json:"indirect"`              // indirect dependency
	Supported   bool   `json:"supported"`             // whatap-go-inst support
	Transformer string `json:"transformer,omitempty"` // matching transformer name
}

// FileReport represents per-file processing result
type FileReport struct {
	Path         string       `json:"path"`
	Status       FileStatus   `json:"status"`
	Reason       string       `json:"reason,omitempty"`        // Skip/error reason
	Transformers []string     `json:"transformers,omitempty"`  // Applied transformers
	Changes      []string     `json:"changes,omitempty"`       // Change details
	Error        string       `json:"error,omitempty"`         // Error message
	Diagnostics  []Diagnostic `json:"diagnostics,omitempty"`   // Diagnostic messages
	SizeBytes    int          `json:"size_bytes,omitempty"`    // §240 source file size
	LineCount    int          `json:"line_count,omitempty"`    // §240 source line count
}

// Summary represents summary information
type Summary struct {
	Total                int            `json:"total"`
	Instrumented         int            `json:"instrumented"`
	Skipped              int            `json:"skipped"`
	Copied               int            `json:"copied"`
	Errors               int            `json:"errors"`
	Removed              int            `json:"removed"`
	Warnings             int            `json:"warnings"`
	SupportedLibraries   int            `json:"supported_libraries"`
	UnsupportedLibraries int            `json:"unsupported_libraries"`
	FragmentCount        int            `json:"fragment_count,omitempty"`  // §240 (§239 fragments merged by parent)
	SkipReasons          map[string]int `json:"skip_reasons,omitempty"`    // §240 reason → count
	ImportCfgFails       int            `json:"importcfg_fails,omitempty"` // §240 appendToImportCfg failures
}

// ConfigSnapshot captures the effective instrumentation configuration that
// produced this report. Stored so the report is reproducible — readers can
// see which packages the user opted in / excluded without having to chase
// down the original whatap.yaml. §240 / §242.
//
// EnabledPackages / DisabledPackages hold the user-supplied lists verbatim.
// Values are full Rule.Target package paths (e.g. "fmt",
// "github.com/gin-gonic/gin"). §242 replaced the former Name-based values.
type ConfigSnapshot struct {
	EnabledPackages  []string `json:"enabled_packages,omitempty"`
	DisabledPackages []string `json:"disabled_packages,omitempty"`
	ExternalModules  []string `json:"external_modules,omitempty"`
	ErrorTracking    bool     `json:"error_tracking,omitempty"`
	OutputDir        string   `json:"output_dir,omitempty"`
	CustomRuleCount  int      `json:"custom_rule_count,omitempty"` // len(cfg.Rules)
	ConfigPath       string   `json:"config_path,omitempty"`
}

// BuildOutcome records how the go build subprocess finished. §240.
type BuildOutcome struct {
	Success  bool   `json:"success"`
	ExitCode int    `json:"exit_code,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Environment captures the host runtime context so the report is reproducible
// without remote access. §240.
type Environment struct {
	GoVersion   string `json:"go_version,omitempty"`   // runtime.Version()
	GOOS        string `json:"goos,omitempty"`
	GOARCH      string `json:"goarch,omitempty"`
	CGOEnabled  string `json:"cgo_enabled,omitempty"`  // "1"/"0"/""
	VendorMode  bool   `json:"vendor_mode,omitempty"`
}

// WhatapInfo captures the CLI's own identification. §240.
type WhatapInfo struct {
	Version   string `json:"version,omitempty"`
	GitCommit string `json:"git_commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
}

// GoAPIInfo captures what go-api version the user's project is wired up to.
// §240. `Replaced` + `ReplacePath` surfaces dev-workflow replace directives.
type GoAPIInfo struct {
	Version     string `json:"version,omitempty"`
	Replaced    bool   `json:"replaced,omitempty"`
	ReplacePath string `json:"replace_path,omitempty"`
}

// ModuleInfo captures the user's go.mod metadata. §240.
type ModuleInfo struct {
	Path      string `json:"path,omitempty"`
	GoVersion string `json:"go_version,omitempty"`
}

// BuildInvocation captures how the user invoked whatap-go-inst + go build.
// §240.
type BuildInvocation struct {
	SubCmd    string   `json:"sub_cmd,omitempty"`
	BuildArgs []string `json:"build_args,omitempty"`
}

// Timings captures wall-clock seconds per build phase. §240.
// Matches the [TIMING] debug lines runFastBuild already prints to stderr.
type Timings struct {
	DepSetup   float64 `json:"dep_setup,omitempty"`
	PreResolve float64 `json:"pre_resolve,omitempty"`
	GoBuild    float64 `json:"go_build,omitempty"`
	Total      float64 `json:"total,omitempty"`
}

// PreResolveInfo surfaces the outcome of the whatap package pre-resolution
// step. Useful to diagnose importcfg patch failures or missing archives. §240.
type PreResolveInfo struct {
	ResolvedCount    int      `json:"resolved_count,omitempty"`
	ReplacedModules  []string `json:"replaced_modules,omitempty"`
	ExternalModules  []string `json:"external_modules,omitempty"` // resolved expansions
}

// Report represents the full report
type Report struct {
	Timestamp    string           `json:"timestamp"`
	Command      string           `json:"command"`
	SourceDir    string           `json:"source_dir,omitempty"`
	OutputDir    string           `json:"output_dir,omitempty"`
	Environment  *Environment     `json:"environment,omitempty"`
	Whatap       *WhatapInfo      `json:"whatap,omitempty"`
	GoAPI        *GoAPIInfo       `json:"go_api,omitempty"`
	Module       *ModuleInfo      `json:"module,omitempty"`
	Invocation   *BuildInvocation `json:"invocation,omitempty"`
	Config       *ConfigSnapshot  `json:"config,omitempty"`
	Build        *BuildOutcome    `json:"build,omitempty"`
	Timings      *Timings         `json:"timings,omitempty"`
	PreResolve   *PreResolveInfo  `json:"pre_resolve,omitempty"`
	Summary      Summary          `json:"summary"`
	Dependencies []Dependency     `json:"dependencies,omitempty"`
	Files        []FileReport     `json:"files"`

	mu       sync.Mutex   `json:"-"`
	logLevel LogLevel     `json:"-"`
	warnings []Diagnostic `json:"-"` // collected warnings for summary
}

// SetConfigSnapshot stores the effective config for the report. §240.
func (r *Report) SetConfigSnapshot(snap *ConfigSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Config = snap
}

// SetBuildOutcome records the outcome of the underlying build subprocess.
// Called by runFastBuild just before FinalizeReport so success and failure
// are both captured. §240.
func (r *Report) SetBuildOutcome(outcome *BuildOutcome) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Build = outcome
}

// SetEnvironment records the host runtime context. §240.
func (r *Report) SetEnvironment(env *Environment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Environment = env
}

// SetWhatap records whatap-go-inst CLI identification. §240.
func (r *Report) SetWhatap(w *WhatapInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Whatap = w
}

// SetGoAPI records the go-api version + replace information. §240.
func (r *Report) SetGoAPI(g *GoAPIInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.GoAPI = g
}

// SetModule records user project module metadata. §240.
func (r *Report) SetModule(m *ModuleInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Module = m
}

// SetInvocation records how the user invoked the CLI. §240.
func (r *Report) SetInvocation(inv *BuildInvocation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Invocation = inv
}

// SetTimings records wall-clock phase durations. §240.
func (r *Report) SetTimings(t *Timings) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Timings = t
}

// SetPreResolve records the outcome of the pre-resolve step. §240.
func (r *Report) SetPreResolve(pr *PreResolveInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.PreResolve = pr
}

// SetFragmentCount stores how many fragment JSONs the parent merged. §240.
func (r *Report) SetFragmentCount(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Summary.FragmentCount = n
}

// NewReport creates a new report
func NewReport(command string) *Report {
	return &Report{
		Timestamp:    time.Now().Format(time.RFC3339),
		Command:      command,
		Dependencies: make([]Dependency, 0),
		Files:        make([]FileReport, 0),
		warnings:     make([]Diagnostic, 0),
		logLevel:     LogNormal,
	}
}

// SetLogLevel sets the log level
func (r *Report) SetLogLevel(level LogLevel) {
	r.logLevel = level
}

// SetDirs sets source/output directories
func (r *Report) SetDirs(srcDir, outDir string) {
	r.SourceDir = srcDir
	r.OutputDir = outDir
}

// AddFile adds a file report
func (r *Report) AddFile(fr FileReport) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Files = append(r.Files, fr)

	// Update summary
	switch fr.Status {
	case StatusInstrumented:
		r.Summary.Instrumented++
	case StatusSkipped:
		r.Summary.Skipped++
		// §240: classify skip reasons so readers can see why files weren't
		// touched without scrolling through the full Files array.
		if fr.Reason != "" {
			if r.Summary.SkipReasons == nil {
				r.Summary.SkipReasons = make(map[string]int)
			}
			r.Summary.SkipReasons[fr.Reason]++
		}
	case StatusCopied:
		r.Summary.Copied++
	case StatusError:
		r.Summary.Errors++
	case StatusRemoved:
		r.Summary.Removed++
	}
	r.Summary.Total++

	// Count warnings from diagnostics
	for _, d := range fr.Diagnostics {
		if d.Level == DiagWarning {
			r.Summary.Warnings++
			r.warnings = append(r.warnings, Diagnostic{
				Level:   d.Level,
				Message: fmt.Sprintf("%s:%d - %s", fr.Path, d.Line, d.Message),
				Hint:    d.Hint,
			})
		}
	}

	// Log output
	r.logFile(fr)
}

// AddDependency adds a dependency to the report
func (r *Report) AddDependency(dep Dependency) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Dependencies = append(r.Dependencies, dep)
	if dep.Supported {
		r.Summary.SupportedLibraries++
	} else if !dep.Indirect {
		// Only count direct dependencies as unsupported
		r.Summary.UnsupportedLibraries++
	}
}

// AddWarning adds a standalone warning
func (r *Report) AddWarning(msg, hint string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Summary.Warnings++
	r.warnings = append(r.warnings, Diagnostic{
		Level:   DiagWarning,
		Message: msg,
		Hint:    hint,
	})
}

// logFile outputs file processing log
func (r *Report) logFile(fr FileReport) {
	switch r.logLevel {
	case LogQuiet:
		// Summary only, no individual file logs
		return

	case LogNormal:
		// Default: status and filename only
		switch fr.Status {
		case StatusInstrumented:
			fmt.Printf("✅ %s\n", fr.Path)
		case StatusError:
			fmt.Printf("❌ %s: %s\n", fr.Path, fr.Error)
		case StatusRemoved:
			fmt.Printf("🔄 %s\n", fr.Path)
		}

	case LogVerbose:
		// Detailed: includes transformation details
		switch fr.Status {
		case StatusInstrumented:
			fmt.Printf("✅ %s\n", fr.Path)
			if len(fr.Transformers) > 0 {
				fmt.Printf("   transformers: %v\n", fr.Transformers)
			}
			for _, change := range fr.Changes {
				fmt.Printf("   • %s\n", change)
			}
		case StatusSkipped:
			fmt.Printf("⏭️  %s (%s)\n", fr.Path, fr.Reason)
		case StatusError:
			fmt.Printf("❌ %s: %s\n", fr.Path, fr.Error)
		case StatusRemoved:
			fmt.Printf("🔄 %s\n", fr.Path)
			for _, change := range fr.Changes {
				fmt.Printf("   • %s\n", change)
			}
		case StatusCopied:
			fmt.Printf("📄 %s\n", fr.Path)
		}

	case LogDebug:
		// Debug: all information
		fmt.Printf("[%s] %s\n", fr.Status, fr.Path)
		if fr.Reason != "" {
			fmt.Printf("   reason: %s\n", fr.Reason)
		}
		if len(fr.Transformers) > 0 {
			fmt.Printf("   transformers: %v\n", fr.Transformers)
		}
		for _, change := range fr.Changes {
			fmt.Printf("   change: %s\n", change)
		}
		if fr.Error != "" {
			fmt.Printf("   error: %s\n", fr.Error)
		}
	}
}

// PrintSummary prints the summary
func (r *Report) PrintSummary() {
	fmt.Println()
	fmt.Println("─────────────────────────────────")
	fmt.Println("📊 Summary")
	fmt.Println("─────────────────────────────────")

	if r.Summary.Instrumented > 0 {
		fmt.Printf("   ✅ Instrumented: %d files\n", r.Summary.Instrumented)
	}
	if r.Summary.Removed > 0 {
		fmt.Printf("   🔄 Removed: %d files\n", r.Summary.Removed)
	}
	if r.Summary.Skipped > 0 {
		fmt.Printf("   ⏭️  Skipped: %d files\n", r.Summary.Skipped)
	}
	if r.Summary.Copied > 0 && r.logLevel >= LogVerbose {
		fmt.Printf("   📄 Copied: %d files\n", r.Summary.Copied)
	}
	if r.Summary.Warnings > 0 {
		fmt.Printf("   ⚠️  Warnings: %d\n", r.Summary.Warnings)
	}
	if r.Summary.Errors > 0 {
		fmt.Printf("   ❌ Errors: %d files\n", r.Summary.Errors)
	}
	fmt.Printf("   📁 Total: %d files\n", r.Summary.Total)
	fmt.Println("─────────────────────────────────")

	// Print dependencies if available
	r.printDependencies()

	// Print warnings if available
	r.printWarnings()
}

// printDependencies prints dependency information
func (r *Report) printDependencies() {
	if len(r.Dependencies) == 0 {
		return
	}

	// Only show in verbose mode or if there are supported libraries
	if r.logLevel < LogVerbose && r.Summary.SupportedLibraries == 0 {
		return
	}

	fmt.Println("─────────────────────────────────")
	fmt.Println("📦 Dependencies (go.mod)")
	fmt.Println("─────────────────────────────────")

	for _, dep := range r.Dependencies {
		if dep.Indirect && r.logLevel < LogVerbose {
			continue // Skip indirect deps in normal mode
		}

		if dep.Supported {
			fmt.Printf("   ✅ %s %s → %s\n", dep.Path, dep.Version, dep.Transformer)
		} else if r.logLevel >= LogVerbose {
			indirect := ""
			if dep.Indirect {
				indirect = " (indirect)"
			}
			fmt.Printf("   ⬜ %s %s%s\n", dep.Path, dep.Version, indirect)
		}
	}
	fmt.Println("─────────────────────────────────")
}

// printWarnings prints collected warnings
func (r *Report) printWarnings() {
	if len(r.warnings) == 0 || r.logLevel < LogVerbose {
		return
	}

	fmt.Println("─────────────────────────────────")
	fmt.Println("⚠️  Warnings")
	fmt.Println("─────────────────────────────────")

	for _, w := range r.warnings {
		fmt.Printf("   %s\n", w.Message)
		if w.Hint != "" && r.logLevel >= LogDebug {
			fmt.Printf("      💡 %s\n", w.Hint)
		}
	}
	fmt.Println("─────────────────────────────────")
}

// SaveJSON saves the report to a JSON file
func (r *Report) SaveJSON(path string) error {
	if err := r.SaveJSONQuiet(path); err != nil {
		return err
	}
	fmt.Printf("📄 Report saved: %s\n", path)
	return nil
}

// SaveJSONQuiet saves the report to a JSON file without printing to stdout.
// §239: toolexec child processes use this to avoid polluting compiler output.
func (r *Report) SaveJSONQuiet(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}

// MergeFragment merges another report's per-file records into this report.
// §239: Used by the parent `runFastBuild` process to aggregate fragment JSONs
// produced by toolexec child processes. Only Files and counter-style Summary
// fields are merged — Dependencies / Command / Timestamp / SourceDir / OutputDir
// stay as the parent's authoritative values (SupportedLibraries / UnsupportedLibraries
// are owned by the parent's dependency load).
func (r *Report) MergeFragment(frag *Report) {
	if frag == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Files = append(r.Files, frag.Files...)
	r.Summary.Total += frag.Summary.Total
	r.Summary.Instrumented += frag.Summary.Instrumented
	r.Summary.Skipped += frag.Summary.Skipped
	r.Summary.Copied += frag.Summary.Copied
	r.Summary.Errors += frag.Summary.Errors
	r.Summary.Removed += frag.Summary.Removed
	r.Summary.Warnings += frag.Summary.Warnings
	r.Summary.ImportCfgFails += frag.Summary.ImportCfgFails
	// §240: merge skip reasons. Null-safe — child fragments may omit the map.
	for reason, n := range frag.Summary.SkipReasons {
		if r.Summary.SkipReasons == nil {
			r.Summary.SkipReasons = make(map[string]int)
		}
		r.Summary.SkipReasons[reason] += n
	}
}

// IncImportCfgFail atomically bumps the importcfg-append failure counter. §240.
func (r *Report) IncImportCfgFail() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Summary.ImportCfgFails++
}

// LoadJSONReport reads a report JSON file and returns a parsed Report.
// §239: Used by the parent to read toolexec child fragment files.
func LoadJSONReport(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Global report instance
var globalReport *Report

// Init initializes the global report
func Init(command string) {
	globalReport = NewReport(command)
}

// Get returns the global report
func Get() *Report {
	if globalReport == nil {
		globalReport = NewReport("unknown")
	}
	return globalReport
}

// SetLevel sets the global log level
func SetLevel(level LogLevel) {
	Get().SetLogLevel(level)
}

// TransformerInfo holds transformer information for dependency matching.
// §242 removed the former Name field — ImportPath is the single identifier
// (e.g. "github.com/gin-gonic/gin", "fmt").
type TransformerInfo struct {
	ImportPath        string
	SupportedVersions []string // §148: supported major versions (nil = no filtering)
}

// transformerEntry holds transformer info for dependency matching
type transformerEntry struct {
	supportedVersions []string
}

// LoadDependencies parses go.mod and matches against supported transformers
func (r *Report) LoadDependencies(goModPath string, transformers []TransformerInfo) error {
	file, err := os.Open(goModPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Build import path to transformer entry map
	supportedPaths := make(map[string]transformerEntry)
	for _, t := range transformers {
		supportedPaths[t.ImportPath] = transformerEntry{
			supportedVersions: t.SupportedVersions,
		}
	}

	scanner := bufio.NewScanner(file)
	inRequire := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Handle require block
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if line == ")" {
			inRequire = false
			continue
		}

		// Parse dependency line
		if inRequire || strings.HasPrefix(line, "require ") {
			dep := parseDependencyLine(line, supportedPaths)
			if dep != nil {
				r.AddDependency(*dep)
			}
		}
	}

	return scanner.Err()
}

// parseDependencyLine parses a single dependency line from go.mod
func parseDependencyLine(line string, supportedPaths map[string]transformerEntry) *Dependency {
	// Remove "require " prefix if present
	line = strings.TrimPrefix(line, "require ")
	line = strings.TrimSpace(line)

	// Skip empty or comment lines
	if line == "" || strings.HasPrefix(line, "//") || line == "(" || line == ")" {
		return nil
	}

	// Check for indirect
	indirect := strings.Contains(line, "// indirect")
	line = strings.Split(line, "//")[0]
	line = strings.TrimSpace(line)

	// Split into path and version
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil
	}

	path := parts[0]
	version := parts[1]

	// Skip whatap packages (our own libraries)
	if strings.HasPrefix(path, "github.com/whatap/") {
		return nil
	}

	// Check if supported
	transformer, supported := matchTransformer(path, supportedPaths)

	return &Dependency{
		Path:        path,
		Version:     version,
		Indirect:    indirect,
		Supported:   supported,
		Transformer: transformer,
	}
}

// isVersionSuffix checks if a string is a Go module version suffix (v2, v3, etc.)
func isVersionSuffix(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// extractVersionFromPath extracts the version suffix from a dependency path.
// e.g., "github.com/labstack/echo/v4" → "v4", "github.com/gin-gonic/gin" → ""
func extractVersionFromPath(depPath string) string {
	parts := strings.Split(depPath, "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if isVersionSuffix(last) {
			return last
		}
	}
	return ""
}

// matchTransformer checks if a dependency path matches any supported
// transformer. Returns the matched supported package path (e.g.
// "github.com/gin-gonic/gin") when matched — §242 replaced the former Name
// identifier with the full import path so the report's `transformer` field
// ties directly to user yaml's `enabled_packages` / `disabled_packages`.
func matchTransformer(depPath string, supportedPaths map[string]transformerEntry) (string, bool) {
	// Direct match
	if entry, ok := supportedPaths[depPath]; ok {
		// §148: Check version filtering
		if len(entry.supportedVersions) > 0 {
			depVersion := extractVersionFromPath(depPath)
			if !containsString(entry.supportedVersions, depVersion) {
				return "", false
			}
		}
		return depPath, true
	}

	// Check if the dependency is a sub-package of a supported path
	for supportedPath, entry := range supportedPaths {
		if strings.HasPrefix(depPath, supportedPath+"/") {
			// §148: Check version filtering for prefix match
			if len(entry.supportedVersions) > 0 {
				// Extract the segment right after the prefix
				suffix := strings.TrimPrefix(depPath, supportedPath+"/")
				firstSeg := suffix
				if idx := strings.Index(suffix, "/"); idx >= 0 {
					firstSeg = suffix[:idx]
				}
				// Determine the version
				depVersion := ""
				if isVersionSuffix(firstSeg) {
					depVersion = firstSeg
				}
				if !containsString(entry.supportedVersions, depVersion) {
					continue
				}
			}
			return supportedPath, true
		}
		// Also check if supported path is a sub-package (e.g., go-redis/redis/v9)
		if strings.HasPrefix(supportedPath, depPath+"/") {
			return supportedPath, true
		}
	}

	return "", false
}

// containsString checks if a string slice contains a specific string
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// LoadDependenciesFromDir loads dependencies from go.mod in the given directory
func (r *Report) LoadDependenciesFromDir(dir string, transformers []TransformerInfo) error {
	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return nil // No go.mod, skip silently
	}
	return r.LoadDependencies(goModPath, transformers)
}
