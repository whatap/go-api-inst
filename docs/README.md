# whatap-go-inst

Go AST-based automatic instrumentation — adds WhaTap APM monitoring to your Go application at build time, with no source code changes.

## Quick Start

### Build Wrapper (Recommended)

```bash
# Install (Linux amd64)
curl -sSL https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_amd64.tar.gz | tar xz
sudo mv whatap-go-inst /usr/local/bin/

# Build (no init required)
whatap-go-inst go build ./...
```

### Inspect Transformed Source

```bash
# Dump the instrumented copy alongside the build (originals stay untouched)
whatap-go-inst --output=./instrumented go build -o myapp ./...
```

## Instrumentation Workflow

| Command | Source Changes | Use Case |
|---------|---------------|----------|
| `whatap-go-inst go build` | None | **Default** — builds instrumented binary |
| `whatap-go-inst --output go build` | None (dumps transformed copy to `whatap-instrumented/`) | Review / CI artifact / diff |
| `whatap-go-inst --output=./dir go build` | None (dumps to `./dir`) | Custom inspection path |

> Legacy `whatap-go-inst inject` / `whatap-go-inst generate` / `whatap-go-inst init` / `whatap-go-inst uninit` subcommands and `--wrap` / `--no-output` flags were removed in v0.6.0. The build wrapper + `--output` combo is the single workflow (handles dependency add + instrumentation + build in one step). **`whatap-go-inst remove` is still shipped** — it strips manually written instrumentation calls (not needed in the default build-wrapper flow because the originals are never modified).

## Documentation

| Document | Description |
|----------|-------------|
| [User Guide](./user-guide.md) | Full CLI reference, options, commands |
| [Build Wrapper Mode](./build-wrapper.md) | Build wrapper details |
| [Inspect Transformed Source (`--output`)](./source-inject.md) | Dump instrumented source for review |
| [Multi-Module Projects](./multi-module.md) | External module and multi-module instrumentation |
| [Configuration Guide](./config.md) | Config file, presets, packages |
| [Custom Instrumentation](./custom-instrumentation.md) | User-defined instrumentation rules (5 types) |
| [Instrumentation Rules](./instrumentation-rules.md) | Transformation patterns per framework |
| [LLM Monitoring](./llm-monitoring.md) | LLM API call monitoring — auto-inject + manual API + URL auto-match |
| [Troubleshooting](./troubleshooting.md) | Common errors and solutions |

## Supported Frameworks

### Web Frameworks

| Framework | Middleware/Wrapper | Status |
|-----------|-------------------|--------|
| Gin | `whatapgin.Middleware()` | Supported |
| Echo | `whatapecho.Middleware()` | v3, v4 (v5+ skipped) |
| Fiber | `whatapfiber.Middleware()` | v2 only |
| Chi | `whatapchi.WrapRouter()` | Supported |
| Gorilla Mux | `whatapmux.WrapRouter()` | Supported |
| net/http | `whataphttp.Func()`, `WrapHandler()` | Supported |
| FastHTTP | `whatapfasthttp.Middleware()` | Supported |

### Database

| Library | Transformation | Status |
|---------|----------------|--------|
| database/sql | `whatapsql.Open()` | Supported |
| sqlx | `whatapsqlx.Open()` | Supported |
| GORM (gorm.io) | `whatapgorm.Open()` | Supported |
| GORM (jinzhu) | `whatapgorm.Open()` | Supported |

### External Services

| Library | Transformation | Status |
|---------|----------------|--------|
| go-redis v9 | `whatapgoredis.NewClient()` | v9 (v10+ skipped) |
| go-redis v8 | `whatapgoredis.NewClient()` | v8 (v7- skipped) |
| Redigo | `whatapredigo.Dial()` | Supported |
| MongoDB | `whatapmongo.Connect()` | Supported |
| Aerospike | `whatapsql.Wrap()` | v6, v8 (v7, v9+ skipped) |
| Sarama | Interceptor injection | Supported |
| gRPC | Server/Client Interceptor | Supported |
| Kubernetes | `config.Wrap()` | Supported |

### Logging

| Library | Transformation | Status |
|---------|----------------|--------|
| log | `logsink.GetTraceLogWriter()` | Supported |
| logrus | Hook-based + `WrapLogger()` | Supported |
| zap | `logsink.HookStderr()` | Supported |
| fmt | `whatapfmt.Print*()` | Supported |

## Related Projects

- [whatap/go-api](https://github.com/whatap/go-api) — Monitoring library
- [whatap/go-api-example](https://github.com/whatap/go-api-example) — Usage examples
