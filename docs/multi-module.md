# Multi-Module Project Instrumentation Guide

Explains how to use whatap-go-inst with projects composed of multiple Go modules.

## Table of Contents

- [Terminology](#terminology)
- [Package Type Handling](#package-type-handling)
- [Multi-Module Scenarios](#multi-module-scenarios)
- [Recommended Approaches](#recommended-approaches)
- [Mode Behavior Comparison](#mode-behavior-comparison)
- [Important Notes](#important-notes)
- [FAQ](#faq)

---

## Terminology

| Term | Description | Example |
|------|-------------|---------|
| **Module** | Unit with go.mod | `module mycompany/user-api` |
| **Package** | Directory-level code grouping | `mycompany/user-api/pkg/auth` |
| **External module** | Dependencies from go get | `github.com/gin-gonic/gin` |
| **Local module** | Separate module in same project | `replace ../shared-lib` |

---

## Package Type Handling

whatap-go-inst handles packages differently based on their location.

### toolexec Skip Rules

```go
// 1. Go standard library → Skip
if strings.HasPrefix(path, os.Getenv("GOROOT")) {
    return true  // No transformation
}

// 2. External packages (go get) → Skip
if strings.HasPrefix(path, os.Getenv("GOMODCACHE")) {
    return true  // No transformation
}

// 3. Everything else (my code) → Transform
return false
```

### Summary

| Package Type | Path | Instrumented |
|--------------|------|--------------|
| Go standard library | `$GOROOT/src/...` | No (skip) |
| External modules (go get) | `$GOMODCACHE/...` | No (skip) |
| My project code | Local path | Yes |
| replace local modules | Local path (../) | Yes (fast mode only) |

---

## Multi-Module Scenarios

### Scenario: 3 Separate Modules

```
C:/projects/
├── db-lib/              # Module A: DB logic library
│   ├── go.mod           # module mycompany/db-lib
│   ├── connection.go    # Uses sql.Open()
│   └── query.go
│
├── web-lib/             # Module B: Web server library
│   ├── go.mod           # module mycompany/web-lib
│   ├── server.go        # Uses gin.Default()
│   └── handler.go
│
└── main-app/            # Module C: Main app
    ├── go.mod           # module mycompany/main-app
    └── main.go          # Imports A, B
```

### Module C's go.mod

```go
module mycompany/main-app

go 1.21

require (
    mycompany/db-lib v1.0.0
    mycompany/web-lib v1.0.0
)
```

---

## Recommended Approaches

### Approach 1: go.mod replace + Fast Mode (Development)

The simplest method for local development.

**Step 1: Add replace to go.mod**

```go
// main-app/go.mod
module mycompany/main-app

require (
    mycompany/db-lib v1.0.0
    mycompany/web-lib v1.0.0
)

// Replace with local paths
replace mycompany/db-lib => ../db-lib
replace mycompany/web-lib => ../web-lib
```

**Step 2: Initialize and add whatap dependency (all modules)**

```bash
# Initialize in main-app
cd main-app && whatap-go-inst init

# Add dependency in each module
cd db-lib && go get github.com/whatap/go-api@latest
cd web-lib && go get github.com/whatap/go-api@latest
cd main-app && go get github.com/whatap/go-api@latest
```

**Step 3: Build**

```bash
cd main-app
whatap-go-inst go build ./...
```

**Result:**
- main-app instrumented
- db-lib instrumented (local reference via replace)
- web-lib instrumented (local reference via replace)

---

### Approach 2: Separate inject for Each Module (Deployment)

Recommended method for production deployment.

**Step 1: Inject each module**

```bash
# Instrument db-lib
cd db-lib
whatap-go-inst inject -s . -o ../db-lib-instrumented

# Instrument web-lib
cd web-lib
whatap-go-inst inject -s . -o ../web-lib-instrumented

# Instrument main-app
cd main-app
whatap-go-inst inject -s . -o ../main-app-instrumented
```

**Step 2: Replace with instrumented versions**

```go
// main-app-instrumented/go.mod
replace mycompany/db-lib => ../db-lib-instrumented
replace mycompany/web-lib => ../web-lib-instrumented
```

**Step 3: Build**

```bash
cd main-app-instrumented
go build ./...
```

---

### Approach 3: Monorepo Structure (New Projects)

For new projects, a single module structure is simplest.

```
myproject/
├── go.mod              # Single module mycompany/myproject
├── cmd/
│   └── main/
│       └── main.go     # main package
├── internal/
│   ├── db/
│   │   └── connection.go
│   └── web/
│       └── server.go
└── pkg/
    └── shared/
        └── utils.go
```

```bash
# Initialize (once)
whatap-go-inst init

# Add dependencies (required!)
go get github.com/whatap/go-api@latest
go mod tidy

# Build
whatap-go-inst go build ./cmd/main
```

---

## Mode Behavior Comparison

### --wrap Mode vs Fast Mode

| Item | --wrap Mode | Fast Mode |
|------|-------------|-----------|
| **main module** | Instrumented | Instrumented |
| **replace modules** | Not instrumented | Instrumented |
| **External modules (GOMODCACHE)** | Skip | Skip |
| **Prerequisites** | None | init required + whatap/go-api in go.mod |

### --wrap Mode replace Handling

```go
// --wrap mode only adjusts replace paths
replace mycompany/db-lib => ../db-lib

// After temp directory copy:
replace mycompany/db-lib => /original/path/to/db-lib  // References original!
```

**Result:** replace target modules are not copied, reference original (non-instrumented) code

### Fast Mode replace Handling

```
Go compiler → Requests db-lib/connection.go compilation
           ↓
toolexec → Check path: ../db-lib/connection.go
         → GOROOT? No
         → GOMODCACHE? No
         → Local path → Instrument!
```

**Result:** replace target modules are instrumented at compile time

---

## Important Notes

### 1. Fast Mode Requirements

For replace module instrumentation, **all modules** need whatap dependency.

```bash
# Run init in main module
whatap-go-inst init

# Add dependency in each module
go get github.com/whatap/go-api@latest
```

### 2. --output Flag Limitations

With `--output` flag in fast mode, replace modules are not saved properly.

| Module | Save Path | Issue |
|--------|-----------|-------|
| main module | `output/pkg/service.go` | Path structure preserved |
| replace module | `output/connection.go` | Flat (path lost) |

```
# Expected output
output/
├── main.go              # main module (OK)
├── pkg/
│   └── handler.go       # main module (OK)
├── connection.go        # db-lib (path lost!)
└── server.go            # web-lib (path lost!)
```

**Recommendation:** If you need to inspect instrumented source, use Approach 2 (separate inject)

### 3. Injecting Libraries Without main

Libraries without main function can be injected.

| Item | Handling |
|------|----------|
| `trace.Init()` | Not added (correct) |
| `sql.Open()` → `whatapsql.Open()` | Transformed |
| `gin.Default()` + middleware | Added |

```bash
# Inject library module
cd db-lib
whatap-go-inst inject -s . -o ./instrumented

# Result: Transformed to whatapsql.Open()
# trace.Init() not added (should only be called in main)
```

### 4. Dependency Propagation

Modules using instrumented libraries also need whatap dependency.

```go
// db-lib-instrumented/connection.go
import "github.com/whatap/go-api/instrumentation/.../whatapsql"

// main-app needs this dependency to build
```

```bash
cd main-app
go get github.com/whatap/go-api@latest
```

---

## FAQ

### Q1: I want to instrument replace modules in --wrap mode

**A:** --wrap mode does not instrument replace modules. Two options:

1. **Use fast mode** (recommended)
   ```bash
   whatap-go-inst init
   go get github.com/whatap/go-api@latest && go mod tidy
   whatap-go-inst go build ./...
   ```

2. **Separate inject for each module**
   ```bash
   whatap-go-inst inject -s ../db-lib -o ../db-lib-inst
   whatap-go-inst inject -s ../web-lib -o ../web-lib-inst
   ```

### Q2: Can I instrument external libraries (gin, gorm, etc.)?

**A:** No. External libraries are in `$GOMODCACHE` and are skipped. This is intentional.

- Modifying external libraries affects other projects
- Go module system's immutability principle

Instead, **calls to external libraries in user code** are transformed:
- `sql.Open()` → `whatapsql.Open()`
- `gin.Default()` → Middleware auto-added

### Q3: Monorepo vs multi-module, which is better?

| Structure | Pros | Cons |
|-----------|------|------|
| **Monorepo** | Simple instrumentation, easy dependency management | Hard to reuse modules |
| **Multi-module** | Independent module deployment | Complex instrumentation setup |

**Recommendation:**
- New projects → Monorepo
- Existing multi-module → replace + fast mode

### Q4: How do I build multi-module in CI/CD?

```yaml
# GitHub Actions example
jobs:
  build:
    steps:
      # 1. Checkout all modules
      - uses: actions/checkout@v3
        with:
          path: main-app
      - uses: actions/checkout@v3
        with:
          repository: mycompany/db-lib
          path: db-lib
      - uses: actions/checkout@v3
        with:
          repository: mycompany/web-lib
          path: web-lib

      # 2. Initialize and add whatap dependency
      - run: |
          cd main-app && whatap-go-inst init
          cd db-lib && go get github.com/whatap/go-api@latest
          cd web-lib && go get github.com/whatap/go-api@latest
          cd main-app && go get github.com/whatap/go-api@latest

      # 3. Build
      - run: |
          cd main-app
          whatap-go-inst go build -o myapp ./...
```

---

## Related Documents

- [Build Wrapper Mode](./build-wrapper.md)
- [User Guide](./user-guide.md)
- [Config Settings](./config.md)
