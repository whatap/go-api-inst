package ast

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFramework(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		wantName   string
		wantNil    bool
	}{
		// Gin
		{"gin", "github.com/gin-gonic/gin", "gin", false},
		{"gin middleware skip", "github.com/gin-gonic/gin/binding", "", true},

		// Echo
		{"echo v4", "github.com/labstack/echo/v4", "echo", false},
		{"echo middleware skip", "github.com/labstack/echo/v4/middleware", "", true},

		// Fiber
		{"fiber v2", "github.com/gofiber/fiber/v2", "fiber", false},
		{"fiber middleware skip", "github.com/gofiber/fiber/v2/middleware/logger", "", true},

		// Chi
		{"chi v5", "github.com/go-chi/chi/v5", "chi", false},
		{"chi middleware skip", "github.com/go-chi/chi/v5/middleware", "", true},

		// Gorilla
		{"gorilla mux", "github.com/gorilla/mux", "gorilla", false},

		// FastHTTP
		{"fasthttp", "github.com/valyala/fasthttp", "fasthttp", false},

		// net/http
		{"net/http", "net/http", "nethttp", false},

		// database/sql
		{"database/sql", "database/sql", "sql", false},

		// GORM
		{"gorm", "gorm.io/gorm", "gorm", false},

		// Redigo
		{"redigo", "github.com/gomodule/redigo/redis", "redigo", false},

		// Sarama
		{"sarama IBM", "github.com/IBM/sarama", "sarama", false},
		{"sarama Shopify", "github.com/Shopify/sarama", "sarama", false},

		// gRPC
		{"grpc", "google.golang.org/grpc", "grpc", false},

		// Kubernetes
		{"k8s client", "k8s.io/client-go/kubernetes", "k8s", false},
		{"k8s rest", "k8s.io/client-go/rest", "k8srest", false},

		// Not a framework
		{"random package", "github.com/random/package", "", true},
		{"fmt", "fmt", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFramework(tt.importPath)
			if tt.wantNil {
				if got != nil {
					t.Errorf("detectFramework(%q) = %v, want nil", tt.importPath, got)
				}
				return
			}
			if got == nil {
				t.Errorf("detectFramework(%q) = nil, want %q", tt.importPath, tt.wantName)
				return
			}
			if got.Name != tt.wantName {
				t.Errorf("detectFramework(%q).Name = %q, want %q", tt.importPath, got.Name, tt.wantName)
			}
			if got.ImportPath != tt.importPath {
				t.Errorf("detectFramework(%q).ImportPath = %q, want %q", tt.importPath, got.ImportPath, tt.importPath)
			}
		})
	}
}

func TestIsVersionSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/v2", true},
		{"/v4", true},
		{"/v10", true},
		{"/v123", true},
		{"", false},
		{"/", false},
		{"/v", false},
		{"/va", false},
		{"/v2/middleware", false},
		{"v2", false},
		{"/x2", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isVersionSuffix(tt.input)
			if got != tt.want {
				t.Errorf("isVersionSuffix(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAnalyzeFile(t *testing.T) {
	// Create temp file for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "main.go")

	content := `package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

func main() {
	fmt.Println("Hello")
	r := gin.Default()
	r.Run(":8080")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewAnalyzer()
	result, err := analyzer.AnalyzeFile(testFile)
	if err != nil {
		t.Fatalf("AnalyzeFile() error = %v", err)
	}

	// Check package name
	if result.PackageName != "main" {
		t.Errorf("PackageName = %q, want %q", result.PackageName, "main")
	}

	// Check HasMain
	if !result.HasMain {
		t.Error("HasMain = false, want true")
	}

	// Check Imports
	expectedImports := []string{"fmt", "github.com/gin-gonic/gin"}
	if len(result.Imports) != len(expectedImports) {
		t.Errorf("len(Imports) = %d, want %d", len(result.Imports), len(expectedImports))
	}

	// Check Frameworks
	if len(result.Frameworks) != 1 {
		t.Errorf("len(Frameworks) = %d, want 1", len(result.Frameworks))
	} else if result.Frameworks[0].Name != "gin" {
		t.Errorf("Frameworks[0].Name = %q, want %q", result.Frameworks[0].Name, "gin")
	}

	// Check HasWhatapTrace
	if result.HasWhatapTrace {
		t.Error("HasWhatapTrace = true, want false")
	}
}

func TestAnalyzeFile_WithWhatap(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "main.go")

	content := `package main

import (
	"github.com/whatap/go-api/trace"
)

func main() {
	trace.Init(nil)
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewAnalyzer()
	result, err := analyzer.AnalyzeFile(testFile)
	if err != nil {
		t.Fatalf("AnalyzeFile() error = %v", err)
	}

	if !result.HasWhatapTrace {
		t.Error("HasWhatapTrace = false, want true")
	}
}

func TestAnalyzeFile_NoMain(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "helper.go")

	content := `package helper

func DoSomething() string {
	return "hello"
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewAnalyzer()
	result, err := analyzer.AnalyzeFile(testFile)
	if err != nil {
		t.Fatalf("AnalyzeFile() error = %v", err)
	}

	if result.HasMain {
		t.Error("HasMain = true, want false")
	}
}

func TestAnalyzeDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create main.go
	mainContent := `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.Run()
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create helper.go
	helperContent := `package main

func helper() string {
	return "helper"
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "helper.go"), []byte(helperContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create test file (should be skipped)
	testContent := `package main

import "testing"

func TestHelper(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "helper_test.go"), []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewAnalyzer()
	results, err := analyzer.AnalyzeDir(tmpDir)
	if err != nil {
		t.Fatalf("AnalyzeDir() error = %v", err)
	}

	// Should have 2 files (main.go, helper.go) - test file excluded
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2", len(results))
	}
}
