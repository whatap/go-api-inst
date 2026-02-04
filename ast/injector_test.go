package ast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/whatap/go-api-inst/ast/common"
)

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"github.com/labstack/echo/v4", "v4"},
		{"github.com/gofiber/fiber/v2", "v2"},
		{"github.com/go-chi/chi/v5", "v5"},
		{"github.com/gin-gonic/gin", ""},
		{"github.com/gorilla/mux", ""},
		{"net/http", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := common.ExtractVersion(tt.input)
			if got != tt.want {
				t.Errorf("ExtractVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInjectFile_Gin(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "hello"})
	})
	r.Run(":8080")
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	// Read output file
	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check trace import
	if !strings.Contains(outputStr, `"github.com/whatap/go-api/trace"`) {
		t.Error("Output should contain trace import")
	}

	// Check whatapgin import
	if !strings.Contains(outputStr, "whatapgin") {
		t.Error("Output should contain whatapgin import")
	}

	// Check trace.Init
	if !strings.Contains(outputStr, "trace.Init(nil)") {
		t.Error("Output should contain trace.Init(nil)")
	}

	// Check trace.Shutdown
	if !strings.Contains(outputStr, "trace.Shutdown()") {
		t.Error("Output should contain trace.Shutdown()")
	}

	// Check middleware
	if !strings.Contains(outputStr, "whatapgin.Middleware()") {
		t.Error("Output should contain whatapgin.Middleware()")
	}
}

func TestInjectFile_NetHttp(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"fmt"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello")
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check whataphttp import
	if !strings.Contains(outputStr, "whataphttp") {
		t.Error("Output should contain whataphttp import")
	}

	// Check whataphttp.Func wrapping
	if !strings.Contains(outputStr, "whataphttp.Func(handler)") {
		t.Error("Output should contain whataphttp.Func(handler)")
	}
}

func TestInjectFile_Sql(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, _ := sql.Open("mysql", "user:pass@/dbname")
	defer db.Close()
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check whatapsql import
	if !strings.Contains(outputStr, "whatapsql") {
		t.Error("Output should contain whatapsql import")
	}

	// Check sql.Open -> whatapsql.Open
	if !strings.Contains(outputStr, "whatapsql.Open") {
		t.Error("Output should contain whatapsql.Open")
	}
}

func TestInjectFile_AlreadyInjected(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	// Already has whatap imports
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
	r.Run(":8080")
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	// Should be identical (just copied)
	if string(output) != content {
		t.Error("Already injected file should be copied without changes")
	}
}

func TestInjectFile_Chi(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"net/http"
	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})
	http.ListenAndServe(":8080", r)
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check whatapchi import
	if !strings.Contains(outputStr, "whatapchi") {
		t.Error("Output should contain whatapchi import")
	}

	// Chi uses Middleware (function value), not Middleware()
	if !strings.Contains(outputStr, "whatapchi.Middleware") {
		t.Error("Output should contain whatapchi.Middleware")
	}
}

