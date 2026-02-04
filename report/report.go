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
}

// Summary represents summary information
type Summary struct {
	Total                int `json:"total"`
	Instrumented         int `json:"instrumented"`
	Skipped              int `json:"skipped"`
	Copied               int `json:"copied"`
	Errors               int `json:"errors"`
	Removed              int `json:"removed"`
	Warnings             int `json:"warnings"`
	SupportedLibraries   int `json:"supported_libraries"`
	UnsupportedLibraries int `json:"unsupported_libraries"`
}

// Report represents the full report
type Report struct {
	Timestamp    string       `json:"timestamp"`
	Command      string       `json:"command"`
	SourceDir    string       `json:"source_dir,omitempty"`
	OutputDir    string       `json:"output_dir,omitempty"`
	Summary      Summary      `json:"summary"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
	Files        []FileReport `json:"files"`

	mu       sync.Mutex   `json:"-"`
	logLevel LogLevel     `json:"-"`
	warnings []Diagnostic `json:"-"` // collected warnings for summary
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
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	fmt.Printf("📄 Report saved: %s\n", path)
	return nil
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

// TransformerInfo holds transformer information for dependency matching
type TransformerInfo struct {
	Name       string
	ImportPath string
}

// LoadDependencies parses go.mod and matches against supported transformers
func (r *Report) LoadDependencies(goModPath string, transformers []TransformerInfo) error {
	file, err := os.Open(goModPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Build import path to transformer name map
	supportedPaths := make(map[string]string)
	for _, t := range transformers {
		supportedPaths[t.ImportPath] = t.Name
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
func parseDependencyLine(line string, supportedPaths map[string]string) *Dependency {
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

// matchTransformer checks if a dependency path matches any supported transformer
func matchTransformer(depPath string, supportedPaths map[string]string) (string, bool) {
	// Direct match
	if name, ok := supportedPaths[depPath]; ok {
		return name, true
	}

	// Check if the dependency is a sub-package of a supported path
	for supportedPath, name := range supportedPaths {
		if strings.HasPrefix(depPath, supportedPath+"/") {
			return name, true
		}
		// Also check if supported path is a sub-package (e.g., go-redis/redis/v9)
		if strings.HasPrefix(supportedPath, depPath+"/") {
			return name, true
		}
	}

	return "", false
}

// LoadDependenciesFromDir loads dependencies from go.mod in the given directory
func (r *Report) LoadDependenciesFromDir(dir string, transformers []TransformerInfo) error {
	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return nil // No go.mod, skip silently
	}
	return r.LoadDependencies(goModPath, transformers)
}
