# Logging Library Transformation Rules

Transformation rules for logging libraries. Connects log output to WhaTap logsink for transaction correlation.

## Standard log Package

```go
import (
    "log"
    "github.com/whatap/go-api/logsink"
    "os"
)

func main() {
    trace.Init(nil)
    defer trace.Shutdown()
    log.SetOutput(logsink.GetTraceLogWriter(os.Stderr))  // Inserted
    // ...
}
```

| Original | After Transformation | Description |
|----------|---------------------|-------------|
| (none) | `log.SetOutput(logsink.GetTraceLogWriter(os.Stderr))` | Inserted after trace.Init |

### log.New() Instance Wrapping

When `log.New()` is used to create a custom logger instance, the writer argument is automatically wrapped with `logsink.GetTraceLogWriter()`:

```go
// Before
logger := log.New(os.Stderr, "prefix: ", log.LstdFlags)

// After
logger := log.New(logsink.GetTraceLogWriter(os.Stderr), "prefix: ", log.LstdFlags)
```

This transformation applies in all contexts: struct fields, return statements, function arguments, and variable assignments.

| Original | After Transformation | Description |
|----------|---------------------|-------------|
| `log.New(w, prefix, flag)` | `log.New(logsink.GetTraceLogWriter(w), prefix, flag)` | Writer argument wrapped |

---

## logrus

```go
import (
    log "github.com/sirupsen/logrus"
    "github.com/whatap/go-api/logsink"
    "os"
)

func main() {
    trace.Init(nil)
    defer trace.Shutdown()
    log.SetOutput(logsink.GetTraceLogWriter(os.Stderr))  // Inserted
    // ...
}
```

| Original | After Transformation | Description |
|----------|---------------------|-------------|
| (none) | `log.SetOutput(logsink.GetTraceLogWriter(os.Stderr))` | Auto-recognizes alias |

> **Note**: Automatically recognized even when logrus is aliased as `log`.

### logrus.New() Instance Wrapping

When `logrus.New()` is used to create a custom logger instance, it is automatically wrapped with `WrapLogger`:

```go
// Before
logger := logrus.New()

// After
logger := whataplogrus.WrapLogger(logrus.New())
```

**Signature**: `whataplogrus.WrapLogger(*logrus.Logger) *logrus.Logger`

This transformation applies in all contexts: struct fields, return statements, function arguments, and variable assignments.

---

## zap (uber-go/zap)

zap outputs directly to os.Stderr, so HookStderr() is used.

```go
import (
    "go.uber.org/zap"
    "github.com/whatap/go-api/logsink"
)

func main() {
    trace.Init(nil)
    defer trace.Shutdown()
    logsink.HookStderr()  // Inserted

    logger, _ := zap.NewProduction()
    // ...
}
```

| Original | After Transformation | Description |
|----------|---------------------|-------------|
| (none) | `logsink.HookStderr()` | os.Stderr redirection |

> **Note**: zap uses pipe-based HookStderr(). Future improvement planned with zapcore.WriteSyncer integration.

---

## Supported Versions Summary

| Library | Supported Versions | Import Path | Notes |
|---------|-------------------|-------------|-------|
| Standard log | Go standard | `log` | SetOutput insertion |
| logrus | All versions | `github.com/sirupsen/logrus` | SetOutput insertion, alias support |
| zap | All versions | `go.uber.org/zap` | HookStderr used |
| **fmt** | Go standard | `fmt` | **whatapfmt transformation** |

---

## fmt Package (whatapfmt)

Transforms fmt.Print family functions to whatapfmt to correlate stdout output with transactions.

```go
import (
    "fmt"
    "github.com/whatap/go-api/instrumentation/fmt/whatapfmt"
)

func main() {
    trace.Init(nil)
    defer trace.Shutdown()
    whatapfmt.Println("Server starting...")  // fmt.Println → whatapfmt.Println transformed
}
```

| Original | After Transformation | Description |
|----------|---------------------|-------------|
| `fmt.Print(...)` | `whatapfmt.Print(...)` | stdout output + txid correlation |
| `fmt.Printf(...)` | `whatapfmt.Printf(...)` | stdout output + txid correlation |
| `fmt.Println(...)` | `whatapfmt.Println(...)` | stdout output + txid correlation |
| `fmt.Sprintf(...)` | (not transformed) | Returns string, not output |
| `fmt.Sprint(...)` | (not transformed) | Returns string, not output |

> **Note**: fmt.Sprint* functions are not transformed as they only return strings without output.

### Configuration

```ini
# whatap.conf
logsink_enabled=true
logsink_fmt_enabled=true   # Enable whatapfmt
```

Category is shared with `AppStdOut` (same as ProxyStream stdout).
