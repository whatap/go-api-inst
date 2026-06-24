# Build Wrapper Mode

`whatap-go-inst` wraps `go` commands and transforms your Go source during compilation, without changing the files on disk. This is the **only** build mode since v0.6.0 — the legacy `--wrap` / `--no-output` flags and the `inject` / `generate` / `init` / `uninit` subcommands were removed. The build wrapper handles dependency add + instrumentation + build in a single step (no `init` / `go mod tidy` pre-step required).

## Overview

```bash
# Build (no init required, no source modification on disk)
whatap-go-inst go build ./...

# Specify output binary
whatap-go-inst go build -o myapp .

# Run / Test also wrapped
whatap-go-inst go run .
whatap-go-inst go test ./...
```

- The original `go.mod` / source tree is **never** modified.
- `github.com/whatap/go-api` is added to `go.mod` automatically during the build (and rolled back for vendor projects).
- Uses Go's build cache and incremental builds transparently.

---

## How It Works

1. Auto-add `github.com/whatap/go-api` dependency to `go.mod` (if missing) and run `go mod tidy`.
2. **[vendor projects]** Make sure the whatap packages are present in `vendor/`.
3. Look up where the whatap and standard-library packages are compiled, so they can be linked later.
4. Build the project, transforming the source as each package is compiled. (Internally this uses Go's `-toolexec` mechanism — you never invoke it directly.)
5. Make the newly added packages available to the Go linker.
6. **[vendor]** Roll back `go.mod` / `go.sum` / `vendor/` to the original state.

The transformed source is not written to disk unless you ask for it (see next section).

---

## Inspect the transformed source (`--output`)

By default no instrumented source is saved (zero I/O overhead). Add `--output` to dump a buildable copy of the transformed tree:

```bash
# Default output directory: whatap-instrumented/
whatap-go-inst --output go build ./...

# Custom path
whatap-go-inst --output=./instrumented go build ./...

# Or via environment variable
GO_API_AST_OUTPUT_DIR=./instrumented whatap-go-inst go build ./...
```

The output directory is a **complete, buildable** Go project: transformed `.go` files, `go.mod`, `go.sum`, and (for `--external-module`) the `_modules/` subtree with `replace` directives. You can `cd ./instrumented && go build` without `whatap-go-inst` to verify the injection result.

---

## Example

**Original code (`main.go`):**
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

**Code actually compiled into the binary:**
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

1. **External packages are not transformed by default.** Use `--external-module` to instrument specific GOMODCACHE modules. See [Multi-Module Projects](./multi-module.md).
2. **Standard library is not transformed.** Packages in GOROOT are skipped.
3. **Test files are not transformed.** `_test.go` files are skipped.
4. **Legacy subcommands removed (v0.6.0):** `whatap-go-inst inject` / `generate` / `init` / `uninit` and the `--wrap` / `--no-output` flags no longer exist. Use `whatap-go-inst go build [--output[=DIR]]` for every workflow. The build wrapper handles dependency add + instrumentation + build in one step. (`whatap-go-inst remove` is still available for stripping manually written instrumentation calls.)

---

## Next Steps

- [Source Inspection (`--output`)](./source-inject.md) — inspecting / diffing transformed source
- [Multi-Module Projects](./multi-module.md)
