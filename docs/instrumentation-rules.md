# Instrumentation Transformation Rules by Package

Documentation of code transformation rules that whatap-go-inst performs for each package.

## Table of Contents

| Document | Contents |
|----------|----------|
| [Common Transformations](./rules/common.md) | Import addition, main() initialization, error tracking, context preservation, version independence |
| [Web Frameworks](./rules/web-frameworks.md) | Gin, Echo, Fiber, Chi, Gorilla, net/http, FastHTTP |
| [Database](./rules/database.md) | database/sql, sqlx, GORM (gorm.io, jinzhu) |
| [External Services](./rules/external-services.md) | Redis (Redigo, go-redis), MongoDB, **Aerospike**, Kafka (Sarama), gRPC, Kubernetes |
| [Logging Libraries](./rules/log.md) | Standard log, logrus, zap, **fmt (whatapfmt)** |
| [LLM SDKs](./llm-monitoring.md) | sashabaranov, Eino (eino-ext), Anthropic, openai-go — auto-inject adapters (nested module, requires `llm_enabled=true`) |
| [Remove Rules](./rules/remove.md) | Stripping hand-written whatap/go-api calls and imports |
| [Supported Versions](./rules/versions.md) | Supported versions by package, implementation TODOs |

---

## Quick Reference

### Whatap Import Path Prefix

All instrumentation package paths start with this prefix:

```
github.com/whatap/go-api/instrumentation/
```

### Main Transformation Patterns

| Type | Pattern | Example |
|------|---------|---------|
| Middleware insertion | `.Use(whatapXXX.Middleware())` | Gin, Echo, Fiber |
| Function value passing | `.Use(whatapXXX.Middleware)` | Chi |
| In-place wrapping | `whatapmux.WrapRouter(mux.NewRouter())` | Gorilla |
| Handler wrapping | `whataphttp.Func(handler)`, `whataphttp.WrapHandler(handler)` | net/http |
| Function replacement | `whatapsql.Open()` | database/sql, sqlx, GORM |
| Closure wrapping | `whatapsql.Wrap(ctx, ...)` | Aerospike |
| Interceptor addition | `grpc.ChainUnaryInterceptor(...)` | gRPC |
| Hook insertion | `config.Wrap(...)` | Kubernetes |

---

## Supported Packages Catalog

The values below are the exact strings you put in `instrumentation.enabled_packages` / `disabled_packages` in `.whatap/config.yaml`. Matching is **exact**: `github.com/labstack/echo` and `github.com/labstack/echo/v4` are independent entries, and there is no prefix or wildcard matching. For external modules use the go.mod `require` path; for stdlib use the canonical import path (`fmt`, `log`, `database/sql`, `net/http`).

### Enabled by default (Default=true)

These rules register automatically whenever the project imports the package. No configuration is required.

| Area | Package Path |
|---|---|
| Web | `github.com/gin-gonic/gin` |
|  | `github.com/labstack/echo` |
|  | `github.com/labstack/echo/v4` |
|  | `github.com/gofiber/fiber/v2` |
|  | `github.com/go-chi/chi` |
|  | `github.com/go-chi/chi/v5` |
|  | `github.com/gorilla/mux` |
|  | `net/http` |
|  | `github.com/valyala/fasthttp` |
| Database | `database/sql` |
|  | `github.com/jmoiron/sqlx` |
|  | `gorm.io/gorm` |
|  | `github.com/jinzhu/gorm` |
| External | `github.com/gomodule/redigo/redis` |
|  | `github.com/redis/go-redis/v9` |
|  | `github.com/go-redis/redis/v8` |
|  | `go.mongodb.org/mongo-driver/mongo` |
|  | `github.com/aerospike/aerospike-client-go/v6` |
|  | `github.com/IBM/sarama` |
|  | `github.com/Shopify/sarama` |
|  | `google.golang.org/grpc` |
|  | `k8s.io/client-go` |
| Log | `log` |
|  | `github.com/sirupsen/logrus` |
|  | `go.uber.org/zap` |
| LLM | `github.com/sashabaranov/go-openai` |
|  | `github.com/cloudwego/eino-ext/components/model/openai` |
|  | `github.com/cloudwego/eino-ext/components/model/claude` |
|  | `github.com/anthropics/anthropic-sdk-go` |
|  | `github.com/openai/openai-go` |

> LLM rules require `llm_enabled=true` in `whatap.conf` at runtime. They wrap the SDK's HTTPClient transport so the RoundTrip single entry point produces the LLM step. For eino-ext, both the constructor and the compose pipeline are auto-injected: compose methods are wrapped at the call site (`AppendChatModel(WrapToolCallingChatModel(cm))`) and direct `Generate`/`Stream` calls are transformed, so model name and token usage are captured without manual `WrapChatModel(cm)` calls. For Anthropic and openai-go, the canonical `client.<Service>.New(ctx, params)` form is auto-converted; call sites passing extra trailing `option.RequestOption` arguments need a manual `whatapanthropic.WrapAndNewMessage(...)` / `whatapopenaigo.WrapAndNewChatCompletion(...)` call.

### Opt-in (disabled by default)

These rules register **only** when the user lists them under `enabled_packages`. Reason: the wrapper lives on a hot path and introduces noticeable overhead in workloads that exercise it heavily.

| Area | Package | Reason for opt-in |
|---|---|---|
| Log | `fmt` | High-frequency hot-path overhead, noticeable on log shippers (observed on Loki 2.9.x). Typical apps log a handful of lines per request and are unaffected; log shippers / promtail-like workloads are. |

### yaml templates (copy/paste)

```yaml
# 1. Opt into fmt collection (default is off)
instrumentation:
  enabled_packages:
    - fmt

# 2. Disable every log library (keep sql/redis/mongo/grpc/…)
instrumentation:
  disabled_packages:
    - log
    - github.com/sirupsen/logrus
    - go.uber.org/zap
    # fmt already defaults to off; listing it is harmless but unnecessary.

# 3. Instrument only web traffic (exclude everything else)
instrumentation:
  disabled_packages:
    - database/sql
    - github.com/jmoiron/sqlx
    - gorm.io/gorm
    - github.com/jinzhu/gorm
    - github.com/gomodule/redigo/redis
    - github.com/redis/go-redis/v9
    - github.com/go-redis/redis/v8
    - go.mongodb.org/mongo-driver/mongo
    - github.com/aerospike/aerospike-client-go/v6
    - github.com/IBM/sarama
    - github.com/Shopify/sarama
    - google.golang.org/grpc
    - k8s.io/client-go
    - log
    - github.com/sirupsen/logrus
    - go.uber.org/zap

# 4. Exclude one specific major version (exact match, not a prefix)
instrumentation:
  disabled_packages:
    - github.com/labstack/echo/v4    # v4 only; the legacy github.com/labstack/echo rules still register
```

If you list a package path that no built-in rule matches (typo, fork you have in `replace`, or a library whatap does not instrument yet) `whatap-go-inst` prints a `stderr` warning (`[whatap-go-inst] unknown package in enabled_packages: "…"`) and the build continues. User-defined rules declared in the `rules:` array are **not** filtered by `disabled_packages` — they always register because declaring one is treated as an explicit opt-in.

---

## References

- [go-api Documentation](https://pkg.go.dev/github.com/whatap/go-api)
- [go-api-example](https://github.com/whatap/go-api-example)