func TestInjectDir(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "output")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create main.go
	mainContent := `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
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

	injector := NewInjector()
	if err := injector.InjectDir(srcDir, dstDir); err != nil {
		t.Fatalf("InjectDir() error = %v", err)
	}

	// Check main.go was injected
	outputMain, err := os.ReadFile(filepath.Join(dstDir, "main.go"))
	if err != nil {
		t.Fatalf("Failed to read output main.go: %v", err)
	}
	if !strings.Contains(string(outputMain), "whatapgin") {
		t.Error("main.go should contain whatapgin")
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

func TestInjectFile_Grpc(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func connectToServer() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	_ = context.Background()
	_ = conn
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check whatapgrpc import
	if !strings.Contains(outputStr, "whatapgrpc") {
		t.Error("Output should contain whatapgrpc import")
	}

	// Check server interceptor
	if !strings.Contains(outputStr, "grpc.UnaryInterceptor") {
		t.Error("Output should contain grpc.UnaryInterceptor")
	}
	if !strings.Contains(outputStr, "grpc.StreamInterceptor") {
		t.Error("Output should contain grpc.StreamInterceptor")
	}
	if !strings.Contains(outputStr, "whatapgrpc.UnaryServerInterceptor") {
		t.Error("Output should contain whatapgrpc.UnaryServerInterceptor")
	}

	// Check client interceptor
	if !strings.Contains(outputStr, "grpc.WithUnaryInterceptor") {
		t.Error("Output should contain grpc.WithUnaryInterceptor")
	}
	if !strings.Contains(outputStr, "grpc.WithStreamInterceptor") {
		t.Error("Output should contain grpc.WithStreamInterceptor")
	}
	if !strings.Contains(outputStr, "whatapgrpc.UnaryClientInterceptor") {
		t.Error("Output should contain whatapgrpc.UnaryClientInterceptor")
	}
}

func TestInjectFile_HttpClient_Get(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"fmt"
	"net/http"
)

func main() {
	resp, err := http.Get("http://example.com/api")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check whataphttp import
	if !strings.Contains(outputStr, "whataphttp") {
		t.Error("Output should contain whataphttp import")
	}

	// Check http.Get -> whataphttp.HttpGet
	if !strings.Contains(outputStr, "whataphttp.HttpGet") {
		t.Error("Output should contain whataphttp.HttpGet")
	}

	// Check nil context (when no handler context available, we use nil)
	if !strings.Contains(outputStr, "whataphttp.HttpGet(nil,") {
		t.Error("Output should contain whataphttp.HttpGet(nil, ...)")
	}
}

func TestInjectFile_HttpClient_Post(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"bytes"
	"fmt"
	"net/http"
)

func main() {
	resp, err := http.Post("http://example.com/api", "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check http.Post -> whataphttp.HttpPost
	if !strings.Contains(outputStr, "whataphttp.HttpPost") {
		t.Error("Output should contain whataphttp.HttpPost")
	}
}

func TestInjectFile_HttpClient_DefaultClient(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"fmt"
	"net/http"
)

func main() {
	resp, err := http.DefaultClient.Get("http://example.com/api")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check http.DefaultClient.Get -> whataphttp.DefaultClientGet (marker function)
	if !strings.Contains(outputStr, "whataphttp.DefaultClientGet") {
		t.Error("Output should contain whataphttp.DefaultClientGet")
	}

	// Should NOT contain http.DefaultClient.Get call pattern anymore
	if strings.Contains(outputStr, "http.DefaultClient.Get") {
		t.Error("Output should not contain http.DefaultClient.Get call")
	}
}

func TestInjectFile_HttpClient_EmptyClient(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"fmt"
	"net/http"
)

func main() {
	client := &http.Client{}
	resp, err := client.Get("http://example.com/api")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check Transport was added
	if !strings.Contains(outputStr, "Transport:") {
		t.Error("Output should contain Transport field")
	}

	// Check NewRoundTripWithEmptyTransport wrapping (marker function - indicates was empty Client{})
	if !strings.Contains(outputStr, "whataphttp.NewRoundTripWithEmptyTransport") {
		t.Error("Output should contain whataphttp.NewRoundTripWithEmptyTransport")
	}
}

func TestInjectFile_HttpClient_WithTransport(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: http.DefaultTransport,
	}
	resp, err := client.Get("http://example.com/api")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check NewRoundTrip wrapping of existing Transport
	if !strings.Contains(outputStr, "whataphttp.NewRoundTrip") {
		t.Error("Output should contain whataphttp.NewRoundTrip")
	}

	// Check the original http.DefaultTransport is wrapped
	// Uses nil context when not inside a handler function
	if !strings.Contains(outputStr, "whataphttp.NewRoundTrip(nil, http.DefaultTransport)") {
		t.Error("Output should wrap http.DefaultTransport with NewRoundTrip")
	}
}

func TestInjectFile_Kubernetes(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "main.go")
	dstFile := filepath.Join(tmpDir, "output", "main.go")

	content := `package main

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	fmt.Println(clientset)
}
`
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector()
	if err := injector.InjectFile(srcFile, dstFile); err != nil {
		t.Fatalf("InjectFile() error = %v", err)
	}

	output, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	outputStr := string(output)

	// Check whatapkubernetes import
	if !strings.Contains(outputStr, "whatapkubernetes") {
		t.Error("Output should contain whatapkubernetes import")
	}

	// Check config.Wrap call
	if !strings.Contains(outputStr, "config.Wrap") {
		t.Error("Output should contain config.Wrap")
	}

	// Check WrapRoundTripper
	if !strings.Contains(outputStr, "whatapkubernetes.WrapRoundTripper") {
		t.Error("Output should contain whatapkubernetes.WrapRoundTripper")
	}
}
