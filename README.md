# whatap-go-inst

Go AST-based automatic instrumentation tool for source code.

Automatically injects/removes `github.com/whatap/go-api` monitoring code, similar to Datadog Orchestrion.

## Installation

### Option 1: Download Binary (Recommended)

Download pre-built binaries from [GitHub Releases](https://github.com/whatap/go-api-inst/releases).

```bash
# Example: Linux amd64
VERSION=0.5.0
curl -LO https://github.com/whatap/go-api-inst/releases/download/v${VERSION}/whatap-go-inst_linux_amd64.tar.gz
tar xzf whatap-go-inst_linux_amd64.tar.gz
sudo mv whatap-go-inst /usr/local/bin/
```

Available binaries:
- `whatap-go-inst_linux_amd64.tar.gz` - Linux x86_64
- `whatap-go-inst_linux_arm64.tar.gz` - Linux ARM64
- `goinst_linux_amd64.tar.gz` - Short alias (optional)
- `goinst_linux_arm64.tar.gz` - Short alias (optional)

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

### Method 2: Direct Source Modification

Modify source code directly and output to a separate directory.

```bash
# Inject monitoring code
whatap-go-inst inject --src ./myapp --output ./instrumented

# Remove monitoring code
whatap-go-inst remove --src ./instrumented --output ./clean
```

### Docker Example

Download the binary from GitHub Releases for Docker builds.

```dockerfile
# Stage 1: Build with instrumentation
FROM golang:1.21-alpine AS builder

# Install whatap-go-inst
RUN wget -qO- https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_amd64.tar.gz | tar xz -C /usr/local/bin/

WORKDIR /app
COPY . .

# Build with instrumentation (no init required)
RUN whatap-go-inst --no-output go build -o /app/server .

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

EXPOSE 8080
CMD ["/bin/sh", "-c", "/usr/whatap/agent/whatap-agent start && ./server"]
```

> **Note**: Get `license` and `whatap.server.host` from [WhaTap Console](https://service.whatap.io) after creating a project.

## Commands

| Command | Description |
|---------|-------------|
| `whatap-go-inst go <cmd>` | Wrap go commands (build, run, test, install) |
| `whatap-go-inst inject` | Inject monitoring code into source |
| `whatap-go-inst remove` | Remove monitoring code from source |
| `whatap-go-inst version` | Print version information |

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

// Chi
r := chi.NewRouter()
r.Use(whatapchi.Middleware)  // Auto-injected (function value)

// net/http (handler wrapping)
mux.HandleFunc("/", whataphttp.Func(handler))  // Auto-wrapped
mux.Handle("/api", whataphttp.Handler(h))      // Auto-wrapped
```

## Implementation Status

| Feature | Status | Notes |
|---------|--------|-------|
| trace.Init/Shutdown injection | Done | At main() function start |
| Auto import addition | Done | Version-specific paths (v2, v4) |
| Web framework middleware injection | Done | Gin, Echo, Fiber, Chi, Gorilla, FastHTTP |
| net/http handler wrapping | Done | whataphttp.Func(), whataphttp.Handler() |
| HTTP client wrapping | Done | http.Get, http.DefaultClient, etc. |
| DB instrumentation | Done | sql, sqlx, GORM |
| Redis instrumentation | Done | go-redis v8/v9, Redigo |
| MongoDB instrumentation | Done | CommandMonitor-based |
| gRPC/Kafka instrumentation | Done | Interceptor-based |
| Code removal | Done | Original restored after inject→remove |
| Log library instrumentation | Done | log, logrus, zap |
| Transformer pattern | Done | 22 package-specific transformers (ast/packages/) |
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
- `github.com/aerospike/aerospike-client-go` (v6, v8)

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

## Related Projects

| Project | Description |
|---------|-------------|
| [go-api](https://github.com/whatap/go-api) | WhaTap Go monitoring library |
| [go-api-example](https://github.com/whatap/go-api-example) | go-api usage examples |

## Documentation

For detailed developer guides, see the [docs/](./docs/) directory:

- [Build Wrapper Mode](./docs/build-wrapper.md) - Simplest approach (recommended)
- [Direct Source Modification](./docs/source-inject.md) - Separate directory output
- [Transformation Rules](./docs/instrumentation-rules.md) - Framework-specific patterns
- [User Guide](./docs/user-guide.md) - Detailed usage
- [Custom Instrumentation](./docs/custom-instrumentation.md) - Define custom rules via YAML
- [Multi-Module Projects](./docs/multi-module.md) - Working with multiple Go modules
- [Configuration](./docs/config.md) - Configuration options
- [Troubleshooting](./docs/troubleshooting.md) - Error solutions

## License

Apache 2.0
