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
- No setup required — just build
- Does not modify the original source tree
- Auto-adds dependencies in Go's `$WORK` directory
- No instrumented source saved to disk by default — add `--output` (or `GO_API_AST_OUTPUT_DIR=…`) to dump a buildable copy

---

## Global Options

| Option | Short | Default | Description |
|--------|-------|---------|-------------|
| `--config` | | `.whatap/config.yaml` | Config file path |
| `--verbose` | `-v` | `false` | Verbose output (includes transformation details) |
| `--quiet` | `-q` | `false` | Summary only |
| `--report` | | | JSON report file path |
| `--output` | | *(unset)* | Dump the transformed source. `--output` alone → `whatap-instrumented/`. `--output=DIR` → custom path. Omit the flag entirely to skip saving and avoid any I/O. Can also be set via `GO_API_AST_OUTPUT_DIR`. |
| `--error-tracking` | | `false` | Inject error tracking code (`trace.Error` in `if err != nil` patterns) |
| `--external-module` | | | External module to instrument from GOMODCACHE (repeatable, comma-separated, wildcard supported) |

> Legacy flags `--wrap` and `--no-output`, and subcommands `inject` / `generate` / `init` / `uninit`, were **removed in v0.6.0**. The build-wrapper (`whatap-go-inst go build …`) is the single workflow; `--output` replaces the previous inject + copy pattern. `--fast` is accepted but hidden (no-op) to keep old scripts working. The `remove` subcommand is still shipped — see "Other Subcommands" below.

### JSON Report

```bash
whatap-go-inst --report=report.json go build ./...
```

Report structure:
```json
{
  "timestamp": "2026-04-24T15:00:00+09:00",
  "command": "go build",
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

### `whatap-go-inst go` - Build Wrapper

Wraps go commands to auto-inject monitoring code during build. This is the only build mode since v0.6.0.

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

# Dump instrumented source to whatap-instrumented/ (default)
whatap-go-inst --output go build ./...

# Dump to a custom path
whatap-go-inst --output=./instrumented go build ./...

# Instrument external module from GOMODCACHE
whatap-go-inst --external-module=mycompany.com/internal/lib go build ./...

# Wildcard external module
whatap-go-inst --external-module="mycompany.com/internal/*" go build ./...
```

#### Internal Operation

1. Auto-add `github.com/whatap/go-api` to `go.mod` (if missing) and run `go mod tidy`.
2. **[vendor]** Make sure the whatap packages are present in `vendor/`.
3. Look up where the whatap and standard-library packages are compiled, so they can be linked later.
4. Build the project, transforming the source as each package is compiled (in Go's temporary build directory, not your source tree). Internally this uses Go's `-toolexec` mechanism — you never invoke it directly.
5. Make the newly added packages available to the Go linker.
6. If `--output` / `GO_API_AST_OUTPUT_DIR` is set, save a buildable copy of the transformed tree to that directory.
7. **[vendor]** Roll back `go.mod` / `go.sum` / `vendor/` to the original state.

> **Details**: [Build Wrapper Mode](./build-wrapper.md) · [Inspect Transformed Source](./source-inject.md)

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

The build wrapper does not modify your originals, so the recommended path is to first strip the manually written `go-api` calls from your tree, then let the wrapper re-inject equivalent calls during the build. `whatap-go-inst remove` automates the strip step.

> **What `remove` does — and does not — clean up:** the wrapper rewrites code in a per-build scratch directory (`$WORK`), so auto-instrumented call sites never reach your source tree. Therefore `whatap-go-inst remove` only targets **manually written** `go-api` imports and calls — it is a cleanup tool for hand-rolled instrumentation, not an inverse of the build wrapper. Stopping the wrapper is enough to revert auto-injected changes; you do not need to run `remove` first.

`whatap-go-inst remove` automates the strip step:

```bash
# 1. Remove manually written go-api lines from your source. Two options:
#    a) Use the bundled command — handles imports, trace.Init/Shutdown,
#       framework middleware, common wrapper unwraps, and standalone
#       trace.Step / AddHook calls in one pass:
whatap-go-inst remove --src . --output ./cleaned

#    b) Or strip them yourself with git grep / codemod / editor search-replace.
#       Typical patterns:
#         trace.Start(…), trace.End(…), defer trace.End(…)
#         <router>.Use(whatap*.Middleware())
#         whatap*.WrapHandler(…), whatap*.Wrap(…)
#         import "github.com/whatap/go-api/…"

# 2. Verify the cleaned tree compiles without whatap-go-inst
go build ./...

# 3. Let the build wrapper re-inject instrumentation
whatap-go-inst go build ./...
```

> The legacy `--all` flag is now a deprecated no-op (manual pattern removal is the default).

## Other Subcommands

| Command | Purpose |
|---------|---------|
| `whatap-go-inst remove --src SRC [--output OUT]` | Strip **manually written** `whatap/go-api` monitoring code from a source tree. Useful for migrating from hand-rolled instrumentation to the build wrapper, or when retiring instrumentation. (`--all` flag deprecated — manual pattern removal is the default.) |
| `whatap-go-inst version` | Print version, git commit, and build date. |

| Code Type | Migration notes |
|-----------|-----------------|
| Standard patterns (middleware, sql.Open, etc.) | Safe to strip — build wrapper re-injects equivalent calls |
| Custom code (`trace.Start`, custom hooks) | Keep manually if your logic needs ctx threading beyond what auto-inject provides |

---

## Related Documents

- [Build Wrapper Mode](./build-wrapper.md) — Build wrapper details
- [Inspect Transformed Source (`--output`)](./source-inject.md) — Dump instrumented source for review
- [Multi-Module Projects](./multi-module.md) — External module and multi-module instrumentation
- [Configuration Guide](./config.md) — Config file, presets, packages
- [Custom Instrumentation](./custom-instrumentation.md) — User-defined rules
- [Instrumentation Rules](./instrumentation-rules.md) — Transformation patterns per framework
- [Troubleshooting](./troubleshooting.md) — Common errors and solutions
- [whatap/go-api](https://github.com/whatap/go-api) — WhaTap Go monitoring library
- [whatap/go-api-example](https://github.com/whatap/go-api-example) — Usage examples
