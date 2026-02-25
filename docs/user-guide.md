# whatap-go-inst User Guide

CLI tool for automatically injecting WhaTap monitoring code into Go applications.

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

### Method 1: Direct Binary Download (Recommended)

```bash
# Linux (amd64)
curl -sSL https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_amd64.tar.gz | tar xz
sudo mv whatap-go-inst /usr/local/bin/

# Linux (arm64)
curl -sSL https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_arm64.tar.gz | tar xz
sudo mv whatap-go-inst /usr/local/bin/
```

### Method 2: go install (Go 1.21+)

```bash
# Official binary
go install github.com/whatap/go-api-inst/cmd/whatap-go-inst@latest

# Short binary name
go install github.com/whatap/go-api-inst/cmd/goinst@latest
```

### Verify Installation

```bash
whatap-go-inst version

# Or short command
goinst version
```

### Build from Source

```bash
git clone https://github.com/whatap/go-api-inst.git
cd go-api-inst
go build -o whatap-go-inst .
```

---

## Quick Start

```bash
# Build (no init required)
whatap-go-inst go build ./...

# Run
./myapp  # or whatap-go-inst go run .
```

This approach:
- No setup required - just build
- Does not modify original source code at all
- Auto-adds dependencies in temp directory
- Transformed source saved in whatap-instrumented/ (use `--no-output` to disable)

---

## Global Options

| Option | Short | Default | Description |
|--------|-------|---------|-------------|
| `--config` | | `.whatap/config.yaml` | Config file path |
| `--verbose` | `-v` | `false` | Verbose output (includes transformation details) |
| `--quiet` | `-q` | `false` | Summary only |
| `--report` | | | JSON report file path |
| `--output` | | `whatap-instrumented/` | Instrumented source output directory |
| `--no-output` | | `false` | Do not save instrumented source |
| `--error-tracking` | | `false` | Inject error tracking code (`trace.Error` in `if err != nil` patterns) |
| `--external-module` | | | External module to instrument from GOMODCACHE (repeatable, comma-separated, wildcard supported) |

### JSON Report

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
whatap-go-inst [global-options] go <command> [arguments]
```

| Command | Description | Example |
|---------|-------------|---------|
| `build` | Build | `whatap-go-inst go build ./...` |
| `run` | Run | `whatap-go-inst go run .` |
| `test` | Test | `whatap-go-inst go test ./...` |
| `install` | Install | `whatap-go-inst go install ./...` |

#### Examples

```bash
# Build entire project
whatap-go-inst go build ./...

# Specify output file
whatap-go-inst go build -o myapp .

# Run directly
whatap-go-inst go run .

# Run tests
whatap-go-inst go test ./...

# Build with error tracking code
whatap-go-inst --error-tracking go build ./...

# Disable instrumented source output
whatap-go-inst --no-output go build ./...

# Save instrumented source to custom path
whatap-go-inst --output ./instrumented go build ./...

