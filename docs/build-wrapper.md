# Build Wrapper Mode

Wraps `go` commands to automatically inject instrumentation code.

## Overview

```bash
# 1. Initialize (once, in go.mod directory)
whatap-go-inst init

# 2. Add dependencies (required!)
go get github.com/whatap/go-api@latest
go mod tidy

# 3. Instrumented build
whatap-go-inst go build ./...
```

## Build Modes

### Fast Mode (default) vs Wrap Mode

| Item | Fast Mode (default) | --wrap Mode |
|------|---------------------|-------------|
| **Command** | `whatap-go-inst go build` | `whatap-go-inst go --wrap build` |
| **Prerequisites** | `init` required | None |
| **Speed** | Fast | Slow |
| **Source copy** | No copy | Copies to temp folder |
| **Original changes** | Adds tool.go | No changes |
| **Use case** | Daily development, CI/CD | First test, 100% original preservation |

### init Command

```bash
whatap-go-inst init
```

**Generated files:**
```
whatap_inst.tool.go      # whatap dependency imports (//go:build whatap_tools)
whatap_inst_generate.go  # go:generate directive
```

**Operations:**
1. Analyze project (detect frameworks in use)
2. Generate tool.go (import required whatap packages)

> **Note**: After init, run `go get github.com/whatap/go-api@latest` and `go mod tidy` to add dependencies.

### Fast Mode Operation

```
┌─────────────────────────────────────────────────────────────┐
│  whatap-go-inst go build ./...                              │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│  go build -tags whatap_tools -toolexec="..." ./...          │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│  1. -tags whatap_tools includes tool.go                     │
│  2. Go includes whatap packages in dependencies             │
│  3. toolexec injects instrumentation at compile time        │
└─────────────────────────────────────────────────────────────┘
```

### Wrap Mode Operation (--wrap)

```
┌─────────────────────────────────────────────────────────────┐
│  whatap-go-inst go --wrap build ./...                       │
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

### Error Message When Building Without init

```
$ whatap-go-inst go build ./...

Error: whatap_inst.tool.go not found.

Run init first (in go.mod directory):
  whatap-go-inst init

Or use wrap mode:
  whatap-go-inst go --wrap build ./...
```

### Fast Mode Considerations

1. **init required**
   - `whatap_inst.tool.go` must exist
   - Error message with solution shown if missing

2. **go-api version update**
   - Re-run `init` or execute directly:
     ```bash
     go get github.com/whatap/go-api@latest
     ```

3. **Save transformed source (optional)**
   - By default, whatap-instrumented/ is not created
   - Use `--output` flag to save instrumented source
   - Or use `GO_API_AST_OUTPUT_DIR` environment variable

### --wrap Mode Considerations

1. **Original go.mod is not modified**
   - `go get`, `go mod tidy` only run in temp folder
   - Original source remains unchanged

2. **Downloads @latest every time**
   - `go get github.com/whatap/go-api@latest` runs automatically
   - New go-api versions are automatically applied

3. **whatap-instrumented/ directory created**
   - Can check transformed source code
   - Useful for debugging

## Usage

### Recommended Workflow

```bash
# 1. Initialize (once)
whatap-go-inst init

# 2. Add dependency (once)
go get github.com/whatap/go-api@latest
go mod tidy

# 3. Instrumented build (daily use)
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

# Also save instrumented source files (--output or -O)
whatap-go-inst go --output ./instrumented build ./...
whatap-go-inst go -O ./instrumented build ./...
```

> **Note**: When using `--output` flag, only .go files used in build are saved.
> For full source, use the `inject` command.

### Wrap Mode Build (--wrap)

```bash
# Build without dependency (adds temp dependency)
whatap-go-inst go --wrap build .

# Wrap build all packages
whatap-go-inst go --wrap build ./...

# Specify output file
whatap-go-inst go --wrap build -o myapp .
```

> **Note**: Wrap mode builds in a temp directory so original go.mod is not modified.
> Useful for testing or first-time use.

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

1. **Add dependency (recommended)**
   ```bash
   go get github.com/whatap/go-api@latest
   whatap-go-inst go build ./...
   ```

2. **Use wrap mode (temporary)**
   ```bash
   whatap-go-inst go --wrap build ./...
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

- [toolexec Mode Details](./toolexec.md)
- [go:generate Mode](./go-generate.md)
- [Direct Source Modification Mode](./source-inject.md)
