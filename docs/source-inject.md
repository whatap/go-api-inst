# Direct Source Modification Mode

Directly analyzes source code and generates transformed results in a separate directory.

## Overview

```bash
# Inject monitoring code
whatap-go-inst inject --src ./myapp --output ./instrumented

# Remove monitoring code
whatap-go-inst remove --src ./instrumented --output ./clean
```

---

## Usage

### Basic Usage

```bash
# Transform entire directory
whatap-go-inst inject --src ./myapp --output ./instrumented

# Transform single file
whatap-go-inst inject --src ./myapp/main.go --output ./instrumented/main.go

# Short options
whatap-go-inst inject -s ./myapp -o ./instrumented

# Default: src=".", output="./output"
whatap-go-inst inject
```

### Remove

```bash
whatap-go-inst remove -s ./instrumented -o ./clean

# Compare with original (should have no differences)
diff -r ./original ./clean
```

---

## Example

### Complete Workflow

**1. Run inject:**
```bash
whatap-go-inst inject -s ./myapp -o ./instrumented
```

**2. Check results:**
```bash
diff myapp/main.go instrumented/main.go
```

**3. Build:**
```bash
cd instrumented
go get github.com/whatap/go-api@latest
go mod tidy
go build -o ../myapp .
```

### Before / After

**Original (main.go):**
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

**After Transformation (instrumented/main.go):**
```go
package main

import (
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
    r.Run(":8080")
}
```

After `inject` → `remove`, code is restored identical to the original.

---

## External Module Instrumentation (v0.5.4+)

```bash
# Specify external module
whatap-go-inst inject -s . -o ./instrumented --external-module=mycompany/db-lib

# Wildcard patterns
whatap-go-inst inject -s . -o ./instrumented --external-module="mycompany.com/internal/*"
```

Output structure:
```
./instrumented/
├── main.go                  (instrumented)
├── go.mod                   (replace directive added)
└── _modules/
    └── mycompany/
        └── db-lib/
            ├── connection.go  (instrumented)
            └── go.mod         (whatap/go-api require added)
```

For details, see [Multi-Module Projects](./multi-module.md).

---

## Notes

- **go.mod handling**: After inject, run `go get github.com/whatap/go-api@latest` + `go mod tidy` in the output directory.
- **Replace directives**: If original go.mod has replace with relative paths, paths may need adjustment.
- **Skipped files**: `_test.go`, files already importing `whatap/go-api`, files with parsing errors.
- **Non-Go files**: `go.mod`, `go.sum`, config files, etc. are copied as-is.
- **Comment preservation**: Using the `dave/dst` library, comments from the original code are preserved.

---

## Next Steps

- [Build Wrapper Mode](./build-wrapper.md) — Simpler approach (recommended)
- [Multi-Module Projects](./multi-module.md)
