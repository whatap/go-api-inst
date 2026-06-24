# Web Framework Transformation Rules

## github.com/gin-gonic/gin

**Detection Pattern**: `gin.Default()`, `gin.New()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
```

**Transformation Rule**:
```go
// Before
r := gin.Default()

// After
r := gin.Default()
r.Use(whatapgin.Middleware())
```

**Anonymous Function Support** (Cobra pattern):
```go
// Before - Cobra Command with gin
var serverCmd = &cobra.Command{
    Run: func(cmd *cobra.Command, args []string) {
        r := gin.New()
        r.Run(":8080")
    },
}

// After - Middleware inserted in anonymous function
var serverCmd = &cobra.Command{
    Run: func(cmd *cobra.Command, args []string) {
        r := gin.New()
        r.Use(whatapgin.Middleware())  // ✅ Inserted
        r.Run(":8080")
    },
}
```

> **Note**: All frameworks support anonymous function transformation. See [Common Rules](./common.md#transformation-scope).

### Wrap Function (WrapEngine)

For struct field initialization and return statements where middleware insertion is not possible, `WrapEngine` wraps the engine instance in-place:

```go
// Before
svc := &Service{
    Engine: gin.New(),
}

// After
svc := &Service{
    Engine: whatapgin.WrapEngine(gin.New()),
}
```

**Signature**: `whatapgin.WrapEngine(*gin.Engine) *gin.Engine`

> **Note**: `WrapEngine` internally calls `engine.Use(Middleware())` and returns the engine.

---

## github.com/labstack/echo/v4

**Detection Pattern**: `echo.New()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/labstack/echo/v4/whatapecho"
```

**Transformation Rule**:
```go
// Before
e := echo.New()

// After
e := echo.New()
e.Use(whatapecho.Middleware())
```

> **Note**: Both `github.com/labstack/echo` (v3) and `echo/v4` are supported. v5+ is not supported and will be skipped.

### Wrap Function (WrapEcho)

For struct field initialization and return statements:

```go
// Before
svc := &Service{
    Echo: echo.New(),
}

// After
svc := &Service{
    Echo: whatapecho.WrapEcho(echo.New()),
}
```

**Signature**: `whatapecho.WrapEcho(*echo.Echo) *echo.Echo`

---

## github.com/gofiber/fiber/v2

**Detection Pattern**: `fiber.New()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/gofiber/fiber/v2/whatapfiber"
```

**Transformation Rule**:
```go
// Before
app := fiber.New()

// After
app := fiber.New()
app.Use(whatapfiber.Middleware())
```

> **Note**: Only fiber/v2 is supported. fiber v1 (`github.com/gofiber/fiber` without version) and v3+ are not supported and will be skipped.

### Wrap Function (WrapApp)

For struct field initialization and return statements:

```go
// Before
svc := &Service{
    App: fiber.New(),
}

// After
svc := &Service{
    App: whatapfiber.WrapApp(fiber.New()),
}
```

**Signature**: `whatapfiber.WrapApp(*fiber.App) *fiber.App`

---

## github.com/go-chi/chi (v4, v5)

**Detection Pattern**: `chi.NewRouter()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi"
```

**Transformation Rule**:
```go
// Before
r := chi.NewRouter()

// After (in-place wrapping)
r := whatapchi.WrapRouter(chi.NewRouter())
```

> **Note**: Both `github.com/go-chi/chi` (v4) and `chi/v5` are supported. v6+ is not supported and will be skipped.
> Uses in-place wrapping (not middleware insertion). `WrapRouter` internally calls `r.Use(Middleware)` and returns the router.

**Signature**: `whatapchi.WrapRouter[T any](r T) T`

---

## github.com/gorilla/mux

**Detection Pattern**: `mux.NewRouter()`, `.Subrouter()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/gorilla/mux/whatapmux"
```

**Transformation Rules**:
```go
// Before
r := mux.NewRouter()

// After (in-place wrapping)
r := whatapmux.WrapRouter(mux.NewRouter())
```

### Subrouter

gorilla/mux `Subrouter()` does **not** inherit parent router's `Use()` middleware. The tool automatically wraps Subrouter calls:

```go
// Before
api := r.PathPrefix("/api").Subrouter()

// After
api := whatapmux.WrapRouter(r.PathPrefix("/api").Subrouter())
```

> **Note**: Uses in-place wrapping (not middleware insertion). `WrapRouter` internally calls `r.Use(Middleware())` and returns the router. When both `NewRouter()` and `Subrouter()` are wrapped, duplicate transactions are automatically skipped.

**Signature**: `whatapmux.WrapRouter(*mux.Router) *mux.Router`

---

## net/http (Server)

**Detection Pattern**: `HandleFunc()`, `Handle()` calls

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
```

**Transformation Rule - HandleFunc**:
```go
// Before
mux.HandleFunc("/", handleRoot)
http.HandleFunc("/api", apiHandler)

// After
mux.HandleFunc("/", whataphttp.Func(handleRoot))
http.HandleFunc("/api", whataphttp.Func(apiHandler))
```

**Transformation Rule - Handle**:
```go
// Before
mux.Handle("/", handler)
http.Handle("/api", apiHandler)

// After
mux.Handle("/", whataphttp.WrapHandler(handler))
http.Handle("/api", whataphttp.WrapHandler(apiHandler))
```

> **Note**: net/http doesn't have a middleware concept, so handler wrapping is used.

### Wrap Function (WrapHandler) — Struct Literal

For `http.Server{Handler: ...}` struct literal patterns:

```go
// Before
s := &http.Server{
    Addr:    ":8080",
    Handler: mux,
}

// After
s := &http.Server{
    Addr:    ":8080",
    Handler: whataphttp.WrapHandler(mux),
}
```

**Signature**: `whataphttp.WrapHandler(http.Handler) http.Handler`

---

## net/http (Client)

**Detection Pattern**: `http.Get()`, `http.Post()`, `http.PostForm()`, `http.DefaultClient.Get()`, `http.Client{}`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
import "context"  // Added only when handler context detection fails
```

### Handler Context Auto-Detection

When HTTP client calls are inside web handler functions, the handler's context is automatically detected and used.
This enables proper distributed tracing (mtid) propagation.

**Supported Frameworks and Parameters**:

| Type | Signature | Extracted context |
|------|-----------|------------------|
| context.Context | `func(ctx context.Context, ...)` | `ctx` |
| net/http | `func(w http.ResponseWriter, r *http.Request)` | `r.Context()` |
| Gin | `func(c *gin.Context)` | `c.Request.Context()` |
| Echo | `func(c echo.Context)` | `c.Request().Context()` |
| Fiber | `func(c *fiber.Ctx)` | `c.UserContext()` |
| FastHTTP | `func(ctx *fasthttp.RequestCtx)` | `ctx` |

> **Priority**: `context.Context` parameter is detected first.
> This ensures context is correctly passed even in helper functions, not just handlers.

**Transformation Rule - http.Get/Post/PostForm**:
```go
// Before (inside Gin handler)
r.GET("/call-api", func(c *gin.Context) {
    resp, err := http.Get("http://example.com/api")
})

// After (handler context auto-detected - uses c.Request.Context())
r.GET("/call-api", func(c *gin.Context) {
    resp, err := whataphttp.HttpGet(c.Request.Context(), "http://example.com/api")
})
```

```go
// Before (inside net/http handler)
func handler(w http.ResponseWriter, r *http.Request) {
    resp, err := http.Get("http://example.com/api")
}

// After (uses r.Context())
func handler(w http.ResponseWriter, r *http.Request) {
    resp, err := whataphttp.HttpGet(r.Context(), "http://example.com/api")
}
```

```go
// Before (context.Context parameter)
func fetchData(ctx context.Context, url string) {
    resp, err := http.Get(url)
}

// After (uses ctx directly)
func fetchData(ctx context.Context, url string) {
    resp, err := whataphttp.HttpGet(ctx, url)
}
```

```go
// Before (outside handler - no context detected)
func noContextFunc() {
    resp, err := http.Get("http://example.com/api")
}

// After (fallback: uses context.Background())
func noContextFunc() {
    resp, err := whataphttp.HttpGet(context.Background(), "http://example.com/api")
}
```

**Transformation Rule - http.DefaultClient.Get/Post/PostForm** (marker functions):
```go
// Before
resp, err := http.DefaultClient.Get("http://example.com/api")
resp, err := http.DefaultClient.Post("http://example.com/api", "application/json", body)
resp, err := http.DefaultClient.PostForm("http://example.com/api", values)

// After (marker functions - restored to original form on remove, handler context auto-detected)
resp, err := whataphttp.DefaultClientGet(c.Request.Context(), "http://example.com/api")
resp, err := whataphttp.DefaultClientPost(r.Context(), "http://example.com/api", "application/json", body)
resp, err := whataphttp.DefaultClientPostForm(context.Background(), "http://example.com/api", values)
```

**Transformation Rule - Empty http.Client{} initialization** (marker functions):
```go
// Before
client := &http.Client{
    Timeout: 10 * time.Second,
}

// After (marker function - Transport field removed on remove, handler context auto-detected)
client := &http.Client{
    Timeout:   10 * time.Second,
    Transport: whataphttp.NewRoundTripWithEmptyTransport(c.Request.Context()),
}
```

```go
// Before (with Transport specified)
client := &http.Client{
    Transport: customTransport,
}

// After
client := &http.Client{
    Transport: whataphttp.NewRoundTrip(r.Context(), customTransport),
}
```

> **Note**: HTTP client calls are recorded as substeps of the current transaction, enabling external call tracking.
> Distributed tracing (mtid) is also properly propagated through handler context detection.

---

## github.com/valyala/fasthttp

**Detection Pattern**: `fasthttp.ListenAndServe()`, `fasthttp.Server{}`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp"
```

**Transformation Rule**:
```go
// Before
fasthttp.ListenAndServe(":8080", handler)

// After
fasthttp.ListenAndServe(":8080", whatapfasthttp.Middleware(handler))
```

### Wrap Function (WrapHandler) — Struct Literal

For `fasthttp.Server{Handler: ...}` struct literal patterns:

```go
// Before
s := &fasthttp.Server{
    Handler: myHandler,
}

// After
s := &fasthttp.Server{
    Handler: whatapfasthttp.WrapHandler(myHandler),
}
```

**Signature**: `whatapfasthttp.WrapHandler(fasthttp.RequestHandler) fasthttp.RequestHandler`

---

## Transformation Rules Summary

### Framework Middleware Insertion

| Package | Detection Pattern | Inserted Code | Method | Wrap Function |
|---------|------------------|---------------|--------|---------------|
| `gin-gonic/gin` | `gin.Default()`, `gin.New()` | `r.Use(whatapgin.Middleware())` | Function call | `WrapEngine()` |
| `labstack/echo/v4` | `echo.New()` | `e.Use(whatapecho.Middleware())` | Function call | `WrapEcho()` |
| `gofiber/fiber/v2` | `fiber.New()` | `app.Use(whatapfiber.Middleware())` | Function call | `WrapApp()` |
| `go-chi/chi` | `chi.NewRouter()` | `whatapchi.WrapRouter(chi.NewRouter())` | In-place wrap | `WrapRouter()` |
| `gorilla/mux` | `mux.NewRouter()`, `.Subrouter()` | `whatapmux.WrapRouter(...)` | In-place wrap | `WrapRouter()` |
| `net/http` | `http.Server{Handler}` | `whataphttp.WrapHandler(handler)` | Struct literal | `WrapHandler()` |
| `valyala/fasthttp` | `fasthttp.Server{Handler}` | `whatapfasthttp.WrapHandler(handler)` | Struct literal | `WrapHandler()` |

### net/http Handler Wrapping (Server)

| Original Function | After Transformation |
|-------------------|---------------------|
| `HandleFunc(path, handler)` | `HandleFunc(path, whataphttp.Func(handler))` |
| `Handle(path, handler)` | `Handle(path, whataphttp.WrapHandler(handler))` |

### net/http Client Wrapping

> **Note**: The context argument is determined through handler context auto-detection.
> Inside handlers, `r.Context()`, `c.Request.Context()`, etc. are used,
> while outside handlers, `context.Background()` is used as fallback.

| Original Code | After Transformation |
|---------------|---------------------|
| `http.Get(url)` | `whataphttp.HttpGet(ctx, url)` |
| `http.Post(url, contentType, body)` | `whataphttp.HttpPost(ctx, url, contentType, body)` |
| `http.PostForm(url, values)` | `whataphttp.HttpPostForm(ctx, url, values)` |
| `http.DefaultClient.Get(url)` | `whataphttp.DefaultClientGet(ctx, url)` |
| `http.DefaultClient.Post(...)` | `whataphttp.DefaultClientPost(ctx, ...)` |
| `&http.Client{}` (empty Client) | `&http.Client{Transport: whataphttp.NewRoundTripWithEmptyTransport(ctx)}` |
| `&http.Client{Transport: t}` | `&http.Client{Transport: whataphttp.NewRoundTrip(ctx, t)}` |

### Whatap Import Paths

| Original Package | Whatap Instrumentation Import |
|-----------------|------------------------------|
| `github.com/gin-gonic/gin` | `.../gin-gonic/gin/whatapgin` |
| `github.com/labstack/echo/v4` | `.../labstack/echo/v4/whatapecho` |
| `github.com/gofiber/fiber/v2` | `.../gofiber/fiber/v2/whatapfiber` |
| `github.com/go-chi/chi` | `.../go-chi/chi/whatapchi` |
| `github.com/gorilla/mux` | `.../gorilla/mux/whatapmux` |
| `github.com/valyala/fasthttp` | `.../valyala/fasthttp/whatapfasthttp` |
| `net/http` | `.../net/http/whataphttp` |

> **Note**: All paths are prefixed with `github.com/whatap/go-api/instrumentation/`
