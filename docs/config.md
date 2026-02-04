# Configuration Guide

Detailed guide for whatap-go-inst configuration files.

## Table of Contents

- [Config File Location](#config-file-location)
- [Config File Format](#config-file-format)
- [Preset Options](#preset-options)
- [Package List](#package-list)
- [Custom Settings (User-Defined Instrumentation)](#custom-settings-user-defined-instrumentation)
- [Exclude Patterns](#exclude-patterns)
- [Copy Exclude Directories](#copy-exclude-directories)
- [Priority](#priority)
- [Usage Examples](#usage-examples)
- [Environment Variables](#environment-variables)

---

## Config File Location

Config files are searched in the following order:

| Order | Location | Description |
|-------|----------|-------------|
| 1 | `--config` flag | Explicitly specified via CLI |
| 2 | `WHATAP_INST_CONFIG` env var | Specified via environment variable |
| 3 | `.whatap/config.yaml` | **Default recommended location** |
| 4 | `.whatap/whatap.yaml` | Alternative config file |

> **Recommended**: Create your config file at `.whatap/config.yaml`.

---

## Config File Format

### Full Structure

```yaml
# .whatap/config.yaml

instrumentation:
  # Basic options
  error_tracking: false    # Inject error tracking code (--error-tracking)
  debug: false             # Enable debug output (GO_API_AST_DEBUG)
  output_dir: ""           # Instrumented source output directory

  # Package selection
  preset: "full"           # Instrumentation preset (full/minimal/web/database/external/log/custom)
  enabled_packages: []     # Additional packages to enable
  disabled_packages: []    # Packages to disable

# User-defined instrumentation
custom:
  inject: []      # Insert code inside function definitions
  replace: []     # Replace function calls
  hook: []        # Insert before/after function calls
  transform: []   # Code pattern -> template transformation

# File patterns to exclude (optional - defaults auto-applied if omitted)
# exclude:
#   - "*_test.go"
#   - "vendor/**"
```

### instrumentation Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `error_tracking` | bool | `false` | Auto-insert `trace.Error()` in `if err != nil` patterns |
| `debug` | bool | `false` | Debug log output (same as `GO_API_AST_DEBUG=1`) |
| `output_dir` | string | `""` | Instrumented source output directory |
| `preset` | string | `"full"` | Instrumentation preset selection |
| `enabled_packages` | []string | `[]` | Additional packages to enable |
| `disabled_packages` | []string | `[]` | Packages to disable |

---

## Preset Options

Select package groups to instrument.

| Preset | Description | Included Packages |
|--------|-------------|-------------------|
| `full` | Enable all packages **(default)** | Web + DB + External + Log |
| `minimal` | Only add trace.Init/Shutdown | None (main init only) |
| `web` | Web frameworks only | gin, echo, fiber, chi, gorilla, nethttp, fasthttp |
| `database` | Databases only | sql, sqlx, gorm, jinzhugorm |
| `external` | External services only | redigo, goredis, mongo, aerospike, sarama, grpc, k8s |
| `log` | Logging libraries only | fmt, log, logrus, zap |
| `custom` | User-defined | Specify with enabled_packages |

### Preset Behavior

```
Final enabled packages = Preset packages + enabled_packages - disabled_packages
```

- `preset: full` + `disabled_packages: ["grpc"]` -> All except gRPC
- `preset: web` + `enabled_packages: ["sql"]` -> Web + SQL
- `preset: custom` + `enabled_packages: ["gin", "sql"]` -> Only Gin and SQL

---

## Package List

### Web Frameworks (`web`)

| Package Name | Library | Injected Code |
|--------------|---------|---------------|
| `gin` | github.com/gin-gonic/gin | `whatapgin.Middleware()` |
| `echo` | github.com/labstack/echo/v4 | `whatapecho.Middleware()` |
| `fiber` | github.com/gofiber/fiber/v2 | `whatapfiber.Middleware()` |
| `chi` | github.com/go-chi/chi/v5 | `whatapchi.Middleware` |
| `gorilla` | github.com/gorilla/mux | `whatapmux.Middleware` |
| `nethttp` | net/http | `whataphttp.Func()`, `whataphttp.Handler()` |
| `fasthttp` | github.com/valyala/fasthttp | `whatapfasthttp.Middleware()` |

### Database (`database`)

| Package Name | Library | Injected Code |
|--------------|---------|---------------|
| `sql` | database/sql | `whatapsql.Open()` |
| `sqlx` | github.com/jmoiron/sqlx | `whatapsqlx.Open()` |
| `gorm` | gorm.io/gorm | `whatapgorm.Open()` |
| `jinzhugorm` | github.com/jinzhu/gorm | `whatapgorm.Open()` |

### External Services (`external`)

| Package Name | Library | Injected Code |
|--------------|---------|---------------|
| `redigo` | github.com/gomodule/redigo | `whatapredigo.Dial()` |
| `goredis` | github.com/redis/go-redis/v9 | `whatapgoredis.NewClient()` |
| `mongo` | go.mongodb.org/mongo-driver | `whatapmongo.Connect()` |
| `aerospike` | github.com/aerospike/aerospike-client-go | `whatapsql.Wrap()` |
| `sarama` | github.com/IBM/sarama | Interceptor injection |
| `grpc` | google.golang.org/grpc | Server/Client Interceptor |
| `k8s` | k8s.io/client-go | `config.Wrap()` |

### Logging Libraries (`log`)

| Package Name | Library | Injected Code |
|--------------|---------|---------------|
| `fmt` | fmt | `whatapfmt.Print/Printf/Println()` |
| `log` | log | `log.SetOutput(logsink.GetTraceLogWriter())` |
| `logrus` | github.com/sirupsen/logrus | `logrus.SetOutput(logsink.GetTraceLogWriter())` |
| `zap` | go.uber.org/zap | `logsink.HookStderr()` |

---

## Custom Settings (User-Defined Instrumentation)

> **Status**: v1.0 implementation complete. See [custom-instrumentation.md](./custom-instrumentation.md) for detailed guide.

Configure 5 types of user-defined instrumentation rules in the `custom` section.

**Execution order**: add -> inject -> replace -> hook -> transform (sequential)

| Setting | Purpose | Target |
|---------|---------|--------|
| `add` | **Create** new files/functions | Package |
| `inject` | Insert code inside function **definitions** | Function definition |
| `replace` | Replace function **calls** | Function call |
| `hook` | Insert before/after function **calls** | Function call |
| `transform` | Code pattern -> template transformation (flexible) | All code |

### add (Create New Files/Functions)

Create helper functions or wrapper files. Can be used by replace afterwards.

**Configuration:**
```yaml
custom:
  add:
    # Method 1: Add function with inline code
    - package: "myapp/helper"             # Target package
      file: "whatap_helper.go"            # File to create
      content: |
        package helper

        import "github.com/whatap/go-api/trace"

        func WrapQuery(ctx context.Context, query string) string {
            ctx = trace.StartMethod(ctx, "WrapQuery")
            defer trace.EndMethod(ctx, nil)
            return query
        }

    # Method 2: Use template file
    - package: "myapp/db"
      file: "whatap_db.go"
      content_file: "templates/db-helper.go.tmpl"

    # Method 3: Add function to existing file
    - package: "myapp/service"
      file: "service.go"                   # Existing file
      append: true                          # Append to end of file
      content: |
        func traceMethod(ctx context.Context, name string) (context.Context, func()) {
            return trace.StartMethod(ctx, name), func() { trace.EndMethod(ctx, nil) }
        }
```

**Transformation result:**
```go
// Generated file: myapp/helper/whatap_helper.go
package helper

import "github.com/whatap/go-api/trace"

func WrapQuery(ctx context.Context, query string) string {
    ctx = trace.StartMethod(ctx, "WrapQuery")
    defer trace.EndMethod(ctx, nil)
    return query
}
```

**Usage example (add + replace combination):**
```yaml
custom:
  # Step 1: Create helper function
  add:
    - package: "myapp/db"
      file: "whatap_wrapper.go"
      content: |
        package db

        func TracedQuery(ctx context.Context, sql string) (*Result, error) {
            ctx = trace.StartMethod(ctx, "db.query")
            defer trace.EndMethod(ctx, nil)
            return OriginalQuery(sql)
        }

  # Step 2: Replace original function call with helper
  replace:
    - package: "myapp/db"
      function: "Query"
      with: "TracedQuery"
```

### inject (Insert Inside Function Definition)

Insert code at start/end of user function bodies.

> **Note**: Only user-defined functions (in current module) can be targeted. Go standard library or external package functions cannot be targeted.
> **Recommendation**: For functions with mid-function returns, use defer pattern in `start` instead of `end`.

**Configuration:**
```yaml
custom:
  inject:
    - package: "myapp/service"       # Go import path
      function: "*"                   # Function name (* = all)
      start: |
        ctx = trace.StartMethod(ctx)
        defer trace.EndMethod(ctx, err)
      imports:
        - "github.com/whatap/go-api/trace"
```

**Transformation result:**
```go
// Original
func ProcessOrder(ctx context.Context) error {
    // Business logic
}

// After transformation
func ProcessOrder(ctx context.Context) error {
    ctx = trace.StartMethod(ctx)           // <- start inserted
    defer trace.EndMethod(ctx, err)        // <- start inserted
    // Business logic
}
```

### replace (Replace Function Calls)

Replace function calls with different functions.

**Configuration:**
```yaml
custom:
  replace:
    - package: "database/sql"
      function: "Open"
      with: "whatapsql.Open"
      imports:
        - "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"
```

**Transformation result:**
```go
// Original
db, err := sql.Open("mysql", dsn)

// After transformation
db, err := whatapsql.Open("mysql", dsn)
```

### hook (Insert Before/After Function Calls)

Insert code before and after function calls.

> **Note**: You can use just `before` or just `after`.

**Configuration:**
```yaml
custom:
  hook:
    - package: "mycompany/mydb"
      function: "Query"
      before: "ctx, span := trace.Start(ctx, \"db.query\")"
      after: "span.End()"
      imports:
        - "github.com/whatap/go-api/trace"
```

**Transformation result:**
```go
// Original
result, err := mydb.Query(sql)

// After transformation
ctx, span := trace.Start(ctx, "db.query")  // <- before inserted
result, err := mydb.Query(sql)
span.End()                                  // <- after inserted
```

### transform (Code Pattern -> Template Transformation)

Transform complex code patterns using templates. The most flexible approach.

**Example 1: Add middleware**

```yaml
custom:
  transform:
    - package: "github.com/gin-gonic/gin"
      function: "Default"
      template: |
        {{.Original}}
        {{.Var}}.Use(whatapgin.Middleware())
      imports:
        - "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
```

**Transformation result:**
```go
// Original
r := gin.Default()

// After transformation
r := gin.Default()
r.Use(whatapgin.Middleware())
```

**Example 2: Aerospike Wrap (wrap with closure)**

```yaml
custom:
  transform:
    - package: "github.com/aerospike/aerospike-client-go"
      function: "NewClient"
      template: |
        func() (*as.Client, error) {
            ctx, done := whatapsql.Start(context.Background(), "aerospike")
            defer done()
            return {{.Original}}
        }()
      imports:
        - "context"
        - "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"
```

**Transformation result:**
```go
// Original
client, err := as.NewClient(policy, hosts...)

// After transformation
client, err := func() (*as.Client, error) {
    ctx, done := whatapsql.Start(context.Background(), "aerospike")
    defer done()
    return as.NewClient(policy, hosts...)
}()
```

### Template Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Original}}` | Matched original code | `gin.Default()` |
| `{{.Var}}` | Assigned variable name | `r` (from r := ...) |
| `{{.Args}}` | All function arguments | `policy, hosts...` |
| `{{.Arg0}}`, `{{.Arg1}}` | Individual arguments | First, second argument |
| `{{.FuncName}}` | Function name | `Default` |
| `{{.PkgName}}` | Package name | `gin` |

---

## Exclude Patterns

Specify file patterns to exclude from instrumentation.

### Default Exclude Patterns

When no `exclude` patterns are specified in the config file, the following patterns are **automatically applied**:

```go
// Default patterns (auto-applied)
**/*.pb.go           // protobuf generated files
**/*.pb.gw.go        // grpc-gateway generated files
**/*_grpc.pb.go      // grpc generated files
**/*.connect.go      // connect-go generated files
**/*_generated.go    // auto-generated files
**/*_gen.go          // code generator output
**/*_test.go         // test files
vendor/**            // vendor directory
.git/**              // git directory
node_modules/**      // node_modules directory
whatap-instrumented/**  // instrumented output directory
```

These patterns prevent instrumentation of:
- **Generated code** (protobuf, grpc-gateway, etc.) - would cause compile errors
- **Test files** - not needed for production monitoring
- **Dependency directories** (vendor, node_modules) - third-party code

### Custom Exclude Patterns

When you specify custom `exclude` patterns, they **replace** the defaults:

```yaml
# .whatap/config.yaml
exclude:
  - "*_test.go"
  - "vendor/**"
  - "internal/legacy/**"    # Custom: skip legacy code
```

### Glob Pattern Syntax

| Pattern | Description | Example Match |
|---------|-------------|---------------|
| `*` | Match any characters in filename | `*.go` → `main.go` |
| `**` | Match any directory recursively | `**/test/**` → `a/b/test/c/d.go` |
| `?` | Match single character | `test?.go` → `test1.go` |
| `[abc]` | Match character class | `test[12].go` → `test1.go` |

### Examples

#### Example 1: Skip specific directory

```yaml
exclude:
  - "**/*.pb.go"
  - "**/*_test.go"
  - "migrations/**"      # Skip database migrations
```

#### Example 2: Skip generated files only

```yaml
exclude:
  - "**/*.pb.go"
  - "**/*.pb.gw.go"
  - "**/*_generated.go"
```

> **Note**: System paths (`GOROOT`, `GOMODCACHE`) are always excluded regardless of config settings.

---

## Copy Exclude Directories

Specify directories to exclude when copying source files (Build Wrapper mode).

### Default Copy Exclude

The following directories are **always excluded** from copying:

```
.git                  // Git repository
.idea                 // IntelliJ IDEA
.vscode               // VS Code
.github               // GitHub Actions
whatap-instrumented   // Default tool output directory
node_modules          // Large, not needed for Go build
```

> **Note**: `build` and `dist` directories are **NOT** excluded because they are common `go:embed` targets in Go projects (e.g., `ui/build/`, `web/dist/` for frontend assets).

### Custom Copy Exclude

Add additional directories to exclude:

```yaml
# .whatap/config.yaml
copy_exclude:
  - "tmp"           # Custom temporary directory
  - "cache"         # Custom cache directory
  - "data"          # Large data directory
```

> **Note**: Custom `copy_exclude` entries are **added** to the default list (not replaced).

---

## Priority

Configuration value priority:

```
CLI options > Environment variables > Config file > Defaults
```

| Source | Example | Priority |
|--------|---------|----------|
| CLI option | `--error-tracking` | 1 (highest) |
| Environment variable | `GO_API_AST_DEBUG=1` | 2 |
| Config file | `error_tracking: true` | 3 |
| Default | `false` | 4 |

> Even if `error_tracking: true` is set in config file, specifying `--error-tracking=false` via CLI takes precedence.

---

## Usage Examples

### Basic Usage

```bash
# Auto-load .whatap/config.yaml
whatap-go-inst go build ./...

# Explicitly specify config file
whatap-go-inst --config ./my-config.yaml go build ./...
```

### Example 1: minimal - trace only

```yaml
# .whatap/config.yaml
instrumentation:
  preset: "minimal"
```

Result:
- `trace.Init(nil)`, `defer trace.Shutdown()` added
- Framework middleware not added

### Example 2: Instrument only web and DB

```yaml
instrumentation:
  preset: "custom"
  enabled_packages:
    - "gin"
    - "echo"
    - "sql"
    - "gorm"
```

### Example 3: Exclude specific packages from full

```yaml
instrumentation:
  preset: "full"
  disabled_packages:
    - "k8s"
    - "grpc"
    - "aerospike"
```

### Example 4: Enable error tracking

```yaml
instrumentation:
  error_tracking: true
  preset: "web"
```

Result: `trace.Error(err)` auto-inserted in `if err != nil` patterns

### Example 5: Debug mode

```yaml
instrumentation:
  debug: true
  preset: "full"
```

Output example:
```
[whatap-go-inst] Config file: .whatap/config.yaml
[whatap-go-inst] ErrorTracking: false
```

---

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `GO_API_AST_DEBUG` | Enable debug output | `GO_API_AST_DEBUG=1` |
| `GO_API_AST_OUTPUT_DIR` | Instrumented source output directory | `GO_API_AST_OUTPUT_DIR=./out` |
| `WHATAP_INST_CONFIG` | Config file path | `WHATAP_INST_CONFIG=./config.yaml` |

### Environment Variable Usage Examples

```bash
# Build with debug mode
GO_API_AST_DEBUG=1 whatap-go-inst go build ./...

# Specify config file
WHATAP_INST_CONFIG=./prod-config.yaml whatap-go-inst go build ./...
```

---

## Log Collection Settings (whatap.conf)

> **Note**: Log collection works with both whatap-go-inst AST instrumentation and whatap.conf runtime settings.

### AST Instrumentation (whatap-go-inst)

Logging library instrumentation is included in the `log` preset.

```yaml
# .whatap/config.yaml
instrumentation:
  preset: "full"     # Includes log
  # or
  preset: "log"      # Log only
```

Code injected after instrumentation:
| Library | Injected Code |
|---------|---------------|
| Standard log | `log.SetOutput(logsink.GetTraceLogWriter(os.Stderr))` |
| logrus | `logrus.SetOutput(logsink.GetTraceLogWriter(os.Stderr))` |
| zap | `logsink.HookStderr()` |

### Runtime Settings (whatap.conf)

Configure log collection method in whatap.conf.

#### Defaults and Recommended Settings

go-api defaults recommend TraceLogWriter.

| Setting | Default | Description |
|---------|---------|-------------|
| `logsink_enabled` | `false` | Overall log collection on/off |
| `logsink_trace_enabled` | **`true`** | TraceLogWriter (recommended) |
| `logsink_stdout_enabled` | `false` | ProxyStream stdout (legacy) |
| `logsink_stderr_enabled` | `false` | ProxyStream stderr (legacy) |
| `logsink_fmt_enabled` | `false` | whatapfmt (under review) |

```ini
# whatap.conf - Just enable logsink_enabled for auto TraceLogWriter use
logsink_enabled=true
```

#### Log Collection Method Comparison

| Method | Setting | Transaction Correlation | Recommended |
|--------|---------|------------------------|-------------|
| **TraceLogWriter** | `logsink_trace_enabled=true` | txid/mtid/gid included | **Recommended** |
| ProxyStream (stdout) | `logsink_stdout_enabled=true` | No correlation | Legacy |
| ProxyStream (stderr) | `logsink_stderr_enabled=true` | No correlation | Legacy |

#### Why is TraceLogWriter Recommended?

| Item | TraceLogWriter | ProxyStream |
|------|----------------|-------------|
| **Transaction correlation** | Logs include @txid, @mtid, @gid | No transaction tracking |
| **Performance** | No pipe overhead | Requires pipe goroutine |
| **WhaTap dashboard** | Transaction -> Log connection | Only separate logs displayed |

> **Note**: When instrumented with whatap-go-inst, TraceLogWriter is auto-injected. ProxyStream is no longer needed, so disabling is recommended.

### Log Collection Configuration Examples

#### Example 1: Recommended settings (full activation)

```yaml
# .whatap/config.yaml
instrumentation:
  preset: "full"
```

```ini
# whatap.conf
logsink_enabled=true
logsink_trace_enabled=true
logsink_stdout_enabled=false
logsink_stderr_enabled=false
```

#### Example 2: Exclude log instrumentation

When log collection is not needed:

```yaml
# .whatap/config.yaml
instrumentation:
  preset: "full"
  disabled_packages:
    - "log"
    - "logrus"
    - "zap"
```

```ini
# whatap.conf
logsink_enabled=false
```

---

## Related Documentation

- [User Guide](./user-guide.md) - Full usage guide
- [Instrumentation Rules](./instrumentation-rules.md) - Detailed transformation patterns
