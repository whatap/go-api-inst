// Package report provides instrumentation report generation.
package report

import (
	"encoding/json"
	"fmt"
	"os"
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

// FileReport represents per-file processing result
type FileReport struct {
	Path         string     `json:"path"`
	Status       FileStatus `json:"status"`
	Reason       string     `json:"reason,omitempty"`        // Skip/error reason
	Transformers []string   `json:"transformers,omitempty"`  // Applied transformers
	Changes      []string   `json:"changes,omitempty"`       // Change details
	Error        string     `json:"error,omitempty"`         // Error message
}

// Summary represents summary information
type Summary struct {
	Total        int `json:"total"`
	Instrumented int `json:"instrumented"`
	Skipped      int `json:"skipped"`
	Copied       int `json:"copied"`
	Errors       int `json:"errors"`
	Removed      int `json:"removed"`
}

// Report represents the full report
type Report struct {
	Timestamp string       `json:"timestamp"`
	Command   string       `json:"command"`
	SourceDir string       `json:"source_dir,omitempty"`
	OutputDir string       `json:"output_dir,omitempty"`
	Summary   Summary      `json:"summary"`
	Files     []FileReport `json:"files"`

	mu       sync.Mutex
	logLevel LogLevel
}

// NewReport creates a new report
func NewReport(command string) *Report {
	return &Report{
		Timestamp: time.Now().Format(time.RFC3339),
		Command:   command,
		Files:     make([]FileReport, 0),
		logLevel:  LogNormal,
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

	// Log output
	r.logFile(fr)
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
	if r.Summary.Errors > 0 {
		fmt.Printf("   ❌ Errors: %d files\n", r.Summary.Errors)
	}
	fmt.Printf("   📁 Total: %d files\n", r.Summary.Total)
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
