# Supported Versions Summary

## Web Frameworks

| Library | Supported Versions | Import Path | Notes |
|---------|-------------------|-------------|-------|
| Gin | All versions | `github.com/gin-gonic/gin` | |
| Echo | v3, v4 | `github.com/labstack/echo`, `echo/v4` | v3 is legacy |
| Fiber | v1, v2 | `github.com/gofiber/fiber`, `fiber/v2` | v1 is legacy |
| Chi | v4, v5 | `github.com/go-chi/chi`, `chi/v5` | |
| Gorilla Mux | All versions | `github.com/gorilla/mux` | |
| net/http | Go standard | `net/http` | |
| FastHTTP | All versions | `github.com/valyala/fasthttp` | |

## Database

| Library | Supported Versions | Import Path | Notes |
|---------|-------------------|-------------|-------|
| database/sql | Go standard | `database/sql` | |
| sqlx | All versions | `github.com/jmoiron/sqlx` | |
| GORM (gorm.io) | v1 | `gorm.io/gorm` | Current GORM |
| GORM (jinzhu) | v1 | `github.com/jinzhu/gorm` | Legacy GORM |

## Redis

| Library | Supported Versions | Import Path | Notes |
|---------|-------------------|-------------|-------|
| go-redis | v8, v9 | `github.com/redis/go-redis/v9` | New path (v9) |
| go-redis | v8, v9 | `github.com/go-redis/redis/v8` | Old path (v8) |
| Redigo | All versions | `github.com/gomodule/redigo` | |

## Message Queue / RPC / Cloud

| Library | Supported Versions | Import Path | Notes |
|---------|-------------------|-------------|-------|
| Sarama (IBM) | All versions | `github.com/IBM/sarama` | |
| Sarama (Shopify) | All versions | `github.com/Shopify/sarama` | |
| gRPC | All versions | `google.golang.org/grpc` | |
| Kubernetes client-go | All versions | `k8s.io/client-go` | |

## NoSQL

| Library | Supported Versions | Import Path | Notes |
|---------|-------------------|-------------|-------|
| MongoDB | v1, v2 | `go.mongodb.org/mongo-driver` | CommandMonitor based |

## Logging Libraries

| Library | Supported Versions | Import Path | Notes |
|---------|-------------------|-------------|-------|
| Standard log | Go standard | `log` | SetOutput insertion |
| logrus | All versions | `github.com/sirupsen/logrus` | SetOutput insertion, alias support |
| zap | All versions | `go.uber.org/zap` | HookStderr used |

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
