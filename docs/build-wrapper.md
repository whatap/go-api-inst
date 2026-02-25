# Build Wrapper Mode

Wraps `go` commands to automatically inject instrumentation code.

## Overview

```bash
# Build (no init required)
whatap-go-inst go build ./...
```

---

## How It Works

1. Copy source to temp directory
2. Inject instrumentation code
3. `go get github.com/whatap/go-api@latest` + `go mod tidy`
4. Execute specified go command
5. Save transformed source in `whatap-instrumented/` (unless `--no-output`)
6. Delete temp directory

**Key characteristics:**
- No init required — just build
- Original go.mod is not modified (everything runs in temp directory)
- Latest go-api version is automatically downloaded
- `whatap-instrumented/` directory shows transformed source for debugging

---

## Usage

### Build

```bash
# Build all packages
whatap-go-inst go build ./...

# Specify output file
whatap-go-inst go build -o myapp .

# Disable instrumented source output
whatap-go-inst --no-output go build ./...

# Save instrumented source to custom path
whatap-go-inst --output ./instrumented go build ./...
```

### Run

```bash
whatap-go-inst go run .
whatap-go-inst go run . --port 8080
```

### Test

```bash
whatap-go-inst go test ./...
whatap-go-inst go test -v -run TestName ./pkg/...
```

---

## Example

**Original code (main.go):**
```go
package main

import "github.com/gin-gonic/gin"

func main() {
    r := gin.Default()
    r.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "hello"})
    })
    r.Run(":8080")
}
```

**Build:**
```bash
whatap-go-inst go build -o myapp .
```

**Code actually running in the built binary:**
```go
package main

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
```

---

## Dockerfile

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

# WhaTap config
RUN echo "license=your-license-key" > whatap.conf && \
    echo "whatap.server.host=13.124.11.223" >> whatap.conf && \
    echo "app_name=myapp" >> whatap.conf

ENV WHATAP_HOME=/app

EXPOSE 8080
CMD ["/bin/sh", "-c", "/usr/whatap/agent/whatap-agent start && ./server"]
```

> Get `license` and `whatap.server.host` from [WhaTap Console](https://service.whatap.io) after creating a project.

---

## Debug Mode

```bash
GO_API_AST_DEBUG=1 whatap-go-inst go build ./...
```

---

## Notes

1. **External packages are not transformed by default**: Use `--external-module` to instrument specific GOMODCACHE modules. See [Multi-Module Projects](./multi-module.md).
2. **Standard library is not transformed**: Packages in GOROOT are skipped.
3. **Test files are not transformed**: `_test.go` files are skipped.

---

## Next Steps

- [Source Inject Mode](./source-inject.md) — Direct source modification
- [Multi-Module Projects](./multi-module.md)