# Instrument external module from GOMODCACHE
whatap-go-inst --external-module=mycompany.com/internal/lib go build ./...
```

#### Internal Operation

1. Create temp directory
2. Copy source files (`.go`, `go.mod`, `go.sum`)
3. If `--external-module` specified: copy modules from GOMODCACHE, inject, add `replace` directives
4. AST analysis and monitoring code injection
5. Run `go get github.com/whatap/go-api@latest` + `go mod tidy`
6. Execute specified go command
7. Copy build artifacts to original location
8. Save instrumented source to `whatap-instrumented/` (unless `--no-output`)
9. Delete temp directory

> **Details**: [Build Wrapper Mode](./build-wrapper.md)

---

### `whatap-go-inst inject` - Direct Source Modification

Injects monitoring code into source and outputs to separate directory.

```bash
whatap-go-inst inject [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--src` | `-s` | `.` | Source path (file or directory) |
| `--output` | `-o` | `./output` | Output directory |
| `--error-tracking` | | `false` | Inject error tracking code |
| `--external-module` | | | External module to instrument from GOMODCACHE |

```bash
# Inject entire directory
whatap-go-inst inject -s ./myapp -o ./myapp-instrumented

# Build after injection
cd myapp-instrumented
go get github.com/whatap/go-api@latest
go mod tidy
go build -o ../myapp .
```

> **Details**: [Source Inject Mode](./source-inject.md)

---

### `whatap-go-inst remove` - Remove Monitoring Code

Removes injected monitoring code to restore original state.

```bash
whatap-go-inst remove [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--src` | `-s` | `.` | Source path (file or directory) |
| `--output` | `-o` | `./output` | Output directory |
| `--all` | | `false` | Also remove manually injected patterns |

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

---

### `whatap-go-inst version` - Version Info

```bash
whatap-go-inst version
```

---

## Supported Frameworks

### Web Frameworks

| Framework | Import Path | Injected Code |
|-----------|-------------|---------------|
| **Gin** | `github.com/gin-gonic/gin` | `r.Use(whatapgin.Middleware())` |
| **Echo v4** | `github.com/labstack/echo/v4` | `e.Use(whatapecho.Middleware())` |
| **Fiber v2** | `github.com/gofiber/fiber/v2` | `app.Use(whatapfiber.Middleware())` |
| **Chi v5** | `github.com/go-chi/chi/v5` | `whatapchi.WrapRouter(chi.NewRouter())` |
| **Gorilla Mux** | `github.com/gorilla/mux` | `whatapmux.WrapRouter(mux.NewRouter())` |
| **net/http** | `net/http` | `whataphttp.Func()`, `whataphttp.WrapHandler()` |
| **FastHTTP** | `github.com/valyala/fasthttp` | `whatapfasthttp.Middleware()` |

> **Wrap Functions**: For struct field initialization and instance patterns,
> framework-specific Wrap functions are available (e.g., `WrapEngine`, `WrapEcho`, `WrapApp`, `WrapRouter`, `WrapHandler`).
> See [Instrumentation Rules](./rules/) for details.

### Database

| Library | Import Path | Transformation |
|---------|-------------|----------------|
| **database/sql** | `database/sql` | `sql.Open()` → `whatapsql.Open()` |
| **sqlx** | `github.com/jmoiron/sqlx` | `sqlx.Open()` → `whatapsqlx.Open()` |
| **GORM (gorm.io)** | `gorm.io/gorm` | `gorm.Open()` → `whatapgorm.Open()` |
| **GORM (jinzhu)** | `github.com/jinzhu/gorm` | `gorm.Open()` → `whatapgorm.Open()` |

### Redis

| Library | Import Path | Transformation |
|---------|-------------|----------------|
| **go-redis v9** | `github.com/redis/go-redis/v9` | `redis.NewClient()` → `whatapgoredis.NewClient()` |
| **go-redis v8** | `github.com/go-redis/redis/v8` | `redis.NewClient()` → `whatapgoredis.NewClient()` |
| **Redigo** | `github.com/gomodule/redigo` | `redis.Dial()` → `whatapredigo.Dial()` |

### NoSQL / Cache

| Library | Import Path | Transformation |
|---------|-------------|----------------|
| **MongoDB** | `go.mongodb.org/mongo-driver/mongo` | `mongo.Connect()` → `whatapmongo.Connect()` |
| **Aerospike** | `github.com/aerospike/aerospike-client-go` | Closure wrap with `whatapsql.Wrap()` |

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
| **logrus** | `github.com/sirupsen/logrus` | Hook-based + `WrapLogger()` for instances |
| **zap** | `go.uber.org/zap` | `logsink.HookStderr()` |

> **Note**: When logging libraries are instrumented, `@txid`, `@mtid`, `@gid` fields are automatically added to correlate transactions with logs.

### Version Filtering

Unsupported framework versions are automatically skipped (not instrumented):

| Framework | Supported | Skipped |
|-----------|-----------|---------|
| Echo | v3, v4 | v5+ |
| Fiber | v2 | v1, v3+ |
| go-redis | v8, v9 | v7-, v10+ |
| Aerospike | v6, v8 | v5-, v7, v9+ |

See [Version Filtering](./rules/versions.md) for details.

---

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `GO_API_AST_DEBUG` | Enable debug output | `GO_API_AST_DEBUG=1` |
| `GO_API_AST_OUTPUT_DIR` | Instrumented source output directory | `GO_API_AST_OUTPUT_DIR=./instrumented` |
| `WHATAP_INST_CONFIG` | Config file path | `WHATAP_INST_CONFIG=./config.yaml` |

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

```go
// Before
db, _ := sql.Open("mysql", dsn)

// After
db, _ := whatapsql.Open("mysql", dsn)
```

---

## Limitations and Notes

### Unsupported Code Patterns

#### 1. Using dot import

```go
// Middleware NOT auto-added
import . "github.com/gin-gonic/gin"

func main() {
    r := Default()  // Cannot detect without gin. prefix
}
```

**Solution**: Use regular import.

#### 2. Router initialization in global variables

```go
// Middleware NOT auto-added
var Router = gin.Default()  // Global-level initialization
```

**Solution**: Initialize in main() or init() function.

| Pattern | Behavior | Solution |
|---------|----------|----------|
| dot import | Framework detected but middleware not added | Use regular import |
| Global variable init | Middleware not added | Initialize in main()/init() |
| vendor directory | Auto-skipped (expected) | - |

---

## Migration from Manual to Auto Instrumentation

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

| Code Type | remove Behavior |
|-----------|-----------------|
| Standard patterns (middleware, sql.Open, etc.) | Auto-removed |
| Custom code (trace.Start, etc.) | Manual check required |

---

## Related Documents

- [Build Wrapper Mode](./build-wrapper.md) — Build wrapper details
- [Source Inject Mode](./source-inject.md) — Direct source modification
- [Multi-Module Projects](./multi-module.md) — External module and multi-module instrumentation
- [Configuration Guide](./config.md) — Config file, presets, packages
- [Custom Instrumentation](./custom-instrumentation.md) — User-defined rules
- [Instrumentation Rules](./instrumentation-rules.md) — Transformation patterns per framework
- [Troubleshooting](./troubleshooting.md) — Common errors and solutions
- [whatap/go-api](https://github.com/whatap/go-api) — WhaTap Go monitoring library
- [whatap/go-api-example](https://github.com/whatap/go-api-example) — Usage examples
