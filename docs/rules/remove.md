# Remove Rules (Code Removal)

`whatap-go-inst remove` strips **manually written** `whatap`/`go-api` calls and imports from a source tree. The typical use case is migrating from a hand-rolled integration to the toolexec build wrapper: the wrapper rewrites code only inside `$WORK`, so auto-injected changes are never in your source tree to begin with — `remove` exists to clean up the manual remnants that *are*.

> `remove` is a deliberately scoped cleanup pass, **not** an "inverse of auto-inject". It reverts the manual instrumentation shapes that can be undone unambiguously (trace lifecycle, whitelisted wrappers, replaced constructors). Patterns that change a call signature or add a structural argument are **not** auto-reverted — see [What is NOT auto-reverted](#what-is-not-auto-reverted).

## What `remove` reverts

### 1. Trace lifecycle & error tracking

```go
// Removed (from main())
import "github.com/whatap/go-api/trace"
trace.Init(nil)
defer trace.Shutdown()
```

```go
// trace.Error inside an error block is removed
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    trace.Error(context.Background(), err)   // removed
    return
}
```

Standalone `trace.Step` / `trace.Println` / `trace.SetMTrace` statements and any `logsink.*` statements are removed as well.

### 2. Logger hook

```go
logger.AddHook(whataplogrus.NewHook(...))   // removed
```

### 3. Wrapper unwrap (whitelist — wrapper stripped, original argument restored)

Single-argument wrappers are unwrapped in place:

```go
whatapgin.WrapEngine(r)                  → r
whatapecho.WrapEngine(e)                 → e
whatapchi.WrapRouter(chi.NewRouter())    → chi.NewRouter()
whatapmux.WrapRouter(mux.NewRouter())    → mux.NewRouter()
whataphttp.WrapHandler(h)                → h
whataphttp.Func(h)                       → h
whatapsql.Open(...) / whatapdb.Open(...) → inner (wrapper form)
whataplogsink.GetTraceLogWriter(...)     → inner
```

Recognised functions: any `Wrap*` function, plus `whataphttp.{Func, WrapHandler, WrapHandlerFunc}`, `whatapsql`/`whatapdb`.`{Open, OpenDB}`, and `whataplogsink.GetTraceLogWriter`.

### 4. Replaced constructors restored (25 patterns)

Calls the SDK guide has you write as `whatap*.X(...)` are reverted to the original package call:

| Package | whatap form → original |
|---------|------------------------|
| `database/sql` | `whatapsql.Open` → `sql.Open` |
| `jmoiron/sqlx` | `whatapsqlx.{Open, Connect, ConnectContext, MustConnect, MustOpen}` → `sqlx.*` |
| `gorm` (gorm.io, jinzhu) | `whatapgorm.Open` → `gorm.Open` |
| `redis/go-redis/v9` | `whatapgoredis.{NewClient, NewClusterClient, NewFailoverClient, NewRing}` → `redis.*` |
| `go-redis/redis/v8` | `whatapgoredis.{NewClient, NewClusterClient, NewFailoverClient, NewRing}` → `redis.*` |
| `gomodule/redigo` | `whatapredigo.{Dial, DialContext, DialURL, DialURLContext}` → `redis.*` |
| `mongo-driver` | `whatapmongo.{Connect, NewClient}` → `mongo.*` |
| `fmt` | `whatapfmt.{Print, Printf, Println}` → `fmt.*` |

(v8 and v9 both alias to `whatapgoredis`; the file's actual import path disambiguates.)

### 5. Unused imports

After the steps above, any `whatap`/`go-api` imports that are no longer referenced are removed.

## What is NOT auto-reverted

`remove` deliberately skips patterns whose reversal is ambiguous or structural. If you applied any of these by hand, **remove them manually**. For the signature-changing cases `remove` prints a `WARN` pointing at the line.

| Pattern | Why it is skipped | Manual action |
|---------|-------------------|---------------|
| `whataphttp.HttpGet(ctx, url)` / `HttpPost` / `DefaultClientGet` | Changes the call signature (adds `ctx`) | Change back to `http.Get(url)` etc. |
| `whataphttp.NewRoundTrip(ctx, t)` / transport wrapping | Structural change to an `http.Client{Transport: ...}` field | Restore the original `Transport` by hand |
| `ctx, _ := trace.Start(ctx, "name")` | Left-hand `ctx` propagates into later calls | Remove and rewire `ctx` by hand |
| gin/echo/fiber `r.Use(whatapXxx.Middleware())`, fasthttp `whatapfasthttp.Middleware(h)` | `.Use()` / wrap statement, not a whitelisted single-arg wrapper | Delete the `.Use(...)` / wrap call by hand |
| gRPC `grpc.UnaryInterceptor(whatapgrpc.UnaryServerInterceptor())` (and `Stream` / `WithChainUnaryInterceptor` variants) | Interceptor argument inside `grpc.NewServer` / `grpc.Dial` | Delete the interceptor argument by hand |
| Sarama `config.Producer.Interceptors = []sarama.ProducerInterceptor{...}` | Struct field assignment | Delete the assignment by hand |
| Kubernetes `config.Wrap(whatapkubernetes.WrapRoundTripper())` | Structural call on the user's `*rest.Config` | Delete the call by hand |
