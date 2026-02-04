# whatap-go-inst Developer Guide

Go AST-based automatic instrumentation tool for WhaTap APM monitoring.

## Quick Start

### Method 1: Build Wrapper (Recommended)

```bash
# Install
go install github.com/whatap/go-api-inst/cmd/whatap-go-inst@latest

# Build (no init required)
whatap-go-inst go build ./...

# Run
whatap-go-inst go run .
```

### Method 2: Direct Source Modification

```bash
whatap-go-inst inject -s ./myapp -o ./instrumented
cd instrumented && go build .
```

## Instrumentation Modes

| Mode | Command | Requires init | Source Copy | Use Case |
|------|---------|---------------|-------------|----------|
| Build Wrapper | `whatap-go-inst go build` | No | Yes | **Recommended** |
| Direct Modify | `whatap-go-inst inject` | No | Yes | Review/Compare |

### Build Wrapper Mode Details

**No setup required - just build:**
```bash
whatap-go-inst go build ./...
```

**Instrumented source output:**

The instrumented source is automatically saved to `whatap-instrumented/` directory after each build.

```
myapp/
├── main.go                    # Original source
├── whatap-instrumented/       # Auto-generated (add to .gitignore)
│   ├── main.go                # Instrumented source
│   ├── go.mod
│   └── go.sum
└── myapp.exe                  # Built binary
```

To customize the output path:
```bash
whatap-go-inst --output ./custom-dir go build ./...
```

To disable output (no directory created):
```bash
whatap-go-inst --no-output go build ./...
```

**Saving Instrumented Source (optional):**
```bash
# Save instrumented source with --output flag
whatap-go-inst --output ./instrumented go build ./...

# Only .go files used in build are saved (not entire source)
```

## Mode Selection Guide

| Mode | Source Changes | Complexity | Recommended When |
|------|----------------|------------|------------------|
| [Build Wrapper](./build-wrapper.md) | No | Low | **Default**, simplest |
| [Direct Modification](./source-inject.md) | Yes (separate dir) | Medium | Compare/review transformations |

## Documentation

### Instrumentation Modes

1. **[Build Wrapper Mode](./build-wrapper.md)**
   - Simplest approach
   - `whatap-go-inst go build ./...`
   - No original source changes

2. **[Multi-Module Projects](./multi-module.md)**
   - Instrumenting projects with multiple modules
   - replace directive handling
   - Mode comparison and considerations

### Configuration

3. **[Configuration Guide](./config.md)**
   - Config file format and location
   - Preset options (full, minimal, web, database, external, log, custom)
   - Per-package enable/disable
   - Environment variables

4. **[Custom Instrumentation Guide](./custom-instrumentation.md)**
   - Define custom instrumentation rules in YAML
   - 5 rule types: inject, replace, hook, add, transform
   - Wildcard pattern matching, error handling policies

### Reference

5. **[Instrumentation Rules](./instrumentation-rules.md)**
   - Detailed code transformation patterns per framework

6. **[Remove Patterns Checklist](./remove-patterns.md)**
   - Pattern removal capabilities of remove command
   - AST removal support by code format

7. **[Release Guide](./release.md)**
   - Binary distribution with goreleaser
   - GitHub Actions auto-release
   - Go version compatibility

## Supported Frameworks

### Web Frameworks

| Framework | Import Path | Middleware/Wrapper | Status |
|-----------|-------------|-------------------|--------|
| Gin | `github.com/gin-gonic/gin` | `whatapgin.Middleware()` | Supported |
| Echo | `github.com/labstack/echo`, `echo/v4` | `whatapecho.Middleware()` | Supported |
| Fiber | `github.com/gofiber/fiber/v2` | `whatapfiber.Middleware()` | Supported |
| Chi | `github.com/go-chi/chi/v5` | `whatapchi.Middleware` | Supported |
| Gorilla Mux | `github.com/gorilla/mux` | `whatapmux.Middleware` | Supported |
| net/http | `net/http` | `whataphttp.Func()`, `whataphttp.Handler()` | Supported |
| FastHTTP | `github.com/valyala/fasthttp` | `whatapfasthttp.Middleware()` | Supported |

