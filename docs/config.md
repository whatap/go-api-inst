# Configuration Guide

Detailed guide for whatap-go-inst configuration files.

---

## Config File Location

Config files are searched in the following order:

| Order | Location | Description |
|-------|----------|-------------|
| 1 | `--config` flag | Explicitly specified via CLI |
| 2 | `WHATAP_INST_CONFIG` env var | Specified via environment variable |
| 3 | `.whatap/config.yaml` | **Default recommended location** |
| 4 | `.whatap/whatap.yaml` | Alternative config file |

---

## Config File Format

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

# User-defined instrumentation — see custom-instrumentation.md
# custom:
#   inject: []
#   replace: []
#   hook: []
#   transform: []

# External modules to instrument from GOMODCACHE
# external_modules:
#   - "mycompany.com/internal/lib"

# File patterns to exclude (optional - defaults auto-applied if omitted)
# exclude:
#   - "*_test.go"
#   - "vendor/**"
```

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

- `preset: full` + `disabled_packages: ["grpc"]` → All except gRPC
- `preset: web` + `enabled_packages: ["sql"]` → Web + SQL
- `preset: custom` + `enabled_packages: ["gin", "sql"]` → Only Gin and SQL

---

## Package List

### Web Frameworks (`web`)

| Package Name | Library | Injected Code |
|--------------|---------|---------------|
| `gin` | github.com/gin-gonic/gin | `whatapgin.Middleware()` |
| `echo` | github.com/labstack/echo/v4 | `whatapecho.Middleware()` |
| `fiber` | github.com/gofiber/fiber/v2 | `whatapfiber.Middleware()` |
| `chi` | github.com/go-chi/chi/v5 | `whatapchi.WrapRouter()` |
| `gorilla` | github.com/gorilla/mux | `whatapmux.WrapRouter()` |
| `nethttp` | net/http | `whataphttp.Func()`, `whataphttp.WrapHandler()` |
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
| `logrus` | github.com/sirupsen/logrus | Hook-based + `WrapLogger()` |
| `zap` | go.uber.org/zap | `logsink.HookStderr()` |

### Wrap Functions

For struct field initialization and instance creation patterns, the following
Wrap functions are injected automatically:

| Package | Function | Signature |
|---------|----------|-----------|
| gin | WrapEngine | `whatapgin.WrapEngine(*gin.Engine) *gin.Engine` |
| echo | WrapEcho | `whatapecho.WrapEcho(*echo.Echo) *echo.Echo` |
| fiber | WrapApp | `whatapfiber.WrapApp(*fiber.App) *fiber.App` |
| chi | WrapRouter | `whatapchi.WrapRouter[T any](r T) T` |
| gorilla | WrapRouter | `whatapmux.WrapRouter(*mux.Router) *mux.Router` |
| nethttp | WrapHandler | `whataphttp.WrapHandler(http.Handler) http.Handler` |
| fasthttp | WrapHandler | `whatapfasthttp.WrapHandler(fasthttp.RequestHandler) fasthttp.RequestHandler` |
| sarama | WrapConfig | `whatapsarama.WrapConfig(*sarama.Config) *sarama.Config` |
| logrus | WrapLogger | `whataplogrus.WrapLogger(*logrus.Logger) *logrus.Logger` |
| log | (inline) | `log.New(logsink.GetTraceLogWriter(w), prefix, flag)` |

---

## Custom Settings

> See [Custom Instrumentation Guide](./custom-instrumentation.md) for detailed documentation.

5 types of user-defined instrumentation rules in the `custom` section:

| Setting | Purpose | Target |
|---------|---------|--------|
| `add` | Create new files/functions | Package |
| `inject` | Insert code inside function definitions | Function definition |
| `replace` | Replace function calls | Function call |
| `hook` | Insert before/after function calls | Function call |
| `transform` | Code pattern → template transformation | All code |

Execution order: `add → inject → replace → hook → transform`

### Template Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Original}}` | Matched original code | `gin.Default()` |
| `{{.Var}}` | Assigned variable name | `r` (from r := ...) |
| `{{.Args}}` | All function arguments | `policy, hosts...` |
| `{{.Arg0}}`, `{{.Arg1}}` | Individual arguments | First, second argument |
| `{{.FuncName}}` | Function name | `Default` |
| `{{.PkgName}}` | Package name | `gin` |

