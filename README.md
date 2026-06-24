# whatap-go-inst

Go AST-based automatic instrumentation tool — adds WhaTap monitoring to your Go application at build time, with no manual code changes.

It injects (and can remove) `github.com/whatap/go-api` monitoring code during compilation, so your source tree stays unchanged.

## Installation

### Option 1: Download Binary (Recommended)

Download pre-built binaries from [GitHub Releases](https://github.com/whatap/go-api-inst/releases).

```bash
# Linux amd64
curl -sSL https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_amd64.tar.gz | tar xz
sudo mv whatap-go-inst /usr/local/bin/

# Linux arm64
curl -sSL https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_arm64.tar.gz | tar xz
sudo mv whatap-go-inst /usr/local/bin/

# Specific version (e.g., v0.6.0)
curl -sSL https://github.com/whatap/go-api-inst/releases/download/v0.6.0/whatap-go-inst_linux_amd64.tar.gz | tar xz
```

### Option 2: Go Install

```bash
go install github.com/whatap/go-api-inst/cmd/whatap-go-inst@latest
```

### Option 3: Build from Source

```bash
git clone https://github.com/whatap/go-api-inst
cd go-api-inst
go build -o whatap-go-inst .
```

## Usage

### Method 1: Build Wrapper (Recommended)

The simplest method. Just prefix your `go` commands with `whatap-go-inst`.

```bash
# Build (no init required)
whatap-go-inst go build ./...
whatap-go-inst go build -o myapp .

# Run
whatap-go-inst go run .

# Test
whatap-go-inst go test ./...
```

Original source code remains unchanged; instrumentation is only applied to the build output.

### Method 2: Inspect the instrumented source (`--output`)

Keep Method 1's build wrapper workflow, but also dump the transformed `.go` files to a directory for inspection.

```bash
# Build and dump instrumented source to ./instrumented/
whatap-go-inst --output=./instrumented go build -o myapp ./...

# Or via environment variable
GO_API_AST_OUTPUT_DIR=./instrumented whatap-go-inst go build -o myapp ./...
```

The original source remains unchanged. `./instrumented/` will contain a buildable copy of the transformed sources (including `go.mod` / `go.sum`). The legacy `inject` / `generate` CLI subcommands were removed in v0.6.0; this fast-mode `--output` is the supported replacement. (`whatap-go-inst remove` is still available for stripping manually written monitoring code, e.g. when migrating an existing project — see the `Commands` table.)

### Docker Example

Download the binary from GitHub Releases for Docker builds.

```dockerfile
# Stage 1: Build with instrumentation
FROM golang:1.21-alpine AS builder

# Install whatap-go-inst
RUN wget -qO- https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_amd64.tar.gz | tar xz -C /usr/local/bin/

WORKDIR /app
COPY . .

# Build with instrumentation
RUN whatap-go-inst go build -o /app/server .

# Stage 2: Run with WhaTap agent
FROM alpine:latest

# Install WhaTap agent
RUN wget -qO- https://s3.ap-northeast-2.amazonaws.com/repo.whatap.io/alpine/x86_64/whatap-agent.tar.gz | tar xz -C /

WORKDIR /app
COPY --from=builder /app/server .

# Create WhaTap config file
RUN echo "license=your-license-key" > whatap.conf && \
    echo "whatap.server.host=13.124.11.223" >> whatap.conf && \
    echo "app_name=myapp" >> whatap.conf

ENV WHATAP_HOME=/app

EXPOSE 8080
CMD ["/bin/sh", "-c", "/usr/whatap/agent/whatap-agent start && ./server"]
```

> **Note**: Get `license` and `whatap.server.host` from [WhaTap Console](https://service.whatap.io) after creating a project.

## Commands

| Command | Description |
|---------|-------------|
| `whatap-go-inst go <cmd>` | Build wrapper — wraps go commands (build, run, test, install) |
| `whatap-go-inst remove --src SRC [--output OUT]` | Strip **manually written** `whatap/go-api` monitoring code from a source tree (manual cleanup / migration). The legacy `--all` flag is a deprecated no-op since v0.6.0. |
| `whatap-go-inst version` | Print version information |

> The `inject` / `generate` / `init` / `uninit` subcommands were removed in v0.6.0. The build wrapper handles dependency add + instrumentation + build in a single step (`whatap-go-inst go build ./...`); use `--output` to dump the transformed source for inspection.

## Injected Monitoring Patterns

### Import Addition
```go
import (
    "github.com/whatap/go-api/trace"
    "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
)
```

### main() Function Initialization
```go
func main() {
    trace.Init(nil)
    defer trace.Shutdown()
    // ... existing code
}
```

### Web Framework Middleware (Auto-injected)
```go
// Gin
r := gin.Default()
r.Use(whatapgin.Middleware())  // Auto-injected

// Echo v4
e := echo.New()
e.Use(whatapecho.Middleware())  // Auto-injected

// Fiber v2
app := fiber.New()
app.Use(whatapfiber.Middleware())  // Auto-injected

// Chi (in-place wrapping)
r := whatapchi.WrapRouter(chi.NewRouter())

// Gorilla Mux (in-place wrapping)
r := whatapmux.WrapRouter(mux.NewRouter())

// net/http (handler wrapping)
mux.HandleFunc("/", whataphttp.Func(handler))      // Auto-wrapped
mux.Handle("/api", whataphttp.WrapHandler(h))      // Auto-wrapped
```

## Implementation Status

| Feature | Status | Notes |
|---------|--------|-------|
| trace.Init/Shutdown injection | Done | At main() function start |
| Auto import addition | Done | Version-specific paths (v2, v4) |
| Web framework middleware injection | Done | Gin, Echo, Fiber, Chi, Gorilla, FastHTTP |
| net/http handler wrapping | Done | whataphttp.Func(), whataphttp.WrapHandler() |
| HTTP client wrapping | Done | http.Get, http.DefaultClient, etc. |
| DB instrumentation | Done | sql, sqlx, GORM |
| Redis instrumentation | Done | go-redis v8/v9, Redigo |
| MongoDB instrumentation | Done | CommandMonitor-based |
| gRPC/Kafka instrumentation | Done | Interceptor-based |
| Code removal | Done | `whatap-go-inst remove` strips manually inserted `go-api` calls; build-wrapper flow leaves originals untouched |
| Log library instrumentation | Done | log, logrus, zap |
| LLM SDK instrumentation | Done | sashabaranov, Eino (eino-ext), Anthropic, openai-go — auto-inject adapters, nested module, `llm_enabled=true` |
| Instrumentation rules | Done | Unified engine — 115 built-in rules across 10 instrumentation types |
| Custom instrumentation | Done | inject, replace, hook, add, transform rules |

## Supported Frameworks

### Web Frameworks
- `github.com/gin-gonic/gin`
- `github.com/labstack/echo/v4` (including v3)
- `github.com/gofiber/fiber/v2`
- `github.com/go-chi/chi/v5`
- `github.com/gorilla/mux`
- `github.com/valyala/fasthttp`
- `net/http` (server + client)

### Databases
- `database/sql`
- `github.com/jmoiron/sqlx`
- `gorm.io/gorm`
- `github.com/jinzhu/gorm`

### Redis
- `github.com/redis/go-redis/v9`
- `github.com/go-redis/redis/v8`
- `github.com/gomodule/redigo`

### NoSQL
- `go.mongodb.org/mongo-driver/mongo`
- `github.com/aerospike/aerospike-client-go` (v6)

### Message Queue / RPC / Cloud
- `google.golang.org/grpc`
- `github.com/IBM/sarama` (Kafka)
- `github.com/Shopify/sarama` (Kafka)
- `k8s.io/client-go/kubernetes`

### Log Libraries
- `fmt` (standard library, Print/Printf/Println)
- `log` (standard library)
- `github.com/sirupsen/logrus`
- `go.uber.org/zap`

### LLM SDKs (requires `llm_enabled=true`)
- `github.com/sashabaranov/go-openai`
- `github.com/cloudwego/eino-ext/components/model/openai`, `.../claude`
- `github.com/anthropics/anthropic-sdk-go`
- `github.com/openai/openai-go`

> LLM adapters live in the nested module `github.com/whatap/go-api/instrumentation/llm`, which the build wrapper adds automatically. See [LLM Monitoring](./docs/llm-monitoring.md) for details.

## Related Projects

| Project | Description |
|---------|-------------|
| [go-api](https://github.com/whatap/go-api) | WhaTap Go monitoring library |
| [go-api-example](https://github.com/whatap/go-api-example) | go-api usage examples |

## Documentation

For detailed developer guides, see the [docs/](./docs/) directory:

- [Build Wrapper Mode](./docs/build-wrapper.md) - Simplest approach (recommended)
- [Inspect Transformed Source (`--output`)](./docs/source-inject.md) - Dump instrumented source for review
- [Transformation Rules](./docs/instrumentation-rules.md) - Framework-specific patterns
- [LLM Monitoring](./docs/llm-monitoring.md) - LLM SDK auto-instrumentation (sashabaranov, Eino, Anthropic, openai-go)
- [User Guide](./docs/user-guide.md) - Detailed usage
- [Custom Instrumentation](./docs/custom-instrumentation.md) - Define custom rules via YAML
- [Multi-Module Projects](./docs/multi-module.md) - Working with multiple Go modules
- [Configuration](./docs/config.md) - Configuration options
- [Troubleshooting](./docs/troubleshooting.md) - Error solutions

## License

Apache 2.0
