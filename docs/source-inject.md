# Direct Source Modification Mode

Directly analyzes source code and generates transformed results in a separate directory.

## Overview

```bash
# Inject monitoring code
whatap-go-inst inject --src ./myapp --output ./instrumented

# Remove monitoring code
whatap-go-inst remove --src ./instrumented --output ./clean
```

## How It Works

### inject Command

```
┌─────────────────────────────────────────────────────────────────┐
│  whatap-go-inst inject --src ./myapp --output ./instrumented   │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│  Scan source directory                                          │
│                                                                 │
│  ./myapp/                                                       │
│  ├── main.go                                                    │
│  ├── handler/                                                   │
│  │   └── user.go                                                │
│  └── go.mod                                                     │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│  Process each .go file                                          │
│                                                                 │
│  1. Parse AST                                                   │
│  2. Detect frameworks (gin, echo, fiber, etc.)                  │
│  3. Add imports (whatap/go-api)                                 │
│  4. Insert main() initialization code                           │
│  5. Output result file                                          │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│  Create output directory                                        │
│                                                                 │
│  ./instrumented/                                                │
│  ├── main.go           (instrumentation code injected)          │
│  ├── handler/                                                   │
│  │   └── user.go       (copied as-is)                           │
│  └── go.mod            (copied as-is)                           │
└─────────────────────────────────────────────────────────────────┘
```

### remove Command

```
┌─────────────────────────────────────────────────────────────────┐
│  whatap-go-inst remove --src ./instrumented --output ./clean   │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│  Process each .go file                                          │
│                                                                 │
│  1. Parse AST                                                   │
│  2. Remove whatap-related imports                               │
│  3. Remove trace.Init/Shutdown calls                            │
│  4. Output result file                                          │
└─────────────────────────────────────────────────────────────────┘
```

## Usage

### Basic Usage

```bash
# Transform entire directory
whatap-go-inst inject --src ./myapp --output ./instrumented

# Transform single file
whatap-go-inst inject --src ./myapp/main.go --output ./instrumented/main.go
```

### Short Options

```bash
# -s, -o short options
whatap-go-inst inject -s ./myapp -o ./instrumented
whatap-go-inst remove -s ./instrumented -o ./clean
```

### Using Current Directory

```bash
# Use current directory as source
whatap-go-inst inject -o ./instrumented

# Default: src=".", output="./output"
whatap-go-inst inject
```

## Example

### Complete Workflow

**1. Project structure:**
```
myapp/
├── main.go
├── handler/
│   ├── user.go
│   └── product.go
├── go.mod
└── go.sum
```

**2. Run inject:**
```bash
$ whatap-go-inst inject -s ./myapp -o ./instrumented
Source path: ./myapp
Output path: ./instrumented

Injecting: myapp/main.go -> instrumented/main.go
Injecting: myapp/handler/user.go -> instrumented/handler/user.go
Injecting: myapp/handler/product.go -> instrumented/handler/product.go

Done!
```

**3. Check results:**
```bash
$ diff myapp/main.go instrumented/main.go
```

**4. Build:**
```bash
$ cd instrumented
$ go build -o ../myapp-instrumented .
```

### Original (main.go)

```go
package main

import (
    "fmt"
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()

    r.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "hello"})
    })

    fmt.Println("Server starting on :8080")
    r.Run(":8080")
}
```

### After Transformation (instrumented/main.go)

```go
package main

import (
    "fmt"
    "github.com/gin-gonic/gin"
    "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
    "github.com/whatap/go-api/trace"
)

func main() {
    trace.Init(nil)
    defer trace.Shutdown()
    r := gin.Default()
    r.Use(whatapgin.Middleware())  // Middleware auto-injected

    r.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "hello"})
    })

    fmt.Println("Server starting on :8080")
    r.Run(":8080")
}
```

### After Removal (clean/main.go)

```bash
$ whatap-go-inst remove -s ./instrumented -o ./clean
```

Restored to code identical to the original.

## Transformation Rules

### 1. Import Addition

Imports are added based on detected frameworks:

| Framework | Added Import |
|-----------|--------------|
| Common | `github.com/whatap/go-api/trace` |
| Gin | `github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin` |
| Echo v4 | `github.com/whatap/go-api/instrumentation/github.com/labstack/echo/v4/whatapecho` |
| Chi | `github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi` |
| Gorilla | `github.com/whatap/go-api/instrumentation/github.com/gorilla/mux/whatapmux` |
| Fiber v2 | `github.com/whatap/go-api/instrumentation/github.com/gofiber/fiber/v2/whatapfiber` |