User-defined variables are also supported via `vars` field — use `{{.Vars.key}}` in templates.

---

## Exclude Patterns

Specify file patterns to exclude from instrumentation.

### Default Exclude Patterns

When no `exclude` patterns are specified, the following are **automatically applied**:

```
**/*.pb.go                          // protobuf generated files
**/*.pb.gw.go                       // grpc-gateway generated files
**/*_grpc.pb.go                     // grpc generated files
**/*.connect.go                     // connect-go generated files
**/*_generated.go                   // auto-generated files
**/*_gen.go                         // code generator output
**/*_test.go                        // test files
vendor/**                           // vendor directory
.git/**                             // git directory
node_modules/**                     // node_modules directory
whatap-instrumented/**              // instrumented output directory
**/github.com/whatap/go-api/**      // whatap go-api library
```

### Custom Exclude Patterns

When you specify custom `exclude` patterns, they **replace** the defaults:

```yaml
exclude:
  - "*_test.go"
  - "vendor/**"
  - "internal/legacy/**"    # Custom: skip legacy code
```

> **Note**: System paths (`GOROOT`, `GOMODCACHE`) are always excluded regardless of config settings.

---

## Copy Exclude Directories

Directories to exclude when copying source files (Build Wrapper mode).

### Default Copy Exclude

Always excluded: `.git`, `.idea`, `.vscode`, `.github`, `whatap-instrumented`, `node_modules`

> **Note**: `build` and `dist` directories are NOT excluded because they are common `go:embed` targets.

### Custom Copy Exclude

```yaml
copy_exclude:
  - "tmp"
  - "cache"
  - "data"
```

> Custom entries are **added** to the default list (not replaced).

---

## External Modules

Specify external modules (in GOMODCACHE) to instrument.

```yaml
external_modules:
  - "mycompany.com/internal/lib"
  - "mycompany.com/internal/*"    # wildcard supported
```

- Modules are copied from GOMODCACHE (originals never modified)
- `replace` directives are automatically added to `go.mod`
- CLI `--external-module` values merge with config (not replaced)

> **Details**: [Multi-Module Projects](./multi-module.md)

---

## Priority

```
CLI options > Environment variables > Config file > Defaults
```

---

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `GO_API_AST_DEBUG` | Enable debug output | `GO_API_AST_DEBUG=1` |
| `GO_API_AST_OUTPUT_DIR` | Instrumented source output directory | `GO_API_AST_OUTPUT_DIR=./out` |
| `WHATAP_INST_CONFIG` | Config file path | `WHATAP_INST_CONFIG=./config.yaml` |

---

## Log Collection Settings (whatap.conf)

### AST Instrumentation

Logging library instrumentation is included in the `log` preset (and `full` preset).

### Runtime Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `logsink_enabled` | `false` | Overall log collection on/off |
| `logsink_trace_enabled` | **`true`** | TraceLogWriter (recommended) |
| `logsink_stdout_enabled` | `false` | ProxyStream stdout (legacy) |
| `logsink_stderr_enabled` | `false` | ProxyStream stderr (legacy) |

```ini
# whatap.conf - Just enable logsink_enabled for auto TraceLogWriter use
logsink_enabled=true
```

**TraceLogWriter** is recommended over ProxyStream because it supports transaction correlation (`@txid`, `@mtid`, `@gid`) and has no pipe overhead.

> When instrumented with whatap-go-inst, TraceLogWriter is auto-injected. ProxyStream is no longer needed.

---

## Related Documentation

- [User Guide](./user-guide.md) — Full CLI reference
- [Custom Instrumentation](./custom-instrumentation.md) — User-defined rules
- [Instrumentation Rules](./instrumentation-rules.md) — Transformation patterns
