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
version: 1

instrumentation:
  # Basic options
  error_tracking: false    # Inject error tracking code (--error-tracking)
  debug: false             # Enable debug output (GO_API_AST_DEBUG)
  output_dir: ""           # Instrumented source output directory

  # Package filter (v0.6.0 breaking change) — exact-match on package import paths.
  # Leave both empty to accept the default set (built-in rules for every
  # package actually imported by the project are registered automatically).
  enabled_packages: []     # Opt-in packages (e.g. "fmt")
  disabled_packages: []    # Packages to exclude (use the go.mod require path, or the stdlib short name)

  # Skip instrumentation for modules redirected via `replace` in go.mod.
  # Default behaviour (true) is the safe choice: forks may have signature-incompatible
  # APIs that break the whatap wrap (e.g. traefik's containous/mux). Set to false
  # only when you have verified the fork is signature-compatible with the
  # upstream package and want monitoring applied anyway.
  # skip_replaced_modules: true

# User-defined rules and file-generation add rules — see custom-instrumentation.md
# rules:
#   - type: replace
#     target: "database/sql.Open"
#     with: "whatapsql.Open"
# add:
#   - package: "main"
#     file: "whatap_init.go"
#     content: "..."

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
| `enabled_packages` | []string | `[]` | Opt-in list. Opt-in rules (currently `fmt.Print/Printf/Println`) register only when their package path is listed here |
| `disabled_packages` | []string | `[]` | Exclusion list. Rules whose package path appears here are skipped, even if they would otherwise be registered by default |
| `skip_replaced_modules` | bool | `true` | Skip Rules whose target module appears in a `go.mod` `replace` directive. Default is the safer behaviour — set to `false` only when your replace target is signature-compatible with the upstream package |