Versioned frameworks (Echo v4, Fiber v2, etc.) automatically get the correct import path.

### 2. main() Initialization Code

The following code is inserted in the main() function of the main package:

```go
func main() {
    trace.Init(nil)      // Added
    defer trace.Shutdown() // Added

    // ... existing code
}
```

### 3. Middleware Auto-injection

Middleware is automatically injected by detecting router creation patterns:

```go
// Gin
r := gin.Default()
r.Use(whatapgin.Middleware())  // Auto-injected

// Echo
e := echo.New()
e.Use(whatapecho.Middleware())  // Auto-injected

// Fiber
app := fiber.New()
app.Use(whatapfiber.Middleware())  // Auto-injected

// Chi
r := chi.NewRouter()
r.Use(whatapchi.Middleware)  // Auto-injected (function value)
```

### 4. Comment Preservation

Using the `dave/dst` library, comments from the original code are perfectly preserved:

```go
// Original
func main() {
    // Initialize server
    r := gin.Default()
}

// After transformation - comments preserved
func main() {
    trace.Init(nil)
    defer trace.Shutdown()
    // Initialize server
    r := gin.Default()
    r.Use(whatapgin.Middleware())
}
```

After `inject`→`remove`, code is restored 100% identical to the original.

### 5. Skipped Files

- `_test.go` files
- Files that already import `whatap/go-api`
- Files with parsing errors

### 6. Non-Go Files

Files other than `.go` are copied as-is:
- `go.mod`, `go.sum`
- Configuration files (`.yaml`, `.json`, `.toml`)
- Other resource files

## Advantages

| Advantage | Description |
|-----------|-------------|
| **Preserves original** | Original source is not modified |
| **Easy comparison** | Can check changes with diff |
| **Selective build** | Choose to build original or instrumented version |
| **Debugging** | Can directly inspect the actual running code |

## Disadvantages

| Disadvantage | Description |
|--------------|-------------|
| **Disk usage** | Creates copy of source |
| **Sync required** | Need to re-run when original changes |
| **Path management** | Need to manage output directory |

## CI/CD Integration

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

      - name: Inject instrumentation
        run: whatap-go-inst inject -s . -o ./instrumented

      - name: Build instrumented version
        working-directory: ./instrumented
        run: |
          go mod tidy
          go build -o ../myapp .

      - name: Upload artifact
        uses: actions/upload-artifact@v3
        with:
          name: myapp-instrumented
          path: myapp
```

### Makefile

```makefile
SRC_DIR := .
INSTRUMENTED_DIR := ./instrumented
CLEAN_DIR := ./clean
OUTPUT := ./bin/myapp

.PHONY: all inject remove build build-instrumented clean

# Default build (no instrumentation)
build:
	go build -o $(OUTPUT) $(SRC_DIR)

# Inject instrumentation code
inject:
	whatap-go-inst inject -s $(SRC_DIR) -o $(INSTRUMENTED_DIR)

# Build instrumented version
build-instrumented: inject
	cd $(INSTRUMENTED_DIR) && go mod tidy && go build -o ../$(OUTPUT)-instrumented .

# Remove instrumentation code (restore)
remove:
	whatap-go-inst remove -s $(INSTRUMENTED_DIR) -o $(CLEAN_DIR)

# Clean up
clean:
	rm -rf $(INSTRUMENTED_DIR) $(CLEAN_DIR) $(OUTPUT)*
```

## Parallel Build Scenario

```bash
# Build original and instrumented versions simultaneously
whatap-go-inst inject -s ./src -o ./instrumented &
go build -o ./bin/myapp ./src &
wait

cd ./instrumented && go build -o ../bin/myapp-instrumented .
```

## Important Notes

### 1. go.mod Handling

When building in the output directory, you may need to add dependencies:

```bash
cd ./instrumented
go get github.com/whatap/go-api@latest
go mod tidy
go build ./...
```

### 2. Replace Directives

If the original `go.mod` has `replace` directives, relative paths may need adjustment.

### 3. Large Projects

Processing time may be longer for projects with many files:

```bash
# Check progress
whatap-go-inst inject -s ./large-project -o ./instrumented 2>&1 | tee inject.log
```

## Next Steps

- [Build Wrapper Mode](./build-wrapper.md) - Simpler approach
- [Multi-Module Projects](./multi-module.md)
