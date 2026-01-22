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

> **Note**: Versionless echo is also supported (`github.com/labstack/echo`)

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

> **Note**: Versionless fiber is also supported (`github.com/gofiber/fiber`)

---

## github.com/go-chi/chi (including v5)

**Detection Pattern**: `chi.NewRouter()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi"
```

**Transformation Rule**:
```go
// Before
r := chi.NewRouter()

// After
r := chi.NewRouter()
r.Use(whatapchi.Middleware)  // Function value passed (no parentheses)
```

> **Note**: Chi passes `Middleware` (function value) not `Middleware()`.

---

## github.com/gorilla/mux

**Detection Pattern**: `mux.NewRouter()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/gorilla/mux/whatapmux"
```

**Transformation Rule**:
```go
// Before
r := mux.NewRouter()

// After
r := mux.NewRouter()
r.Use(whatapmux.Middleware)  // Function value passed (no parentheses)
```

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
mux.Handle("/", whataphttp.Handler(handler))
http.Handle("/api", whataphttp.Handler(apiHandler))
```

> **Note**: net/http doesn't have a middleware concept, so handler wrapping is used.

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

---

## Transformation Rules Summary

### Framework Middleware Insertion

| Package | Detection Pattern | Inserted Code | Method |
|---------|------------------|---------------|--------|
| `gin-gonic/gin` | `gin.Default()`, `gin.New()` | `r.Use(whatapgin.Middleware())` | Function call |
| `labstack/echo/v4` | `echo.New()` | `e.Use(whatapecho.Middleware())` | Function call |
| `gofiber/fiber/v2` | `fiber.New()` | `app.Use(whatapfiber.Middleware())` | Function call |
| `go-chi/chi` | `chi.NewRouter()` | `r.Use(whatapchi.Middleware)` | Function value |
| `gorilla/mux` | `mux.NewRouter()` | `r.Use(whatapmux.Middleware)` | Function value |
| `valyala/fasthttp` | `fasthttp.ListenAndServe()` | `whatapfasthttp.Middleware(handler)` | Wrapping |

### net/http Handler Wrapping (Server)

| Original Function | After Transformation |
|-------------------|---------------------|
| `HandleFunc(path, handler)` | `HandleFunc(path, whataphttp.Func(handler))` |
| `Handle(path, handler)` | `Handle(path, whataphttp.Handler(handler))` |

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
