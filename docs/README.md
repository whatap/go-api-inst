# whatap-go-inst

Go AST-based automatic instrumentation tool for WhaTap APM monitoring.

## Quick Start

### Build Wrapper (Recommended)

```bash
# Install (Linux amd64)
curl -sSL https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_amd64.tar.gz | tar xz
sudo mv whatap-go-inst /usr/local/bin/

# Build (no init required)
whatap-go-inst go build ./...
```

### Direct Source Modification

```bash
whatap-go-inst inject -s ./myapp -o ./instrumented
cd instrumented && go get github.com/whatap/go-api@latest && go build .
```

## Instrumentation Modes

| Mode | Command | Source Changes | Use Case |
|------|---------|---------------|----------|
| [Build Wrapper](./build-wrapper.md) | `whatap-go-inst go build` | No | **Recommended** |
| [Source Modify](./source-inject.md) | `whatap-go-inst inject` | Yes (separate dir) | Review/Compare |

## Documentation

| Document | Description |
|----------|-------------|
| [User Guide](./user-guide.md) | Full CLI reference, options, commands |
| [Build Wrapper Mode](./build-wrapper.md) | Build wrapper details |
| [Source Inject Mode](./source-inject.md) | Direct source modification |
| [Multi-Module Projects](./multi-module.md) | External module and multi-module instrumentation |
| [Configuration Guide](./config.md) | Config file, presets, packages |
| [Custom Instrumentation](./custom-instrumentation.md) | User-defined instrumentation rules (5 types) |
| [Instrumentation Rules](./instrumentation-rules.md) | Transformation patterns per framework |
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
