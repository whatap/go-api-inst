# Remove Rules (Code Removal)

Use `whatap-go-inst remove` command to remove instrumentation code and restore to original code.

## Common Removal

```go
// Removed
import "github.com/whatap/go-api/trace"

// Removed from main() function
trace.Init(nil)
defer trace.Shutdown()
```

---

## Web Framework Middleware Removal

```go
// Removed (gin, echo, fiber)
r.Use(whatapgin.Middleware())
e.Use(whatapecho.Middleware())
app.Use(whatapfiber.Middleware())

// Removed (chi, gorilla)
r.Use(whatapchi.Middleware)
r.Use(whatapmux.Middleware)
```

---

## net/http Handler Wrapping Removal

```go
// Before (instrumented)
mux.HandleFunc("/", whataphttp.Func(handleRoot))
mux.Handle("/api", whataphttp.Handler(apiHandler))

// After (removed)
mux.HandleFunc("/", handleRoot)
mux.Handle("/api", apiHandler)
```

---

## FastHTTP Handler Wrapping Removal

```go
// Before (instrumented)
r.GET("/", whatapfasthttp.Func(handler))
r.GET("/tasks", whatapfasthttp.Func(getTasks))

// After (removed)
r.GET("/", handler)
r.GET("/tasks", getTasks)
```

---

## HTTP Client Restoration

```go
// Before (instrumented)
resp, err := whataphttp.HttpGet(context.Background(), url)
resp, err := whataphttp.HttpPost(context.Background(), url, contentType, body)

// After (removed)
resp, err := http.Get(url)
resp, err := http.Post(url, contentType, body)
```

**DefaultClient Marker Function Restoration** (preserves original form):
```go
// Before (instrumented)
resp, err := whataphttp.DefaultClientGet(context.Background(), url)
resp, err := whataphttp.DefaultClientPost(context.Background(), url, contentType, body)

// After (removed - restored to original form)
resp, err := http.DefaultClient.Get(url)
resp, err := http.DefaultClient.Post(url, contentType, body)
```

---

## HTTP Client Transport Restoration

**With existing Transport**:
```go
// Before (instrumented)
client := &http.Client{
    Transport: whataphttp.NewRoundTrip(context.Background(), customTransport),
}

// After (removed)
client := &http.Client{
    Transport: customTransport,
}
```

**Empty Client Marker Function Restoration** (Transport field removed):
```go
// Before (instrumented)
client := &http.Client{
    Timeout:   10 * time.Second,
    Transport: whataphttp.NewRoundTripWithEmptyTransport(context.Background()),
}

// After (removed - Transport field removed)
client := &http.Client{
    Timeout: 10 * time.Second,
}
```

---

## Database Restoration

```go
// Before (instrumented)
db, err := whatapsql.Open("mysql", dsn)
db, err := whatapsqlx.Open("mysql", dsn)
db, err := whatapsqlx.Connect("mysql", dsn)
db, err := whatapgorm.Open("mysql", dsn)

// After (removed)
db, err := sql.Open("mysql", dsn)
db, err := sqlx.Open("mysql", dsn)
db, err := sqlx.Connect("mysql", dsn)
db, err := gorm.Open("mysql", dsn)
```

---

## Redigo Restoration

```go
// Before (instrumented)
conn, err := whatapredigo.Dial("tcp", "localhost:6379")
conn, err := whatapredigo.DialContext(ctx, "tcp", "localhost:6379")

// After (removed)
conn, err := redis.Dial("tcp", "localhost:6379")
conn, err := redis.DialContext(ctx, "tcp", "localhost:6379")
```

---

## Sarama Interceptor Removal

```go
// Removed
whatapInterceptor := &whatapsarama.Interceptor{}
config.Producer.Interceptors = []sarama.ProducerInterceptor{whatapInterceptor}
```

---

## gRPC Interceptor Removal

```go
// Before (instrumented)
s := grpc.NewServer(
    grpc.UnaryInterceptor(whatapgrpc.UnaryServerInterceptor()),
    grpc.StreamInterceptor(whatapgrpc.StreamServerInterceptor()),
)

// After (removed)
s := grpc.NewServer()
```

---

## Kubernetes config.Wrap Removal

```go
// Removed
config.Wrap(whatapkubernetes.WrapRoundTripper())
```

---

## MongoDB Restoration

```go
// Before (instrumented)
client, err := whatapmongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
client, err := whatapmongo.NewClient(options.Client().ApplyURI(mongoURI))

// After (removed)
client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
client, err := mongo.NewClient(options.Client().ApplyURI(mongoURI))
```

---

## Error Tracing Removal

```go
// Before (instrumented)
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    trace.Error(context.Background(), err)
    return
}

// After (removed)
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
```

---

## Unused Import Removal

After removing whatap-related code, imports that are no longer used are also automatically removed:

```go
// Removed (if unused)
import "context"
import "github.com/whatap/go-api/instrumentation/..."
```

---

## Restoration Test Results

| App | inject → remove | Notes |
|-----|-----------------|-------|
| gin, echo, fiber, chi, nethttp | 100% restored | |
| sql, sqlx, gorm, jinzhugorm | 100% restored | sqlx added (2025-12-16) |
| gorilla, fasthttp | 100% restored | |
| redigo, sarama | 100% restored | |
| grpc, k8s | 100% restored | |
| echo-old | 100% restored | |
| httpc | 100% restored | Perfect restoration with marker function approach (2025-12-15) |
| goredis, goredis-v8 | 100% restored | go-redis/redis support (2025-12-17) |
| mongodb | Format difference | Import sorting, blank lines (functionality identical) |

> **Total 20 apps restoration tested** (as of 2025-12-30)
> - 19 apps: 100% restored
> - mongodb-app: Format difference (import sorting order, blank lines)
