# Custom Instrumentation Guide

Explains how to define custom instrumentation rules using YAML configuration files.

> **Status**: v1.0 implementation complete

---

## Overview

go-api-inst automatically instruments major libraries such as Gin, Echo, and database/sql. Additionally, users can define custom instrumentation rules:

| Rule | Purpose | Example |
|------|---------|---------|
| `inject` | Insert code at function start/end | Logging, metrics |
| `replace` | Replace function calls | sql.Open → whatapsql.Open |
| `hook` | Insert code before/after function calls | Tracing, debugging |
| `add` | Create new files | Helper code |
| `transform` | Template-based transformation | Complex transformations |

---

## Configuration File

### Location (Search Order)

```
1. Path specified with --config flag
2. WHATAP_INST_CONFIG environment variable
3. .whatap/config.yaml    ← Recommended
4. .whatap/whatap.yaml
```

### Basic Structure

```yaml
# .whatap/config.yaml

instrumentation:
  preset: minimal        # minimal, standard, full
  debug: true           # Debug output

custom:
  inject: [...]         # Insert code into function definitions
  replace: [...]        # Replace function calls
  hook: [...]           # Insert code before/after function calls
  add: [...]            # Create new files
  transform: [...]      # Template-based transformation
```

---

## Rule 1: inject

Inserts code at the start/end of function **definitions**.

### Configuration

```yaml
custom:
  inject:
    - package: "main"           # Package name (package declaration in file)
      function: "Process*"      # Function name pattern (wildcards supported)
      start: |                  # Insert at function start
        fmt.Println("function started")
      end: |                    # Insert at function end (note: after return)
        fmt.Println("function ended")
      imports:                  # Required imports
        - "fmt"
```

### Pattern Matching

| Pattern | Description | Match Examples |
|---------|-------------|----------------|
| `*` | All functions | All functions |
| `Handle*` | Prefix match | HandleRequest, HandleError |
| `*Handler` | Suffix match | RequestHandler, ErrorHandler |
| `Get*DB` | Prefix+suffix | GetUserDB, GetOrderDB |
| `ProcessOrder` | Exact match | ProcessOrder only |

### Transformation Result

```go
// Before
func ProcessData(data string) error {
    return process(data)
}

// After
func ProcessData(data string) error {
    fmt.Println("function started")
    return process(data)
    fmt.Println("function ended")  // Warning: unreachable - use defer
}
```

### Recommended: defer Pattern

Use defer in `start` instead of `end`:

```yaml
inject:
  - package: "main"
    function: "Process*"
    start: |
      startTime := time.Now()
      defer func() {
        fmt.Println("elapsed:", time.Since(startTime))
      }()
    imports:
      - "fmt"
      - "time"
```

---

## Rule 2: replace

Replaces function **calls** with a different function.

### Configuration

```yaml
custom:
  replace:
    - package: "database/sql"   # Original package import path
      function: "Open"          # Original function name
      with: "whatapsql.Open"    # Replacement function (package.function)
      imports:                  # Import for replacement function
        - "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"
```

### Transformation Result

```go
// Before
db, err := sql.Open("mysql", dsn)

// After
db, err := whatapsql.Open("mysql", dsn)
```

### Supported Patterns

| Pattern | Example |
|---------|---------|
| Simple assignment | `db, err := sql.Open(...)` |
| if initialization | `if db, err := sql.Open(...); err != nil` |
| Global variable | `globalDB, err = sql.Open(...)` |
| Inside callback | `func() { return sql.Open(...) }` |

---

## Rule 3: hook

Inserts code before/after function **calls**.

### Configuration

```yaml
custom:
  hook:
    - package: "main"           # Target package (package of called function)
      function: "fetchData"     # Function name (wildcards supported)
      before: |                 # Insert before call
        fmt.Println(">>> fetchData starting")
      after: |                  # Insert after call
        fmt.Println("<<< fetchData completed")
      imports:
        - "fmt"
```

### Supported Call Patterns

| Pattern | Code Example |
|---------|--------------|
| Simple call | `fetchData()` |
| Assignment | `data := fetchData()` |
| if initialization | `if err := saveData(); err != nil` |
| switch condition | `switch getData() { ... }` |
| Inside for loop | `for { processData() }` |

