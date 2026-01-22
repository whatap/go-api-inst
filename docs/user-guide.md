# whatap-go-inst User Guide

CLI tool for automatically injecting WhaTap monitoring code into Go applications.

## Table of Contents

- [System Requirements](#system-requirements)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Global Options](#global-options)
- [Command Reference](#command-reference)
- [Execution Modes](#execution-modes)
- [Supported Frameworks](#supported-frameworks)
- [Environment Variables](#environment-variables)
- [Injected Code](#injected-code)
- [Limitations and Notes](#limitations-and-notes)
- [Migration from Manual to Auto Instrumentation](#migration-from-manual-to-auto-instrumentation)
- [Troubleshooting](#troubleshooting)
- [Multi-Module Projects](#multi-module-projects)

---

## System Requirements

| Item | Requirement | Notes |
|------|-------------|-------|
| **Go Version** | **1.18 or higher** | Required by go-api library |
| **OS** | Linux, macOS, Windows | All OS supported by Go |
| **Architecture** | amd64, arm64, etc. | All architectures supported by Go |

> **Note**: The whatap-go-inst binary itself can run without Go installed. However, Go 1.18+ is required to build the instrumented code.

---

## Installation

### Method 1: go install (Go 1.21+)

For Go 1.21+, toolchain auto-download is supported.

```bash
# Official binary
go install github.com/whatap/go-api-inst/cmd/whatap-go-inst@latest

# Short binary name
go install github.com/whatap/go-api-inst/cmd/goinst@latest
```

### Method 2: Direct Binary Download (Linux)

For Go 1.18~1.20 users or to use without Go installation:

```bash
# Linux (amd64)
curl -sSL https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_amd64.tar.gz | tar xz
sudo mv whatap-go-inst /usr/local/bin/

# Linux (arm64)
curl -sSL https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_arm64.tar.gz | tar xz
sudo mv whatap-go-inst /usr/local/bin/
```

> **macOS/Windows**: Use the `go install` method.

### Verify Installation

```bash
whatap-go-inst version

# Or short command
goinst version
```

After installation:
```bash
# Use instead of regular go build
whatap-go-inst go build ./...

# Or short command
goinst go build ./...
```

### Build from Source

```bash
git clone https://github.com/whatap/go-api-inst.git
cd go-api-inst
go build -o whatap-go-inst .
```

---

## Quick Start

### Fast Mode (Recommended)

```bash
# 1. Initialize (once, in go.mod directory)
whatap-go-inst init

# 2. Add dependencies
go get github.com/whatap/go-api@latest
go mod tidy

# 3. Instrumented build
whatap-go-inst go build ./...

# 4. Run
./myapp  # or whatap-go-inst go run .
```

This approach:
- `init` creates tool.go (whatap import declarations)
- `go get` + `go mod tidy` adds whatap dependencies to go.mod
- Fast build speed (uses toolexec)
- Original source only gets tool.go added

### Wrap Mode (100% Original Preservation)

```bash
# Build without init (slower but no original changes)
whatap-go-inst go --wrap build ./...
```

This approach:
- Does not modify original source code at all
- Use for first test or when original preservation required
- Transformed source saved in whatap-instrumented/

### Test Without Dependencies (--wrap mode)

```bash
# Build without adding dependencies (uses temp dependencies)
whatap-go-inst go --wrap build ./...
```

This approach:
- No changes to original go.mod
- Auto-adds dependencies and builds in temp directory
- Useful for first-time users or testing

---

## Global Options

Common options available for all commands.

| Option | Short | Default | Description |
|--------|-------|---------|-------------|
| `--config` | | `.whatap/config.yaml` | Config file path |
| `--verbose` | `-v` | `false` | Verbose output (includes transformation details) |
| `--quiet` | `-q` | `false` | Summary only |
| `--report` | | | JSON report file path |

### Output Levels

```bash
# Default output
whatap-go-inst inject -s ./src -o ./output

# Verbose output (per-file transformation details)
whatap-go-inst inject -s ./src -o ./output -v

# Summary only
whatap-go-inst inject -s ./src -o ./output -q
```

### JSON Report

Save JSON format reports for analysis or CI/CD integration.

```bash
whatap-go-inst inject -s ./src -o ./output --report=report.json
```

Report structure:
```json
{
  "timestamp": "2026-01-07T15:00:00+09:00",
  "command": "inject",
  "summary": {
    "total": 10,
    "instrumented": 3,
    "skipped": 2,
    "copied": 5,
    "errors": 0
  },
  "files": [
    {
      "path": "main.go",
      "status": "instrumented",
      "transformers": ["gin"],
      "changes": ["added import: whatapgin", "added: trace.Init"]
    }
  ]
}
```

---

## Command Reference

### `whatap-go-inst go` - Build Wrapper (Recommended)

Wraps go commands to auto-inject monitoring code during build.

```bash
whatap-go-inst go [options] <command> [arguments]
```

#### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--wrap` | | Wrap mode (adds temp dependencies, go.mod doesn't need whatap/go-api) |
| `--output` | `-O` | Instrumented source save path |
| `--error-tracking` | | Inject error tracking code (adds `trace.Error` to `if err != nil` patterns) |

#### Supported Commands

| Command | Description | Example |
|---------|-------------|---------|
| `build` | Build | `whatap-go-inst go build ./...` |
| `run` | Run | `whatap-go-inst go run .` |
| `test` | Test | `whatap-go-inst go test ./...` |
| `install` | Install | `whatap-go-inst go install ./...` |

#### Examples

```bash
# Build entire project (requires whatap/go-api in go.mod)
whatap-go-inst go build ./...

# Build specific file
whatap-go-inst go build ./main.go

# Specify output file
whatap-go-inst go build -o myapp .

# Run directly
whatap-go-inst go run .
whatap-go-inst go run main.go

# Run tests
whatap-go-inst go test ./...

# Build with error tracking code
whatap-go-inst go --error-tracking build ./...

# Wrap mode (no whatap/go-api needed in go.mod)
whatap-go-inst go --wrap build ./...

# Save instrumented source
whatap-go-inst go -O ./instrumented build ./...
```

#### Internal Operation

1. Create temp directory (`whatap-go-inst-build-*`)
2. Copy source files (`.go`, `go.mod`, `go.sum`)
3. AST analysis and monitoring code injection
4. Run `go mod tidy` (resolve dependencies)
5. Execute specified go command
6. Copy build artifacts (to original location)
7. Delete temp directory

---

### `whatap-go-inst inject` - Direct Source Modification

Injects monitoring code into source and outputs to separate directory.

```bash
whatap-go-inst inject [flags]
```

#### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--src` | `-s` | `.` | Source path (file or directory) |
| `--output` | `-o` | `./output` | Output directory |
| `--error-tracking` | | `false` | Inject error tracking code |

**Error Tracking Option**: By default, error tracking code is not injected. This prevents duplication since whatap packages already track errors internally. Use `--error-tracking` flag to also track errors in your business logic.

#### Examples

```bash
# Inject entire directory
whatap-go-inst inject -s ./myapp -o ./myapp-instrumented

# With error tracking code
whatap-go-inst inject -s ./myapp -o ./myapp-instrumented --error-tracking

# Single file injection
whatap-go-inst inject -s ./main.go -o ./main_instrumented.go

# Current directory (default)
whatap-go-inst inject
```

#### Build After Injection

```bash
# 1. Inject code
whatap-go-inst inject -s ./myapp -o ./output

# 2. Add dependencies (in output directory)
cd output
go get github.com/whatap/go-api@latest
go mod tidy

# 3. Build
go build -o ../myapp .
```

---

### `whatap-go-inst remove` - Remove Monitoring Code

Removes injected monitoring code to restore original state.

```bash
whatap-go-inst remove [flags]
```

#### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--src` | `-s` | `.` | Source path (file or directory) |
| `--output` | `-o` | `./output` | Output directory |
| `--all` | | `false` | Also remove manually injected patterns |

#### Examples

```bash
# Remove inject patterns only (default)
whatap-go-inst remove -s ./instrumented -o ./clean

# Compare with original (should have no differences)
diff -r ./original ./clean

# Also remove manually injected patterns (--all)
whatap-go-inst remove --all -s ./src -o ./clean
```

#### --all Option Details

The `--all` option removes go-api code that users manually injected, in addition to patterns created by the `inject` command.

**Removed patterns:**

| Pattern | Example |
|---------|---------|
| Standalone statement | `trace.Step(...)`, `trace.Println(...)` |
| defer statement | `defer trace.End(ctx, nil)` |
| AddHook call | `rdb.AddHook(whatapgoredis.NewHook())` |
| logsink call | `logsink.GetTraceLogWriter(...)` |

**Not removed (warning printed):**

| Pattern | Example | Reason |
|---------|---------|--------|
| Variable assignment/declaration | `ctx := trace.Start(...)` | Would break ctx usage |
| Closure pattern | `whatapsql.Wrap(ctx, ..., func(){...})` | Contains business logic |
| struct field | `Transport: whataphttp.NewRoundTripper(...)` | Need to restore field value |
| Other whatap calls | `whatapsql.End(...)`, `httpc.End(...)` | Paired with Start |

Non-removable patterns print warning messages and require manual handling.

```bash
# Warning message example
Warning: main.go: Variable declaration: trace.Start(...) (manual removal required)
Warning: db.go: Closure pattern: whatapsql.Wrap(...) (contains business logic, manual removal required)
```

---

### `whatap-go-inst init` - Project Setup

Analyzes project to add whatap dependencies and set up go:generate directives.

```bash
whatap-go-inst init [directory]
```

#### Actions Performed

1. Analyze project source -> detect frameworks in use
2. Create `whatap_inst.tool.go` file (whatap import declarations for toolexec mode)
3. Create `whatap_inst_generate.go` file (go:generate directive)

> **Note**: After `init`, you must run `go get github.com/whatap/go-api@latest` and `go mod tidy` to add dependencies.

#### Examples

```bash
# Apply init to current directory
whatap-go-inst init

# Add dependencies (required!)
go get github.com/whatap/go-api@latest
go mod tidy

# Apply init to specific directory
whatap-go-inst init ./myapp

# Build after init
whatap-go-inst go build ./...
# or
go generate ./... && go build ./...
```

#### Generated Files

**whatap_inst.tool.go**
```go
// Code generated by whatap-go-inst init; DO NOT EDIT.
//go:build tools

package tools

import (
    _ "github.com/whatap/go-api/trace"
    _ "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
)
```

**whatap_inst_generate.go**
```go
// Code generated by whatap-go-inst init; DO NOT EDIT.

//go:generate whatap-go-inst generate

package main
```

---

### `whatap-go-inst uninit` - Remove Project Setup

Removes files added by the init command.

```bash
whatap-go-inst uninit [directory]
```

#### Examples

```bash
whatap-go-inst uninit
```

---

### `whatap-go-inst generate` - Execute go:generate

Called by go:generate to perform code injection. Generally not called directly.

```bash
whatap-go-inst generate [directory]
```

#### Examples

```bash
# Run via go generate (recommended)
go generate ./...

# Direct execution
whatap-go-inst generate ./myapp
```

---

### `whatap-go-inst toolexec` - Compiler Extension

Used with go build -toolexec flag to inject code at compile time.

```bash
go build -toolexec="whatap-go-inst toolexec" ./...
```

**Note**: toolexec mode does not automatically resolve dependencies. Run `whatap-go-inst init` and add dependencies first.

#### Examples

```bash
# 1. Initialize and add dependencies (once)
whatap-go-inst init
go get github.com/whatap/go-api@latest
go mod tidy

# 2. Build with toolexec
go build -toolexec="whatap-go-inst toolexec" -o myapp ./...
```

---

### `whatap-go-inst version` - Version Info

Displays version information.

```bash
whatap-go-inst version
```

Output example:
```
whatap-go-inst version 0.1.0
  Git commit: abc1234
  Build date: 2024-12-09T10:00:00Z
```

---

## Execution Modes

### Mode Comparison

| Mode | Command | Source Changes | Auto Dependencies | Complexity | Recommended |
|------|---------|----------------|-------------------|------------|-------------|
| Build Wrapper | `whatap-go-inst go build` | No | Yes | Low | **Recommended** |
| Build Wrapper (wrap) | `whatap-go-inst go --wrap build` | No | Yes | Low | Test/First use |
| toolexec | `go build -toolexec=...` | No | No (manual) | Medium | Advanced |
| go:generate | `whatap-go-inst init` + `go generate` | Yes | No (manual) | Medium | Git commit |
| Source Modify | `whatap-go-inst inject` | Yes | No (manual) | Medium | Review needed |

### 1. Build Wrapper Mode (Recommended)

The simplest and recommended approach.

```bash
whatap-go-inst go build ./...
whatap-go-inst go run .
```

**Advantages**:
- No original source changes
- Auto dependency resolution
- Same usage as regular go commands

### 2. toolexec Mode

Uses Go compiler's toolexec mechanism.

```bash
# 1. Initialize (once)
whatap-go-inst init

# 2. Add dependencies (required!)
go get github.com/whatap/go-api@latest
go mod tidy

# 3. Build
go build -toolexec="whatap-go-inst toolexec" ./...
```

**Advantages**:
- No original source changes
- Compile-time transformation

**Disadvantages**:
- Manual dependency management required (`go get` + `go mod tidy`)
- More complex setup

### 3. go:generate Mode

Leverages Go's standard code generation tool.

```bash
# 1. Setup
whatap-go-inst init

# 2. Add dependencies (required!)
go get github.com/whatap/go-api@latest
go mod tidy

# 3. Generate code
go generate ./...

# 4. Build
go build ./...
```

**Advantages**:
- Can commit transformed code to Git
- Compatible with CI/CD pipelines

**Disadvantages**:
- Source code directly modified
- Manual dependency management required (`go get` + `go mod tidy`)

### 4. Direct Source Modification Mode

Generates transformed source in separate directory.

```bash
# Transform
whatap-go-inst inject -s ./myapp -o ./output

# Build
cd output && go get github.com/whatap/go-api@latest && go build .
```

**Advantages**:
- Easy before/after comparison
- Original source preserved

**Disadvantages**:
- Manual steps required

---

## Supported Frameworks

### Web Frameworks

| Framework | Import Path | Injected Code |
|-----------|-------------|---------------|
| **Gin** | `github.com/gin-gonic/gin` | `r.Use(whatapgin.Middleware())` |
| **Echo v4** | `github.com/labstack/echo/v4` | `e.Use(whatapecho.Middleware())` |
| **Fiber v2** | `github.com/gofiber/fiber/v2` | `app.Use(whatapfiber.Middleware())` |
| **Chi v5** | `github.com/go-chi/chi/v5` | `r.Use(whatapchi.Middleware)` |
| **Gorilla Mux** | `github.com/gorilla/mux` | `r.Use(whatapmux.Middleware)` |
| **net/http** | `net/http` | `whataphttp.Func()`, `whataphttp.Handler()` |
| **FastHTTP** | `github.com/valyala/fasthttp` | `whatapfasthttp.Middleware()` |

### Database

| Library | Import Path | Transformation |
|---------|-------------|----------------|
| **database/sql** | `database/sql` | `sql.Open()` -> `whatapsql.Open()` |
| **sqlx** | `github.com/jmoiron/sqlx` | `sqlx.Open()` -> `whatapsqlx.Open()` |
| **GORM (gorm.io)** | `gorm.io/gorm` | `gorm.Open()` -> `whatapgorm.Open()` |
| **GORM (jinzhu)** | `github.com/jinzhu/gorm` | `gorm.Open()` -> `whatapgorm.Open()` |

### Redis

| Library | Import Path | Transformation |
|---------|-------------|----------------|
| **go-redis v9** | `github.com/redis/go-redis/v9` | `redis.NewClient()` -> `whatapgoredis.NewClient()` |
| **go-redis v8** | `github.com/go-redis/redis/v8` | `redis.NewClient()` -> `whatapgoredis.NewClient()` |
| **Redigo** | `github.com/gomodule/redigo` | `redis.Dial()` -> `whatapredigo.Dial()` |

### NoSQL

| Library | Import Path | Transformation |
|---------|-------------|----------------|
| **MongoDB** | `go.mongodb.org/mongo-driver/mongo` | `mongo.Connect()` -> `whatapmongo.Connect()` |

### External Services

| Library | Import Path | Description |
|---------|-------------|-------------|
| **Sarama (IBM)** | `github.com/IBM/sarama` | Kafka client |
| **Sarama (Shopify)** | `github.com/Shopify/sarama` | Kafka client |
| **gRPC** | `google.golang.org/grpc` | Auto Server/Client Interceptor injection |
| **Kubernetes** | `k8s.io/client-go` | Auto `config.Wrap()` injection |

### Logging Libraries

| Library | Import Path | Injected Code |
|---------|-------------|---------------|
| **log** | `log` | `log.SetOutput(logsink.GetTraceLogWriter(os.Stderr))` |
| **logrus** | `github.com/sirupsen/logrus` | `logrus.SetOutput(logsink.GetTraceLogWriter(os.Stderr))` |
| **zap** | `go.uber.org/zap` | `logsink.HookStderr()` |

> **Note**: When logging libraries are instrumented, `@txid`, `@mtid`, `@gid` fields are automatically added to correlate transactions with logs.

---

## Log Collection Guide

WhaTap Go Agent collects logs in two ways.

### Recommended: TraceLogWriter (AST Auto Instrumentation)

**Automatically injected by whatap-go-inst**. Recommended because it can correlate transactions with logs.

```go
// Code auto-injected by AST
log.SetOutput(logsink.GetTraceLogWriter(os.Stderr))
```

| Advantage | Description |
|-----------|-------------|
| **Transaction correlation** | Logs include `@txid`, `@mtid`, `@gid` automatically |
| **Performance** | No pipe overhead |
| **Accuracy** | Each Write() call is one complete log message |

**Supported libraries:**

| Library | Method |
|---------|--------|
| log (standard) | `log.SetOutput(TraceLogWriter)` |
| logrus | `logrus.SetOutput(TraceLogWriter)` |
| zap | `logsink.HookStderr()` |
| fmt | Convert to `whatapfmt.Print*()` |

### Legacy: ProxyStream (stdout/stderr pipe)

**Legacy method** that collects stdout/stderr without code changes. **Not recommended** due to difficulty in transaction correlation.

```ini
# whatap.conf (default: false recommended)
logsink_stdout_enabled=false
logsink_stderr_enabled=false
```

| Disadvantage | Description |
|--------------|-------------|
| **No transaction correlation** | Cannot know txid at output time |
| **Pipe overhead** | Additional goroutine, performance impact |
| **Line splitting** | Multiline logs (stack traces, etc.) get split |

### Defaults and Recommended Settings

go-api defaults recommend TraceLogWriter.

```ini
# whatap.conf defaults
logsink_enabled=false           # Overall log collection (needs activation)
logsink_trace_enabled=true      # TraceLogWriter (default enabled)
logsink_stdout_enabled=false    # ProxyStream stdout (default disabled)
logsink_stderr_enabled=false    # ProxyStream stderr (default disabled)
```

> **Note**: Setting just `logsink_enabled=true` auto-enables TraceLogWriter.

### Comparison Summary

| Item | TraceLogWriter (Recommended) | ProxyStream (Legacy) |
|------|------------------------------|----------------------|
| Transaction correlation | Yes (txid/mtid/gid included) | No |
| Performance | No overhead | Pipe overhead |
| Multiline logs | Collected as single log | Split by line |
| AST instrumentation required | Auto-injected | Not needed |
| Usage condition | Use whatap-go-inst | Manual config |

> **Migration**: Existing ProxyStream users automatically switch to TraceLogWriter when building with whatap-go-inst. Set `logsink_stdout_enabled=false`, `logsink_stderr_enabled=false` in whatap.conf to prevent duplicate collection.

---

## Configuration File

Manage CLI options via configuration file.

> **Detailed documentation**: [Configuration Guide](./config.md)

### Quick Start

```yaml
# .whatap/config.yaml
instrumentation:
  preset: "full"           # Instrumentation preset (full/minimal/web/database/external/log/custom)
  error_tracking: false    # Inject error tracking code
  disabled_packages:       # Packages to disable
    - "k8s"
    - "grpc"
```

### Preset Options

| Preset | Description |
|--------|-------------|
| `full` | Enable all packages **(default)** |
| `minimal` | Add only trace.Init/Shutdown |
| `web` | Web frameworks only (gin, echo, fiber, chi, gorilla, nethttp, fasthttp) |
| `database` | Databases only (sql, sqlx, gorm, jinzhugorm) |
| `external` | External services only (redigo, goredis, mongo, sarama, grpc, k8s) |
| `log` | Logging libraries only (fmt, log, logrus, zap) |
| `custom` | Specify with enabled_packages |

### Priority

```
CLI options > Environment variables > Config file > Defaults
```

---

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `GO_API_AST_DEBUG` | Enable debug output | `GO_API_AST_DEBUG=1` |
| `GO_API_AST_OUTPUT_DIR` | Instrumented source output directory | `GO_API_AST_OUTPUT_DIR=./instrumented` |
| `WHATAP_INST_CONFIG` | Config file path | `WHATAP_INST_CONFIG=./config.yaml` |

### Debug Mode Usage

```bash
# Verbose log output
GO_API_AST_DEBUG=1 whatap-go-inst go build ./...
```

---

## Injected Code

### Import Addition

```go
import (
    "github.com/whatap/go-api/trace"
    "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
)
```

### main() Initialization

```go
func main() {
    trace.Init(nil)
    defer trace.Shutdown()
    // Original code...
}
```

### Web Framework Middleware

**Gin**
```go
r := gin.Default()
r.Use(whatapgin.Middleware())  // Auto-injected
```

**Echo**
```go
e := echo.New()
e.Use(whatapecho.Middleware())  // Auto-injected
```

**net/http**
```go
// Before
http.HandleFunc("/api", handler)

// After
http.HandleFunc("/api", whataphttp.Func(handler))
```

### Database

**database/sql**
```go
// Before
db, _ := sql.Open("mysql", dsn)

// After
db, _ := whatapsql.Open("mysql", dsn)
```

---

## Limitations and Notes

### Unsupported Code Patterns

The following patterns are not auto-instrumented. You need to add whatap code manually.

#### 1. Using dot import

```go
// Middleware NOT auto-added
import . "github.com/gin-gonic/gin"

func main() {
    r := Default()  // Cannot detect without gin. prefix
    r.Run()
}
```

**Solution**: Use regular import
```go
// Works correctly
import "github.com/gin-gonic/gin"

func main() {
    r := gin.Default()  // Detected
    r.Run()
}
```

#### 2. Router initialization in global variables

```go
// Middleware NOT auto-added
var Router = gin.Default()  // Global-level initialization

func main() {
    Router.Run()
}
```

**Solution**: Initialize in main() or init() function
```go
// Works correctly
var Router *gin.Engine

func main() {
    Router = gin.Default()  // Initialized in function - middleware added
    Router.Run()
}
```

### Known Limitations

| Pattern | Behavior | Solution |
|---------|----------|----------|
| dot import | Framework detected but middleware not added | Use regular import |
| Global variable init | Middleware not added | Initialize in main()/init() |
| vendor directory | Auto-skipped (expected) | - |

---

## Migration from Manual to Auto Instrumentation

How to migrate projects with manually inserted whatap/go-api code to auto instrumentation.

### Migration Procedure

```bash
# 1. Remove existing whatap code
whatap-go-inst remove -s . -o ./cleaned

# 2. Check diff (ensure no custom code missing)
diff -r . ./cleaned

# 3. Replace if no issues
cp -r ./cleaned/* ./

# 4. Build with auto injection
whatap-go-inst go build ./...
```

### Notes

| Code Type | remove Behavior |
|-----------|-----------------|
| Standard patterns (middleware, sql.Open, etc.) | Auto-removed |
| Custom code (trace.Start, etc.) | Manual check required |

```go
// Auto-removed
r.Use(whatapgin.Middleware())
db, _ := whatapsql.Open("mysql", dsn)

// Manual check required (custom code)
ctx = trace.Start(ctx, "custom-span")
defer trace.End(ctx, nil)
```

### diff Check Checklist

1. **trace.Init/Shutdown**: Verify removed
2. **Middleware**: Verify all framework middleware removed
3. **Custom tracking code**: Decide to keep/remove code directly in business logic
4. **import statements**: Verify whatap-related imports removed

---

## Troubleshooting

### Instrumentation Not Applied

1. **Clear build cache**
   ```bash
   go clean -cache
   whatap-go-inst go build ./...
   ```

2. **Check with debug mode**
   ```bash
   GO_API_AST_DEBUG=1 whatap-go-inst go build ./...
   ```

### Compilation Error: whatap package not found

```bash
# Auto-resolved when using build wrapper
whatap-go-inst go build ./...

# Manual resolution
go get github.com/whatap/go-api@latest
go mod tidy
```

### Dependency Error in toolexec Mode

toolexec does not auto-add dependencies.

```bash
# 1. Run init first
whatap-go-inst init

# 2. Add dependencies (required!)
go get github.com/whatap/go-api@latest
go mod tidy

# 3. Then use toolexec
go build -toolexec="whatap-go-inst toolexec" ./...
```

### inject/remove Result Differs from Original

After inject -> remove, it should be **functionally identical** to original.

**Known limitations (not bugs):**
- **Blank lines between functions or import groups may disappear**
- Due to DST (Decorated Syntax Tree) library limitations, blank line decorations are not perfectly preserved
- Running `go fmt` does not restore blank lines
- Does not affect code behavior

```bash
# Compare
diff -r ./original ./cleaned

# Only blank line differences = normal
# Code logic differences = please report bug
# https://github.com/whatap/go-api-inst/issues
```

---

## Multi-Module Projects

Additional setup required when instrumenting projects split into multiple Go modules.

### Summary

| Package Type | Instrumented |
|--------------|--------------|
| My project code | Yes |
| External modules (go get) | No (skipped) |
| replace local modules | Yes (default mode only) |

### Recommended Approach

```bash
# Add replace to go.mod
replace mycompany/shared-lib => ../shared-lib

# Initialize (once)
whatap-go-inst init

# Add whatap dependency to all modules
go get github.com/whatap/go-api@latest

# Build (replace modules also instrumented)
whatap-go-inst go build ./...
```

> **Detailed documentation**: [Multi-Module Projects Guide](./multi-module.md)

---

## Related Projects

- [whatap/go-api](https://github.com/whatap/go-api) - WhaTap Go monitoring library
- [whatap/go-api-example](https://github.com/whatap/go-api-example) - Usage examples
