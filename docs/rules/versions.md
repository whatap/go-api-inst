# Supported Versions Summary

## Web Frameworks

| Library | Supported Versions | Import Path | Unsupported |
|---------|-------------------|-------------|-------------|
| Gin | All versions | `github.com/gin-gonic/gin` | - |
| Echo | v3, v4 | `github.com/labstack/echo`, `echo/v4` | v5+ |
| Fiber | v2 | `github.com/gofiber/fiber/v2` | v1, v3+ |
| Chi | v4, v5 | `github.com/go-chi/chi`, `chi/v5` | v6+ |
| Gorilla Mux | All versions | `github.com/gorilla/mux` | - |
| net/http | Go standard | `net/http` | - |
| FastHTTP | All versions | `github.com/valyala/fasthttp` | - |

## Database

| Library | Supported Versions | Import Path | Unsupported |
|---------|-------------------|-------------|-------------|
| database/sql | Go standard | `database/sql` | - |
| sqlx | All versions | `github.com/jmoiron/sqlx` | - |
| GORM (gorm.io) | v1 | `gorm.io/gorm` | - |
| GORM (jinzhu) | v1 | `github.com/jinzhu/gorm` | - |

## Redis

| Library | Supported Versions | Import Path | Unsupported |
|---------|-------------------|-------------|-------------|
| go-redis | v9 | `github.com/redis/go-redis/v9` | v10+ |
| go-redis | v8 | `github.com/go-redis/redis/v8` | v7- |
| Redigo | All versions | `github.com/gomodule/redigo` | - |

## Message Queue / RPC / Cloud

| Library | Supported Versions | Import Path | Unsupported |
|---------|-------------------|-------------|-------------|
| Sarama (IBM) | All versions | `github.com/IBM/sarama` | - |
| Sarama (Shopify) | All versions | `github.com/Shopify/sarama` | - |
| gRPC | All versions | `google.golang.org/grpc` | - |
| Kubernetes client-go | All versions | `k8s.io/client-go` | - |

## NoSQL

| Library | Supported Versions | Import Path | Unsupported |
|---------|-------------------|-------------|-------------|
| MongoDB | v1, v2 | `go.mongodb.org/mongo-driver` | - |
| Aerospike | v6, v8 | `github.com/aerospike/aerospike-client-go` | v5-, v7, v9+ |

## Logging Libraries

| Library | Supported Versions | Import Path | Unsupported |
|---------|-------------------|-------------|-------------|
| Standard log | Go standard | `log` | - |
| logrus | All versions | `github.com/sirupsen/logrus` | - |
| zap | All versions | `go.uber.org/zap` | - |
| fmt | Go standard | `fmt` | - |

---

## Version Filtering (v0.5.4+)

Unsupported major versions are automatically blocked from instrumentation.
For libraries using prefix-based import matching, the transformer's `Detect()` method
checks `SupportedVersions()` against the major version suffix in the import path.
If the version is not in the supported list, `Detect()` returns `false` and the
transformer is silently skipped — no error is produced.

| Transformer | Prefix | Supported Versions | Skipped Example |
|-------------|--------|-------------------|-----------------|
| echo | `github.com/labstack/echo` | `""`, `v4` | echo/v5 |
| fiber | `github.com/gofiber/fiber` | `v2` | fiber (v1), fiber/v3 |
| chi | `github.com/go-chi/chi` | `""`, `v5` | chi/v6 |
| goredis | `github.com/redis/go-redis` | `v9` | go-redis/v10 |
| goredis | `github.com/go-redis/redis` | `v8` | redis/v7 |
| aerospike | `github.com/aerospike/aerospike-client-go` | `v6`, `v8` | v7, v9 |

> The remaining 17 transformers use exact match (`HasImport`) and are not affected by version filtering.

---

## Planned Implementation (TODO)

### Medium Priority
- `github.com/segmentio/kafka-go` → Kafka alternative
- `github.com/jackc/pgx/v5` → PostgreSQL direct driver

### Low Priority
- `github.com/aws/aws-sdk-go-v2` → AWS SDK
- `github.com/julienschmidt/httprouter` → Lightweight router

---

## References

- [go-api Documentation](https://pkg.go.dev/github.com/whatap/go-api)
- [go-api-example](https://github.com/whatap/go-api-example)
