package ast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveFile_Gin(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	// Injected Gin code
	content := `package main

import (
	"github.com/gin-gonic/gin"
	"github.com/whatap/go-api/trace"
	"github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
)

func main() {
	trace.Init(nil)
	defer trace.Shutdown()
	r := gin.Default()
	r.Use(whatapgin.Middleware())
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "hello"})
	})
	r.Run(":8080")
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	remover := NewRemover(false)
	if err := remover.RemoveFile(srcFile, dstFile); err != nil {
		t.Fatalf("RemoveFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check whatap imports removed
	if strings.Contains(outputStr, "whatap/go-api") {
		t.Error("Output should not contain whatap imports")
	}

	// Check trace.Init removed
	if strings.Contains(outputStr, "trace.Init") {
		t.Error("Output should not contain trace.Init")
	}

	// Check trace.Shutdown removed
	if strings.Contains(outputStr, "trace.Shutdown") {
		t.Error("Output should not contain trace.Shutdown")
	}

	// Check middleware removed
	if strings.Contains(outputStr, "whatapgin") {
		t.Error("Output should not contain whatapgin")
	}

	// Check Gin is still there
	if !strings.Contains(outputStr, "gin.Default()") {
		t.Error("Output should still contain gin.Default()")
	}
}

func TestRemoveFile_NetHttp(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	// Injected net/http code
	content := `package main

import (
	"fmt"
	"net/http"
	"github.com/whatap/go-api/trace"
	"github.com/whatap/go-api/instrumentation/net/http/whataphttp"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello")
}

func main() {
	trace.Init(nil)
	defer trace.Shutdown()
	http.HandleFunc("/", whataphttp.Func(handler))
	http.ListenAndServe(":8080", nil)
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	remover := NewRemover(false)
	if err := remover.RemoveFile(srcFile, dstFile); err != nil {
		t.Fatalf("RemoveFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check whatap imports removed
	if strings.Contains(outputStr, "whatap/go-api") {
		t.Error("Output should not contain whatap imports")
	}

	// Check whataphttp.Func unwrapped
	if strings.Contains(outputStr, "whataphttp.Func") {
		t.Error("Output should not contain whataphttp.Func")
	}

	// Check handler is now direct
	if !strings.Contains(outputStr, `HandleFunc("/", handler)`) {
		t.Error("Output should contain HandleFunc(\"/\", handler)")
	}
}

func TestRemoveFile_Sql(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	// Injected SQL code
	content := `package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/whatap/go-api/trace"
	"github.com/whatap/go-api/instrumentation/database/sql/whatapsql"
)

func main() {
	trace.Init(nil)
	defer trace.Shutdown()
	db, _ := whatapsql.Open("mysql", "user:pass@/dbname")
	defer db.Close()
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	remover := NewRemover(false)
	if err := remover.RemoveFile(srcFile, dstFile); err != nil {
		t.Fatalf("RemoveFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check whatap imports removed
	if strings.Contains(outputStr, "whatap/go-api") {
		t.Error("Output should not contain whatap imports")
	}

	// Check whatapsql.Open -> sql.Open
	if strings.Contains(outputStr, "whatapsql.Open") {
		t.Error("Output should not contain whatapsql.Open")
	}

	if !strings.Contains(outputStr, "sql.Open") {
		t.Error("Output should contain sql.Open")
	}
}

func TestRemoveFile_NoWhatap(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	// Clean code without whatap
	content := `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.Run()
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	remover := NewRemover(false)
	if err := remover.RemoveFile(srcFile, dstFile); err != nil {
		t.Fatalf("RemoveFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	// Should be identical (just copied)
	if string(output) != content {
		t.Error("Clean file should be copied without changes")
	}
}

func TestRemoveFile_Chi(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	// Injected Chi code (Middleware as function value, not call)
	content := `package main

import (
	"net/http"
	"github.com/go-chi/chi/v5"
	"github.com/whatap/go-api/trace"
	"github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi"
)

func main() {
	trace.Init(nil)
	defer trace.Shutdown()
	r := chi.NewRouter()
	r.Use(whatapchi.Middleware)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})
	http.ListenAndServe(":8080", r)
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	remover := NewRemover(false)
	if err := remover.RemoveFile(srcFile, dstFile); err != nil {
		t.Fatalf("RemoveFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check whatap imports removed
	if strings.Contains(outputStr, "whatap/go-api") {
		t.Error("Output should not contain whatap imports")
	}

	// Check middleware removed (Chi uses function value)
	if strings.Contains(outputStr, "whatapchi") {
		t.Error("Output should not contain whatapchi")
	}

	// Check Chi is still there
	if !strings.Contains(outputStr, "chi.NewRouter()") {
		t.Error("Output should still contain chi.NewRouter()")
	}
}

func TestRemoveDir(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create injected main.go
	mainContent := `package main

import (
	"github.com/gin-gonic/gin"
	"github.com/whatap/go-api/trace"
	"github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
)

func main() {
	trace.Init(nil)
	defer trace.Shutdown()
	r := gin.Default()
	r.Use(whatapgin.Middleware())
	r.Run()
}
`
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.mod
	goMod := `module test-app

go 1.21
`
	if err := os.WriteFile(filepath.Join(srcDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	remover := NewRemover(false)
	if err := remover.RemoveDir(srcDir, dstDir); err != nil {
		t.Fatalf("RemoveDir() error = %v", err)
	}

	// Check main.go was cleaned
	outputMain, err := os.ReadFile(filepath.Join(dstDir, "main.go"))
	if err != nil {
		t.Fatalf("Failed to read output main.go: %v", err)
	}
	if strings.Contains(string(outputMain), "whatap") {
		t.Error("main.go should not contain whatap")
	}

	// Check go.mod was copied
	outputMod, err := os.ReadFile(filepath.Join(dstDir, "go.mod"))
	if err != nil {
		t.Fatalf("Failed to read output go.mod: %v", err)
	}
	if string(outputMod) != goMod {
		t.Error("go.mod should be copied unchanged")
	}
}

func TestInjectThenRemove_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	injectedFile := filepath.Join(tmpDir, "injected", "main.go")
	cleanedFile := filepath.Join(tmpDir, "cleaned", "main.go")

	// Original clean code
	original := `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "hello"})
	})
	r.Run(":8080")
}
`
	if err := os.WriteFile(srcFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	// Inject
	injector := NewInjector()
	if err := injector.InjectFile(srcFile, injectedFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	// Verify injection happened
	injectedContent, err := os.ReadFile(injectedFile)
	if err != nil {
		t.Fatalf("Failed to read injected file: %v", err)
	}
	if !strings.Contains(string(injectedContent), "whatapgin") {
		t.Error("Injected file should contain whatapgin")
	}

	// Remove
	remover := NewRemover(false)
	if err := remover.RemoveFile(injectedFile, cleanedFile); err != nil {
		t.Fatalf("RemoveFile() error = %v", err)
	}

	// Read cleaned file
	cleanedContent, err := os.ReadFile(cleanedFile)
	if err != nil {
		t.Fatalf("Failed to read cleaned file: %v", err)
	}

	// Verify no whatap remains
	if strings.Contains(string(cleanedContent), "whatap") {
		t.Error("Cleaned file should not contain whatap")
	}

	// Verify core structure preserved
	if !strings.Contains(string(cleanedContent), "gin.Default()") {
		t.Error("Cleaned file should contain gin.Default()")
	}
	if !strings.Contains(string(cleanedContent), `r.Run(":8080")`) {
		t.Error("Cleaned file should contain r.Run(\":8080\")")
	}
}