> **v0.6.0 breaking change — `preset` field removed.** The legacy `preset: full/minimal/web/database/external/log/custom` model has been replaced by the exact-match package filter above. The engine already loads every built-in rule up front and matches them precisely against your code, so a project-level pre-filter is no longer required. See [Migration from the legacy preset schema](#migration-from-the-legacy-preset-schema) below.

---

## Package Filter Semantics

```
For each built-in rule, using its target package path (P):
  1. If P is in disabled_packages → skip the rule (takes precedence)
  2. If the rule is opt-in (e.g. fmt) and P is NOT in enabled_packages → skip the rule
  3. Otherwise → register the rule
```

- **Exact match** — `disabled_packages: ["github.com/labstack/echo/v4"]` excludes only the v4 rules. Add `github.com/labstack/echo` on its own line to also exclude the legacy v3 rules. No prefix or wildcard matching.
- **Full paths for external modules, short names for stdlib** — `github.com/gin-gonic/gin`, `gorm.io/gorm`, `k8s.io/client-go` (external) / `fmt`, `log`, `database/sql`, `net/http` (stdlib).
- **Unknown paths warn, they do not fail** — if you list a package that no built-in rule matches (typo, fork, or a library whatap does not instrument yet), `whatap-go-inst` prints a `stderr` warning and the build continues.
- **User rules (the `rules:` array) are not filtered** — a rule you declare yourself is treated as an explicit opt-in and is always registered, regardless of `disabled_packages`.

### Opt-in rules

A few rules are **opt-in**: they do **not** register unless you list their package in `enabled_packages`:

| Package | Targets | Reason |
|---------|---------|--------|
| `fmt` | `fmt.Print`, `fmt.Printf`, `fmt.Println` | `whatapfmt.*` lives on a hot path for high-frequency log apps (Loki, Promtail). Opting in is required because the overhead is material for that workload (a noticeable p99 increase on Loki-class apps). |

Enable with:

```yaml
instrumentation:
  enabled_packages:
    - fmt
```

See [Supported Packages Catalog](./instrumentation-rules.md) for the full list.

---

## go.mod `replace` directive — `skip_replaced_modules`

When your `go.mod` redirects a module via a `replace` directive, the engine skips rules targeting that module by default. The whatap wrappers (e.g. `whatapmux.WrapRouter`) are written against the upstream API surface — applying them on top of a fork that diverges in signature or field shape can break the build (e.g. traefik's `containous/mux` fork).

### How the check works

- For every rule, the engine derives its target package path (`github.com/gorilla/mux.NewRouter` → `github.com/gorilla/mux`).
- If that path is present in `go.mod`'s `replace` left-hand side (exact match or sub-path), the rule is skipped for both function-decl and call-site matchers.
- Only that one rule is skipped — unrelated rules (e.g. `database/sql`, `net/http`) still apply.

### Forcing instrumentation through a replace

Set `skip_replaced_modules: false` if you have verified the fork is signature-compatible:

```yaml
instrumentation:
  skip_replaced_modules: false  # treat replaced modules like normal dependencies
```

Build failures that result from this opt-out are not bugs in the wrapper — they signal that the fork has diverged.

### Debug visibility

`GO_API_AST_DEBUG=1` surfaces every skip:

```
[v2-resolve] skip target="github.com/gorilla/mux.NewRouter" (replaced in go.mod)
```

---

## Migration from the legacy preset schema

| Legacy (preset schema) | Current (v0.6.0) |
|---|---|
| `preset: full` (or unset) | Unset. `fmt.Print*` is now opt-in and skipped by default — add `enabled_packages: [fmt]` to keep collecting. |
| `preset: minimal` | Stop using `whatap-go-inst` and run plain `go build` instead. No replacement flag (there is no equivalent "init only" mode). |
| `preset: web` / `database` / `custom` | Intent does not map 1-to-1. Pick the packages you want to exclude and list them under `disabled_packages` — copy/paste blocks live in [instrumentation-rules.md](./instrumentation-rules.md). |
| `enabled_packages: [gin]` | `enabled_packages: [github.com/gin-gonic/gin]` — short name → full go.mod require path. |
| `enabled_packages: [sql]` | `enabled_packages: [database/sql]` — stdlib uses its canonical import path too. |
| `disabled_packages: [fmt]` | Unchanged (`fmt` is already excluded by default, but the line is harmless). |
| Rule-level `name:` field | Removed along with the preset. Use `id:` for rule identifiers. |

**Automation note**: report JSON schema also changed — `config.preset` was removed, and `dependencies[].transformer` now emits the full package path instead of a short name. No known external consumers (only internal test scripts) read these values. See the release notes for details.

---

## Package List

Package paths below are the values you put in `enabled_packages` / `disabled_packages`. The full canonical catalog (with yaml template blocks for common exclusion patterns) lives in [instrumentation-rules.md](./instrumentation-rules.md).

### Web Frameworks

| Package Path | Injected Code |
|--------------|---------------|
| `github.com/gin-gonic/gin` | `whatapgin.Middleware()` |
| `github.com/labstack/echo` | `whatapecho.Middleware()` (v3) |
| `github.com/labstack/echo/v4` | `whatapecho.Middleware()` (v4) |
| `github.com/gofiber/fiber/v2` | `whatapfiber.Middleware()` |
| `github.com/go-chi/chi` | `whatapchi.WrapRouter()` |
| `github.com/go-chi/chi/v5` | `whatapchi.WrapRouter()` |
| `github.com/gorilla/mux` | `whatapmux.WrapRouter()` |
| `net/http` | `whataphttp.Func()`, `whataphttp.WrapHandler()` |
| `github.com/valyala/fasthttp` | `whatapfasthttp.Middleware()` |

### Database

| Package Path | Injected Code |
|--------------|---------------|
| `database/sql` | `whatapsql.Open()` |
| `github.com/jmoiron/sqlx` | `whatapsqlx.Open()` |
| `gorm.io/gorm` | `whatapgorm.Open()` |
| `github.com/jinzhu/gorm` | `whatapgorm.Open()` |

### External Services

| Package Path | Injected Code |
|--------------|---------------|
| `github.com/gomodule/redigo/redis` | `whatapredigo.Dial()` |
| `github.com/redis/go-redis/v9` | `whatapgoredis.NewClient()` |
| `github.com/go-redis/redis/v8` | `whatapgoredis.NewClient()` |
| `go.mongodb.org/mongo-driver/mongo` | `whatapmongo.Connect()` |
| `github.com/aerospike/aerospike-client-go/v6` | `whatapas.Wrap*()` (closure wrap) |
| `github.com/IBM/sarama` | Interceptor injection |
| `github.com/Shopify/sarama` | Interceptor injection |
| `google.golang.org/grpc` | Server/Client Interceptor |
| `k8s.io/client-go` | `config.Wrap()` |

### Logging Libraries

| Package Path | Injected Code | Default |
|--------------|---------------|:-------:|
| `log` | `log.SetOutput(logsink.GetTraceLogWriter())` | enabled |
| `github.com/sirupsen/logrus` | Hook-based + `WrapLogger()` | enabled |
| `go.uber.org/zap` | `logsink.HookStderr()` | enabled |
| `fmt` | `whatapfmt.Print/Printf/Println()` | **opt-in** — requires `enabled_packages: [fmt]` |

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

User-defined instrumentation rules are declared in the `rules:` array (with `add` rules in the top-level `add:` array). The legacy `custom: { … }` block was removed in v0.6.0 — see the [Custom Instrumentation Guide](./custom-instrumentation.md) §11 migration table. The main user-facing rule types:

| Type | Purpose | Target |
|------|---------|--------|
| `add` | Create new files/functions | Package (top-level `add:`) |
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

### Source Instrumentation

Logging library instrumentation is enabled by default for `log`, `github.com/sirupsen/logrus`, and `go.uber.org/zap`. `fmt.Print/Printf/Println` requires opt-in via `enabled_packages: [fmt]`.

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