### Transformation Result

```go
// Before
data := fetchData()

// After
fmt.Println(">>> fetchData starting")
data := fetchData()
fmt.Println("<<< fetchData completed")
```

---

## Rule 4: add

Creates new files.

### Configuration

```yaml
custom:
  add:
    - package: "main"           # Package name ("main" is root directory)
      file: "whatap_helper.go"  # File name to create
      content: |                # File content
        package main

        func init() {
            println("helper loaded")
        }
```

### Notes

- `package: "main"` → Creates in root directory
- `package: "pkg/util"` → Creates in pkg/util/ directory
- Overwrites existing files

---

## Rule 5: transform

Performs complex transformations using templates.

### Configuration

```yaml
custom:
  transform:
    - package: "database/sql"
      function: "Query"
      template: |
        whatapsql.TraceQuery(ctx, {{.Original}})
```

### Template Variables

| Variable | Description |
|----------|-------------|
| `{{.Original}}` | Original call code |
| `{{.FuncName}}` | Function name |
| `{{.Args}}` | Argument list |

---

## Package Scope

The `package` field matches based on the **file's package declaration**:

```go
package user  // ← Matches this value
```

| package value | Match Target |
|---------------|--------------|
| `"main"` | Files with `package main` |
| `"user"` | Files with `package user` |
| `"order"` | Files with `package order` |

**External libraries are not modified** - only project files are targeted.

---

## Error Handling

### Fail-Fast Policy

**Immediately stops** on error:

```
Error: main.go: apply custom rules: inject rules: 3:6: expected ';', found is
```

| Error Type | When | Behavior |
|------------|------|----------|
| YAML parsing error | Before start | Stop |
| Go code parsing error | During instrumentation | Stop |
| Missing import | During build | `go build` fails |
| Type mismatch | During build | `go build` fails |

### Design Principle

**Build-time error > Runtime error**

- Invalid configuration is caught at instrumentation time
- Additional validation at `go build`
- Runtime errors are user code responsibility

---

## Duplicate Rule Behavior

When multiple rules match the same target, **all are applied**:

```yaml
inject:
  - package: "main"
    function: "ProcessData"    # Rule 1
    start: "fmt.Println(\"1\")"

  - package: "main"
    function: "Process*"       # Rule 2 (wildcard)
    start: "fmt.Println(\"2\")"
```

Result (reverse order insertion):
```go
func ProcessData() {
    fmt.Println("2")  // Rule 2
    fmt.Println("1")  // Rule 1
    // Original code
}
```

---

## Full Example

```yaml
# .whatap/config.yaml

instrumentation:
  preset: minimal
  debug: true

custom:
  # Logging at function start
  inject:
    - package: "main"
      function: "Handle*"
      start: |
        log.Printf("[%s] started", "{{.FuncName}}")
      imports:
        - "log"

  # Replace sql.Open
  replace:
    - package: "database/sql"
      function: "Open"
      with: "whatapsql.Open"
      imports:
        - "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"

  # Trace specific function calls
  hook:
    - package: "main"
      function: "processOrder"
      before: |
        trace.Step(ctx, "ORDER", "processOrder started")
      after: |
        trace.Step(ctx, "ORDER", "processOrder completed")
      imports:
        - "github.com/whatap/go-api/trace"

  # Create helper file
  add:
    - package: "main"
      file: "whatap_init.go"
      content: |
        package main

        import "github.com/whatap/go-api/trace"

        func init() {
            trace.Init(nil)
        }
```

---

## Usage

```bash
# Default (auto-searches .whatap/config.yaml)
whatap-go-inst inject -s . -o ./output

# Specify config file
whatap-go-inst --config custom.yaml inject -s . -o ./output

# Environment variable
WHATAP_INST_CONFIG=custom.yaml whatap-go-inst inject -s . -o ./output

# Verbose output
whatap-go-inst inject -s . -o ./output -v
```

---

## Related Documents

- [config.md](./config.md) - Configuration file details
- [user-guide.md](./user-guide.md) - User guide
