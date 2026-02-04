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

### Skip Rules

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
| replace local modules | Local path (../) | No (use separate inject) |

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

### Approach 1: Separate inject for Each Module (Recommended)

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

### Approach 2: Monorepo Structure (New Projects)

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
# Build
whatap-go-inst go build ./cmd/main
```

---

## Important Notes

### 1. replace Module Handling

With build wrapper mode, replace target modules reference original (non-instrumented) code.

```go
// go.mod with replace
replace mycompany/db-lib => ../db-lib

// After temp directory copy:
replace mycompany/db-lib => /original/path/to/db-lib  // References original!
```

**Solution:** Use Approach 1 (separate inject for each module)

### 2. replace Modules Not Instrumented

Build wrapper mode does not instrument replace modules. The replace target references original (non-instrumented) code.

**Recommendation:** If you need to instrument replace modules, use Approach 1 (separate inject for each module)

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

### Q1: I want to instrument replace modules

**A:** Build wrapper mode does not instrument replace modules. Use separate inject for each module:

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
- Existing multi-module → Separate inject for each module

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

      # 2. Inject each module
      - run: |
          whatap-go-inst inject -s db-lib -o db-lib-inst
          whatap-go-inst inject -s web-lib -o web-lib-inst
          whatap-go-inst inject -s main-app -o main-app-inst

      # 3. Update replace paths and build
      - run: |
          cd main-app-inst
          # Update go.mod replace paths to instrumented versions
          go mod edit -replace mycompany/db-lib=../db-lib-inst
          go mod edit -replace mycompany/web-lib=../web-lib-inst
          go get github.com/whatap/go-api@latest
          go mod tidy
          go build -o myapp ./...
```

---

## Related Documents

- [Build Wrapper Mode](./build-wrapper.md)
- [User Guide](./user-guide.md)
- [Config Settings](./config.md)
