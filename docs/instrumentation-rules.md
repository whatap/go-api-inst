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
| [Remove Rules](./rules/remove.md) | Code removal and original restoration, restoration test results |
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
| Function value passing | `.Use(whatapXXX.Middleware)` | Chi, Gorilla |
| Handler wrapping | `whataphttp.Func(handler)` | net/http |
| Function replacement | `whatapsql.Open()` | database/sql, sqlx, GORM |
| Closure wrapping | `whatapsql.Wrap(ctx, ...)` | Aerospike |
| Interceptor addition | `grpc.UnaryInterceptor(...)` | gRPC |
| Hook insertion | `config.Wrap(...)` | Kubernetes |

---

## References

- [go-api Documentation](https://pkg.go.dev/github.com/whatap/go-api)
- [go-api-example](https://github.com/whatap/go-api-example)
