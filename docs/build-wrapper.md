# Build Wrapper Mode

Wraps `go` commands to automatically inject instrumentation code.

## Overview

```bash
# Build (no init required)
whatap-go-inst go build ./...
```

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│  whatap-go-inst go build ./...                              │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│  1. Copy source to temp directory                           │
│  2. Inject instrumentation code                             │
│  3. go get github.com/whatap/go-api@latest                  │
│  4. go mod tidy                                             │
│  5. go build                                                │
│  6. Save transformed source in whatap-instrumented/         │
└─────────────────────────────────────────────────────────────┘
```

## Key Features

1. **No init required**
   - Just run `whatap-go-inst go build ./...`
   - Dependencies are automatically added

2. **Original go.mod is not modified**
   - `go get`, `go mod tidy` only run in temp folder
   - Original source remains unchanged

3. **Downloads @latest every time**
   - `go get github.com/whatap/go-api@latest` runs automatically
   - New go-api versions are automatically applied

4. **whatap-instrumented/ directory created**
   - Can check transformed source code
   - Useful for debugging
   - Use `--no-output` to disable

## Usage

### Recommended Workflow

```bash
# Just build (no setup required)
whatap-go-inst go build ./...
```

### Basic Build

```bash
# Build current directory
whatap-go-inst go build .

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
# Direct run
whatap-go-inst go run .

# Pass arguments
whatap-go-inst go run . --port 8080
```

### Test

```bash
# All tests
whatap-go-inst go test ./...

# Specific test
whatap-go-inst go test -v -run TestName ./pkg/...
```

### Install

```bash
whatap-go-inst go install ./...
```

## Example

### Gin Application

**Original code (main.go):**
```go
package main

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
    r.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "hello"})
    })
    r.Run(":8080")
}
```

## Debug Mode

Set environment variable to see debug output:

```bash
GO_API_AST_DEBUG=1 whatap-go-inst go build ./...
```

Output example:
```
[whatap-go-inst] output: /tmp/go-build123/main.a
[whatap-go-inst] transformed: [/tmp/whatap-go-inst-456/main.go]
```

## Advantages

| Advantage | Description |
|-----------|-------------|
| **Simple** | Just add `whatap-go-inst` before command |
| **Preserves original** | Source code is not modified at all |
| **Selective** | Use only when instrumentation is needed |
| **CI/CD friendly** | On/off control via environment variables |

## Disadvantages

| Disadvantage | Description |
|--------------|-------------|
| **Build time increase** | Overhead from AST transformation |
| **Requires whatap-go-inst** | Tool must be installed in build environment |

## CI/CD Configuration Examples

### GitHub Actions

```yaml
name: Build with Instrumentation

on: [push]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install whatap-go-inst
        run: go install github.com/whatap/go-api-inst/cmd/whatap-go-inst@latest

      - name: Build with instrumentation
        run: whatap-go-inst go build -o myapp ./...

      - name: Upload artifact
        uses: actions/upload-artifact@v3
        with:
          name: myapp
          path: myapp
```

### Dockerfile

```dockerfile
FROM golang:1.21 AS builder

WORKDIR /app
COPY . .

# Install whatap-go-inst
RUN go install github.com/whatap/go-api-inst/cmd/whatap-go-inst@latest

# Build with instrumentation
RUN whatap-go-inst go build -o /app/myapp .

FROM alpine:latest
COPY --from=builder /app/myapp /usr/local/bin/
CMD ["myapp"]
```

## Troubleshooting

### No Dependency Error

**Symptom:**
```
Error: github.com/whatap/go-api not found in go.mod.
```

**Cause:**
- go.mod doesn't have whatap/go-api dependency

**Solution:**

Add dependency first:
```bash
go get github.com/whatap/go-api@latest
whatap-go-inst go build ./...
```

### Switching from Manual to Automatic Instrumentation

If you previously added go-api code manually:

```bash
# 1. Remove existing whatap code
whatap-go-inst remove -s . -o ./cleaned

# 2. Check diff (ensure no custom code is lost)
diff -r . ./cleaned

# 3. Replace if no issues
cp -r ./cleaned/* ./

# 4. Switch to automatic instrumentation
whatap-go-inst go build ./...
```

**Note:** Standard patterns (middleware, sql.Open, etc.) are auto-removed, but custom code (trace.Start, etc.) requires manual review.

### Build Cache Issues

**Symptom:**
- Code changes not reflected

**Solution:**
```bash
go clean -cache
whatap-go-inst go build ./...
```

## Important Notes

1. **External packages are not transformed**: Packages in GOMODCACHE are skipped.
2. **Standard library is not transformed**: Packages in GOROOT are skipped.
3. **Test files are not transformed**: `_test.go` files are skipped.
4. **Pre-add dependency required**: Run `go get github.com/whatap/go-api@latest` first.

## Next Steps

- [Direct Source Modification Mode](./source-inject.md)
- [Multi-Module Projects](./multi-module.md)
