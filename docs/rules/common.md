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

## Error Tracing

Automatically inserts `trace.Error(context.Background(), err)` into error check patterns.

### Pattern 1: if err != nil (basic)
```go
// Before
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}

// After
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    trace.Error(context.Background(), err)
    return
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
- Init statements supported: `if err := x(); err != nil`, `if _, err := x(); err != nil`
- Nested structures: for, switch, select, nested if all supported
- main function is skipped (to prevent errors before trace.Init)
- context import automatically added (only when error tracing is inserted)

**Benefits**:
- Can track all errors without special handling per library
- Tracks errors that are difficult to wrap like sqlx StructScan, JSON parsing, etc.

---

## Context Preservation Principle (Required Rule)

During AST instrumentation, **context propagation should be maximized** to minimize goroutine ID extraction calls.

### TraceContext Lookup Priority

```
1. Lookup whatap TraceContext from context.Context (preferred)
2. Lookup TraceContext by goroutine ID (fallback)
```

### Why is this important?

- **goroutine ID extraction cost**: Requires runtime.Stack() parsing or unsafe access
- **context lookup cost**: Simple map lookup (much faster)
- Performance difference is significant at points that may be called thousands of times during request processing

### Instrumentation Rules

```go
// GOOD: Pass context
whatapsql.OpenContext(ctx, driverName, dataSourceName)
whataphttp.NewRoundTrip(ctx, transport)

// AVOID: Call without context (goroutine ID fallback used)
whatapsql.Open(driverName, dataSourceName)
```

### Transformer Implementation Checklist

1. Does the original function accept context? → Pass it through
2. Original function doesn't accept context but wrapper can? → Use context version
3. Allow goroutine ID fallback only when context passing is impossible

**Related documentation**: [goroutine ID Acquisition Design](../../dev-docs/goroutine-id-design.md)

---

## Target Version Independence (Required Rule)

AST instrumentation code should be designed to **not be affected by version changes of target libraries**.

### Why is this important?

- Target libraries (Aerospike, Redis, etc.) may change APIs with version updates
- Method signature changes, new methods added, return type changes, etc.
- Hardcoded types/signatures cause compile errors on version updates

### Recommended Approach: sql.Wrap + go/types

For libraries that don't support Hook/Monitor, use the `sql.Wrap` pattern with `go/types`.

**Step 1: Get type information with go/types**
```go
// Perform type checking before AST transformation
import "go/types"

info := &types.Info{
    Types: make(map[ast.Expr]types.TypeAndValue),
}
// Query return type of client.Get() from compiler
returnType := info.Types[callExpr].Type  // → (*aerospike.Record, aerospike.Error)
```

**Step 2: Generate Wrap closure with queried type**
```go
// Original
record, err := client.Get(nil, key)

// After transformation (type automatically inferred)
record, err := sql.Wrap(ctx, "aerospike", "Get", func() (*aerospike.Record, error) {
    return client.Get(nil, key)
})
```

### Benefits

| Item | Hardcoded Approach | go/types Approach |
|------|-------------------|-------------------|
| Version compatibility | Requires modification per version | Automatic adaptation |
| New method support | Manual addition | Automatic support |
| Type accuracy | Typos possible | Compiler guaranteed |
| Build time | Fast | Slightly increased (~50%) |

### Applicable Targets

- Libraries without Hook/Monitor: Aerospike, etc.
- Newly added external service instrumentation

### Non-applicable Targets (Keep existing approach)

- Libraries with Hook/Monitor support: MongoDB (CommandMonitor), go-redis (Hook)
- Middleware-based: Gin, Echo, Fiber, etc.
- Already stabilized packages: database/sql, gorm, etc.

---

## Backward Compatibility (Required Rule)

When making AST instrumentation changes, **backward compatibility should be maintained as much as possible**.

### Compatibility Principles

```
1. Existing go-api-example source must work without changes
2. Existing user code using whatap packages must work without changes
3. When adding new features, default behavior must be the same as before
```

### Required Verification Items

| Item | Verification Content |
|------|---------------------|
| go-api-example | Existing example code builds/runs without modifications |
| whataphttp | Existing API signatures maintained for HttpGet, HttpPost, etc. |
| whatapsql | Existing API signatures maintained for Open, OpenContext, etc. |
| User code | Existing instrumented code works without re-instrumentation |

### Cautions When Making Changes

```go
// GOOD: Add new parameters as optional
func HttpGet(ctx context.Context, url string) (*http.Response, error)  // Keep existing

// BAD: Change existing signature
func HttpGet(ctx context.Context, url string, opts ...Option)  // Breaks compatibility
```

### Test Checklist

- [ ] Full build test of go-api-example
- [ ] Operation test after instrumenting existing testapps
- [ ] inject → remove restoration test (must be identical to original)
