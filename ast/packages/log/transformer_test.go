package log

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// TestInjectFile_Log_NewInstance tests basic log.New() wrapping in AssignStmt.
func TestInjectFile_Log_NewInstance(t *testing.T) {
	src := `package main

import (
	"log"
	"os"
)

func main() {
	logger := log.New(os.Stderr, "APP: ", log.LstdFlags)
	logger.Println("hello")
}
`
	f := parseSource(t, src)

	tr := &Transformer{}
	transformed, err := tr.Inject(f)
	if err != nil {
		t.Fatal("Inject error:", err)
	}
	if !transformed {
		t.Error("Inject should return true")
	}

	output := renderSource(t, f)
	t.Log("Output:\n" + output)

	if !strings.Contains(output, "logsink.GetTraceLogWriter(os.Stderr)") {
		t.Error("Output should contain logsink.GetTraceLogWriter(os.Stderr)")
	}
	if !strings.Contains(output, `"github.com/whatap/go-api/logsink"`) {
		t.Error("Output should contain logsink import")
	}
}

// TestInjectFile_Log_NewInstance_StructField tests log.New() in struct field initialization.
func TestInjectFile_Log_NewInstance_StructField(t *testing.T) {
	src := `package main

import (
	"log"
	"os"
)

type app struct {
	logger *log.Logger
}

func main() {
	a := &app{
		logger: log.New(os.Stdout, "APP: ", log.LstdFlags),
	}
	a.logger.Println("hello")
}
`
	f := parseSource(t, src)

	tr := &Transformer{}
	transformed, err := tr.Inject(f)
	if err != nil {
		t.Fatal("Inject error:", err)
	}
	if !transformed {
		t.Error("Inject should return true")
	}

	output := renderSource(t, f)
	t.Log("Output:\n" + output)

	if !strings.Contains(output, "logsink.GetTraceLogWriter(os.Stdout)") {
		t.Error("Output should wrap os.Stdout in struct field init")
	}
}

// TestInjectFile_Log_NewInstance_Return tests log.New() in return statement.
func TestInjectFile_Log_NewInstance_Return(t *testing.T) {
	src := `package main

import (
	"log"
	"os"
)

func newLogger() *log.Logger {
	return log.New(os.Stderr, "APP: ", log.LstdFlags)
}

func main() {
	logger := newLogger()
	logger.Println("hello")
}
`
	f := parseSource(t, src)

	tr := &Transformer{}
	transformed, err := tr.Inject(f)
	if err != nil {
		t.Fatal("Inject error:", err)
	}
	if !transformed {
		t.Error("Inject should return true")
	}

	output := renderSource(t, f)
	t.Log("Output:\n" + output)

	if !strings.Contains(output, "logsink.GetTraceLogWriter(os.Stderr)") {
		t.Error("Output should wrap os.Stderr in return statement")
	}
}

// TestInjectFile_Log_NewInstance_FuncArg tests log.New() as function argument.
func TestInjectFile_Log_NewInstance_FuncArg(t *testing.T) {
	src := `package main

import (
	"log"
	"os"
)

func useLogger(l *log.Logger) {
	l.Println("hello")
}

func main() {
	useLogger(log.New(os.Stderr, "APP: ", log.LstdFlags))
}
`
	f := parseSource(t, src)

	tr := &Transformer{}
	transformed, err := tr.Inject(f)
	if err != nil {
		t.Fatal("Inject error:", err)
	}
	if !transformed {
		t.Error("Inject should return true")
	}

	output := renderSource(t, f)
	t.Log("Output:\n" + output)

	if !strings.Contains(output, "logsink.GetTraceLogWriter(os.Stderr)") {
		t.Error("Output should wrap os.Stderr in function argument")
	}
}

// TestInjectFile_Log_NewInstance_AlreadyWrapped tests that already-wrapped log.New() is not double-wrapped.
func TestInjectFile_Log_NewInstance_AlreadyWrapped(t *testing.T) {
	src := `package main

import (
	"log"
	"os"

	"github.com/whatap/go-api/logsink"
)

func main() {
	logger := log.New(logsink.GetTraceLogWriter(os.Stderr), "APP: ", log.LstdFlags)
	logger.Println("hello")
}
`
	f := parseSource(t, src)

	tr := &Transformer{}
	transformed, err := tr.Inject(f)
	if err != nil {
		t.Fatal("Inject error:", err)
	}

	output := renderSource(t, f)
	t.Log("Output:\n" + output)

	// Should not have nested logsink.GetTraceLogWriter(logsink.GetTraceLogWriter(...))
	count := strings.Count(output, "logsink.GetTraceLogWriter")
	if count > 1 {
		t.Errorf("Should not double-wrap: found %d occurrences of logsink.GetTraceLogWriter", count)
	}

	// transformed may be true (for SetOutput injection) or false, but no double-wrap
	_ = transformed
}

