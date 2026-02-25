# Common Transformation Rules

Transformations that apply to all packages.

## Import Addition

```go
import (
    "github.com/whatap/go-api/trace"
)
```

## main() Function Initialization

```go
func main() {
    trace.Init(nil)           // Inserted
    defer trace.Shutdown()    // Inserted
    // ... existing code
}
```

## Transformation Scope

Transformations are applied to **all code blocks**, not just top-level functions.

### Supported Code Locations

| Location | Example | Supported |
|----------|---------|:---------:|
| Top-level functions | `func main() { r := gin.New() }` | Yes |
| Anonymous functions | `var fn = func() { r := gin.New() }` | Yes |
| Struct field functions | `cmd.Run = func() { r := gin.New() }` | Yes |
| Closure arguments | `http.HandleFunc("/", func(w, r) { ... })` | Yes |
| Nested blocks | `if cond { r := gin.New() }` | Yes |

### Common Pattern: Cobra Command

Many Go applications use Cobra for CLI, with initialization in anonymous functions:

```go
// Before
var serverCmd = &cobra.Command{
    Use: "server",
    Run: func(cmd *cobra.Command, args []string) {
        r := gin.New()          // Detected and transformed
        r.Run(":8080")
    },
}

// After
var serverCmd = &cobra.Command{
    Use: "server",
    Run: func(cmd *cobra.Command, args []string) {
        r := gin.New()
        r.Use(whatapgin.Middleware())  // Inserted
        r.Run(":8080")
    },
}
```

---

## Error Tracing

Automatically inserts `trace.Error(context.Background(), err)` into error check patterns.
Enabled with `--error-tracking` flag.

### Pattern 1: if err != nil (basic)
```go
// Before
if err != nil {
    return err
}

// After
if err != nil {
    trace.Error(context.Background(), err)
    return err
}
```

### Pattern 2: After if err == nil { return }
```go
// Before
if err == nil {
    return result
}
handleError(err)

// After
if err == nil {
    return result
}
trace.Error(context.Background(), err)
handleError(err)
```

### Pattern 3: if err == nil { } else { return }
```go
// Before
if err == nil {
    doSomething()
} else {
    return err
}

// After
if err == nil {
    doSomething()
} else {
    trace.Error(context.Background(), err)
    return err
}
```

**Scope**:
- Error variable names: `err`, `e`, `error`
- Init statements supported: `if err := x(); err != nil`
- Nested structures: for, switch, select, nested if all supported
- main function is skipped (to prevent errors before trace.Init)
- context import automatically added when needed

---

## Context Preservation Principle

During AST instrumentation, **context propagation should be maximized** to minimize goroutine ID extraction calls.

### TraceContext Lookup Priority

```
1. Lookup whatap TraceContext from context.Context (preferred)
2. Lookup TraceContext by goroutine ID (fallback)
```

Context lookup is a simple map lookup (fast), while goroutine ID extraction requires runtime.Stack() parsing (expensive). Performance difference is significant at high-frequency call points.

```go
// GOOD: Pass context
whatapsql.OpenContext(ctx, driverName, dataSourceName)

// AVOID: Call without context (goroutine ID fallback used)
whatapsql.Open(driverName, dataSourceName)
```
