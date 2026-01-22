package ast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Framework represents detected framework information
type Framework struct {
	Name       string // gin, echo, fiber, chi, gorilla, fasthttp, nethttp, sql
	ImportPath string // github.com/gin-gonic/gin
	VarName    string // router variable name (r, e, app, etc.)
}

// AnalysisResult represents source code analysis result
type AnalysisResult struct {
	FilePath       string
	PackageName    string
	HasMain        bool // whether main() function exists
	Frameworks     []Framework
	Imports        []string
	HasWhatapTrace bool // whether whatap/go-api is already imported
}

// Analyzer analyzes Go source code
type Analyzer struct {
	fset *token.FileSet
}

// NewAnalyzer creates a new analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		fset: token.NewFileSet(),
	}
}

// AnalyzeFile analyzes a single file
func (a *Analyzer) AnalyzeFile(filePath string) (*AnalysisResult, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	file, err := parser.ParseFile(a.fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	result := &AnalysisResult{
		FilePath:    filePath,
		PackageName: file.Name.Name,
	}

	// Analyze imports
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		result.Imports = append(result.Imports, importPath)

		// Detect whatap
		if strings.Contains(importPath, "whatap/go-api") {
			result.HasWhatapTrace = true
		}

		// Detect framework
		if fw := detectFramework(importPath); fw != nil {
			result.Frameworks = append(result.Frameworks, *fw)
		}
	}

	// Detect main() function
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Name.Name == "main" && fn.Recv == nil {
				result.HasMain = true
			}
		}
		return true
	})

	return result, nil
}

// AnalyzeDir analyzes all Go files in a directory
func (a *Analyzer) AnalyzeDir(dirPath string) ([]*AnalysisResult, error) {
	var results []*AnalysisResult

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Process only .go files
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			// Exclude _test.go
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}

			result, err := a.AnalyzeFile(path)
			if err != nil {
				return err
			}
			results = append(results, result)
		}

		return nil
	})

	return results, err
}

// detectFramework detects framework from import path
// Only detects main framework packages (excludes sub-packages like middleware)
func detectFramework(importPath string) *Framework {
	// Exact pattern matching (main packages only)
	// e.g., github.com/gin-gonic/gin or github.com/gin-gonic/gin/v2
	frameworks := []struct {
		prefix string
		name   string
	}{
		{"github.com/gin-gonic/gin", "gin"},
		{"github.com/labstack/echo", "echo"},
		{"github.com/gofiber/fiber", "fiber"},
		{"github.com/go-chi/chi", "chi"},
		{"github.com/gorilla/mux", "gorilla"},
		{"github.com/valyala/fasthttp", "fasthttp"},
	}

	for _, fw := range frameworks {
		if strings.HasPrefix(importPath, fw.prefix) {
			// Exclude sub-packages (middleware, logger, etc.)
			// github.com/gofiber/fiber/v2 -> OK
			// github.com/gofiber/fiber/v2/middleware/logger -> Skip
			remaining := strings.TrimPrefix(importPath, fw.prefix)

			// Empty string or only version suffix (/v2, /v3, etc.)
			if remaining == "" || isVersionSuffix(remaining) {
				return &Framework{
					Name:       fw.name,
					ImportPath: importPath,
				}
			}
		}
	}

	// Detect net/http
	if importPath == "net/http" {
		return &Framework{
			Name:       "nethttp",
			ImportPath: importPath,
		}
	}

	// Detect database/sql
	if importPath == "database/sql" {
		return &Framework{
			Name:       "sql",
			ImportPath: importPath,
		}
	}

	// Detect gorm.io/gorm (new version)
	if importPath == "gorm.io/gorm" {
		return &Framework{
			Name:       "gorm",
			ImportPath: importPath,
		}
	}

	// Detect github.com/jinzhu/gorm (old version)
	if importPath == "github.com/jinzhu/gorm" {
		return &Framework{
			Name:       "jinzhugorm",
			ImportPath: importPath,
		}
	}

	// Detect github.com/gomodule/redigo/redis
	if importPath == "github.com/gomodule/redigo/redis" {
		return &Framework{
			Name:       "redigo",
			ImportPath: importPath,
		}
	}

	// Detect github.com/IBM/sarama (Kafka)
	if importPath == "github.com/IBM/sarama" || importPath == "github.com/Shopify/sarama" {
		return &Framework{
			Name:       "sarama",
			ImportPath: importPath,
		}
	}

	// Detect google.golang.org/grpc
	if importPath == "google.golang.org/grpc" {
		return &Framework{
			Name:       "grpc",
			ImportPath: importPath,
		}
	}

	// Detect k8s.io/client-go/kubernetes
	if importPath == "k8s.io/client-go/kubernetes" {
		return &Framework{
			Name:       "k8s",
			ImportPath: importPath,
		}
	}

	// Detect k8s.io/client-go/rest (uses InClusterConfig)
	if importPath == "k8s.io/client-go/rest" {
		return &Framework{
			Name:       "k8srest",
			ImportPath: importPath,
		}
	}

	// Detect github.com/jmoiron/sqlx
	if importPath == "github.com/jmoiron/sqlx" {
		return &Framework{
			Name:       "sqlx",
			ImportPath: importPath,
		}
	}

	// Detect github.com/redis/go-redis/v9 (new path)
	if strings.HasPrefix(importPath, "github.com/redis/go-redis") {
		remaining := strings.TrimPrefix(importPath, "github.com/redis/go-redis")
		if remaining == "" || isVersionSuffix(remaining) {
			return &Framework{
				Name:       "goredis",
				ImportPath: importPath,
			}
		}
	}

	// Detect github.com/go-redis/redis (old path)
	if strings.HasPrefix(importPath, "github.com/go-redis/redis") {
		remaining := strings.TrimPrefix(importPath, "github.com/go-redis/redis")
		if remaining == "" || isVersionSuffix(remaining) {
			return &Framework{
				Name:       "goredis",
				ImportPath: importPath,
			}
		}
	}

	// Detect go.mongodb.org/mongo-driver/mongo
	if importPath == "go.mongodb.org/mongo-driver/mongo" {
		return &Framework{
			Name:       "mongo",
			ImportPath: importPath,
		}
	}

	// Detect go.mongodb.org/mongo-driver/v2/mongo (v2)
	if importPath == "go.mongodb.org/mongo-driver/v2/mongo" {
		return &Framework{
			Name:       "mongo",
			ImportPath: importPath,
		}
	}

	return nil
}

// isVersionSuffix checks if the string is a version suffix (/v2, /v3, etc.)
func isVersionSuffix(s string) bool {
	if len(s) < 2 {
		return false
	}
	if s[0] != '/' {
		return false
	}
	if s[1] != 'v' {
		return false
	}
	// Only digits should follow /v
	for i := 2; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 2 // At least one digit must follow /v
}