### Database

| Library | Import Path | Transformation | Status |
|---------|-------------|----------------|--------|
| database/sql | `database/sql` | `whatapsql.Open()` | Supported |
| sqlx | `github.com/jmoiron/sqlx` | `whatapsqlx.Open()` | Supported |
| GORM (gorm.io) | `gorm.io/gorm` | `whatapgorm.Open()` | Supported |
| GORM (jinzhu) | `github.com/jinzhu/gorm` | `whatapgorm.Open()` | Supported |

### HTTP Client

| Pattern | Transformation | Status |
|---------|----------------|--------|
| `http.Get(url)` | `whataphttp.HttpGet(ctx, url)` | Supported |
| `http.Post(...)` | `whataphttp.HttpPost(ctx, ...)` | Supported |
| `http.PostForm(...)` | `whataphttp.HttpPostForm(ctx, ...)` | Supported |
| `http.DefaultClient.Get(url)` | `whataphttp.HttpGet(ctx, url)` | Supported |
| `http.Client{}` | Auto-wrap Transport | Supported |

### External Services

| Library | Import Path | Transformation | Status |
|---------|-------------|----------------|--------|
| Sarama (IBM) | `github.com/IBM/sarama` | Interceptor injection | Supported |
| Sarama (Shopify) | `github.com/Shopify/sarama` | Interceptor injection | Supported |
| gRPC | `google.golang.org/grpc` | Server/Client Interceptor | Supported |
| Kubernetes | `k8s.io/client-go` | `config.Wrap()` | Supported |

### NoSQL / Cache

| Library | Import Path | Transformation | Status |
|---------|-------------|----------------|--------|
| MongoDB | `go.mongodb.org/mongo-driver/mongo` | `whatapmongo.Connect()` | Supported |
| go-redis v9 | `github.com/redis/go-redis/v9` | `whatapgoredis.NewClient()` | Supported |
| go-redis v8 | `github.com/go-redis/redis/v8` | `whatapgoredis.NewClient()` | Supported |
| Redigo | `github.com/gomodule/redigo` | `whatapredigo.Dial()` | Supported |
| Aerospike v6/v8 | `github.com/aerospike/aerospike-client-go` | `whatapsql.Wrap()` | Supported |

### Logging Libraries

| Library | Import Path | Transformation | Status |
|---------|-------------|----------------|--------|
| fmt | `fmt` | `whatapfmt.Print/Printf/Println()` | Supported |
| log | `log` | `log.SetOutput(logsink.GetTraceLogWriter(os.Stderr))` | Supported |
| logrus | `github.com/sirupsen/logrus` | `logrus.SetOutput(logsink.GetTraceLogWriter(os.Stderr))` | Supported |
| zap | `go.uber.org/zap` | `logsink.HookStderr()` | Supported |

### Not Yet Implemented

| Library | Import Path | Status | Notes |
|---------|-------------|--------|-------|
| kafka-go | `github.com/segmentio/kafka-go` | Planned | Kafka alternative |
| pgx | `github.com/jackc/pgx/v5` | Planned | PostgreSQL driver |
| aws-sdk-go-v2 | `github.com/aws/aws-sdk-go-v2` | Planned | AWS SDK |

## Injected Code Patterns

### Import

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
    // ...
}
```

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `GO_API_AST_DEBUG` | Enable debug output | `GO_API_AST_DEBUG=1` |

## Limitations and Notes

### Unsupported Code Patterns

The following patterns are not auto-instrumented. You need to add WhaTap code manually.

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

## Troubleshooting

### Instrumentation Not Applied

1. Clear build cache
   ```bash
   go clean -cache
   ```

2. Check with debug mode
   ```bash
   GO_API_AST_DEBUG=1 whatap-go-inst go build ./...
   ```

### Compilation Errors

1. Add whatap/go-api dependency
   ```bash
   go get github.com/whatap/go-api@latest
   go mod tidy
   ```

## Related Projects

- [whatap/go-api](https://github.com/whatap/go-api) - Monitoring library
- [whatap/go-api-example](https://github.com/whatap/go-api-example) - Usage examples