// TestRemoveFile_Log_NewInstance tests inject → remove round-trip restores original.
func TestRemoveFile_Log_NewInstance(t *testing.T) {
	src := `package main

import (
	"log"
	"os"

	"github.com/whatap/go-api/logsink"
)

func main() {
	logger := log.New(logsink.GetTraceLogWriter(os.Stderr), "APP: ", log.LstdFlags)
	logger.Println("hello")
}
`
	f := parseSource(t, src)

	tr := &Transformer{}
	err := tr.Remove(f)
	if err != nil {
		t.Fatal("Remove error:", err)
	}

	output := renderSource(t, f)
	t.Log("Output:\n" + output)

	// logsink wrapping should be removed
	if strings.Contains(output, "logsink.GetTraceLogWriter") {
		t.Error("Output should not contain logsink.GetTraceLogWriter after Remove")
	}

	// Original writer should be restored
	if !strings.Contains(output, "log.New(os.Stderr,") {
		t.Error("Output should restore original log.New(os.Stderr, ...)")
	}

	// logsink import should be removed
	if strings.Contains(output, `"github.com/whatap/go-api/logsink"`) {
		t.Error("Output should not contain logsink import after Remove")
	}
}

// TestRemoveFile_Log_NewInstance_StructField tests remove of struct field wrapped log.New().
func TestRemoveFile_Log_NewInstance_StructField(t *testing.T) {
	src := `package main

import (
	"log"
	"os"

	"github.com/whatap/go-api/logsink"
)

type app struct {
	logger *log.Logger
}

func main() {
	a := &app{
		logger: log.New(logsink.GetTraceLogWriter(os.Stdout), "APP: ", log.LstdFlags),
	}
	a.logger.Println("hello")
}
`
	f := parseSource(t, src)

	tr := &Transformer{}
	err := tr.Remove(f)
	if err != nil {
		t.Fatal("Remove error:", err)
	}

	output := renderSource(t, f)
	t.Log("Output:\n" + output)

	if strings.Contains(output, "logsink.GetTraceLogWriter") {
		t.Error("Output should not contain logsink.GetTraceLogWriter after Remove")
	}
	if !strings.Contains(output, "log.New(os.Stdout,") {
		t.Error("Output should restore original log.New(os.Stdout, ...)")
	}
}

// TestInjectRemoveRoundTrip tests that inject followed by remove produces equivalent code.
func TestInjectRemoveRoundTrip(t *testing.T) {
	src := `package main

import (
	"log"
	"os"
)

type app struct {
	logger *log.Logger
}

func newLogger() *log.Logger {
	return log.New(os.Stderr, "APP: ", log.LstdFlags)
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("hello")

	a := &app{
		logger: log.New(os.Stderr, "APP: ", log.LstdFlags),
	}
	a.logger.Println("hello")
}
`
	f := parseSource(t, src)

	tr := &Transformer{}

	// Inject
	transformed, err := tr.Inject(f)
	if err != nil {
		t.Fatal("Inject error:", err)
	}
	if !transformed {
		t.Error("Inject should return true")
	}

	injectedOutput := renderSource(t, f)
	t.Log("After inject:\n" + injectedOutput)

	// Verify all log.New() calls are wrapped
	wrapCount := strings.Count(injectedOutput, "logsink.GetTraceLogWriter")
	if wrapCount < 3 {
		t.Errorf("Expected at least 3 logsink.GetTraceLogWriter wrappings, got %d", wrapCount)
	}

	// Remove
	err = tr.Remove(f)
	if err != nil {
		t.Fatal("Remove error:", err)
	}

	removedOutput := renderSource(t, f)
	t.Log("After remove:\n" + removedOutput)

	// All logsink references should be gone
	if strings.Contains(removedOutput, "logsink") {
		t.Error("After Remove, output should not contain any logsink references")
	}

	// Original log.New() calls should be restored
	newCount := strings.Count(removedOutput, "log.New(os.")
	if newCount < 3 {
		t.Errorf("Expected at least 3 restored log.New(os...) calls, got %d", newCount)
	}
}

// parseSource is a test helper that parses Go source code into a dst.File.
func parseSource(t *testing.T, src string) *dst.File {
	t.Helper()
	fset := token.NewFileSet()
	f, err := decorator.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal("Parse error:", err)
	}
	return f
}

// renderSource is a test helper that renders a dst.File back to source code.
func renderSource(t *testing.T, f *dst.File) string {
	t.Helper()
	restorer := decorator.NewRestorer()
	var buf bytes.Buffer
	if err := restorer.Fprint(&buf, f); err != nil {
		t.Fatal("Fprint error:", err)
	}
	return buf.String()
}
