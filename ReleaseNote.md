# Go Auto-Instrumentation Release Notes

## Go auto-instrumentation v0.5.4

February 25, 2026

- [Feature] Support selective instrumentation of external modules (GOMODCACHE). (#138)

    Automatically instrument internal modules downloaded via `go get`.

    ```bash
    # CLI option
    whatap-go-inst --external-module=mycompany.com/internal/lib go build ./...
    ```

    ```yaml
    # Configuration file
    external_modules:
      - "mycompany.com/internal/lib"
    ```

    **How it works**
    - Copies module from GOMODCACHE → injects instrumentation code → automatically adds replace directive
    - Does not modify the original GOMODCACHE
    - Supports both wrap and inject modes

- [Feature] Auto-detect `http.Server{Handler: ...}` struct literal pattern. (#129)

    ```go
    // Automatic transformation
    s := &http.Server{Handler: mux}
    // → s := &http.Server{Handler: whataphttp.WrapHandler(mux)}
    ```

- [Feature] Support `http.Handle()` transformation. (#56)

    ```go
    // Automatic transformation
    http.Handle("/api", handler)
    // → http.Handle("/api", whataphttp.WrapHandler(handler))
    ```

- [Feature] Auto-detect `fasthttp.Server{Handler: ...}` struct literal pattern. (#130)

    ```go
    // Automatic transformation
    s := &fasthttp.Server{Handler: myHandler}
    // → s := &fasthttp.Server{Handler: whatapfasthttp.WrapHandler(myHandler)}
    ```

- [Fixed] Fix anonymous function (FuncLit) code not being transformed. (#117)

    **Problem**
    - Framework code inside anonymous functions like Cobra Command's `Run: func() {...}` was not being transformed

    **Resolution**
    - Traverse all nodes with `dst.Inspect` to handle both `FuncDecl` and `FuncLit`
    - Correctly instruments Cobra-based CLI apps like alist and 1Panel

- [Fixed] Fix bug where only imports are added to empty main() functions. (#125)

    **Problem**
    - Empty `func main() {}` files had trace imports added, causing "imported and not used" compile error

    **Resolution**
    - Detects empty main functions with `FindNonEmptyMainFunc()` helper and skips them

- [Fixed] Fix logger `.New()` instance instrumentation not being applied. (#134)

    **Problem**
    - Logger instances created with `logrus.New()`, `log.New()`, etc. were not instrumented

    **Resolution**
    - `logrus.New()` → `whataplogrus.WrapLogger(logrus.New())` automatic transformation
    - `log.New(w, prefix, flag)` → `log.New(logsink.GetTraceLogWriter(w), prefix, flag)` automatic transformation

- [Fixed] Fix `framework.New()` pattern not detected inside struct field initialization. (#137)

    **Problem**
    - `fiber.New()`, `gin.New()`, etc. inside struct literals were not detected

    **Resolution**
    - Changed 7 transformers to use `dst.Inspect` + in-place CallExpr wrapping
    - `svc := &Service{App: fiber.New()}` → `svc := &Service{App: whatapfiber.WrapApp(fiber.New())}` automatic transformation

- [Change] Change logrus instrumentation pattern to Hook-based approach. (#121)

    **Before**: `logrus.SetOutput(logsink.GetTraceLogWriter(os.Stderr))`
    **After**: `import _ "whataplogrus"` (blank import → Hook auto-registered in init())

    No conflicts with the app's logrus output settings.

---

## Go auto-instrumentation v0.5.3

February 5, 2026

- [Fixed] Fix Aerospike v6 Close() method wrapping bug. (#115)

    Fixed compile error when wrapping `Close()` method in Aerospike v6 where it returns void.

- [Fixed] Fix Sarama NewConfig() duplicate declaration bug. (#116)

    Fixed `whatapInterceptor` duplicate declaration error when calling `sarama.NewConfig()` more than once in the same function.

- [Change] Add extensive test patterns. (#114)

    Added a total of 410 test cases covering various usage patterns for all modules.

---

## Go auto-instrumentation v0.5.2

February 4, 2026

- [Fixed] Fix bug where -tags flag value is converted to build target in wrap mode. (#93)

    **Problem**
    - Running `whatap-go-inst go build -tags sqlite .` converted `sqlite` to `./sqlite`

    **Resolution**
    - Registered go flags that take values (`-tags`, `-ldflags`, `-gcflags`, `-o`, etc.) to skip conversion

- [Fixed] Fix version info auto-detection for go install. (#82)

    Uses `runtime/debug.ReadBuildInfo()` to automatically detect module version and VCS information.

---

## Go auto-instrumentation v0.5.1

January 25, 2026

- [Fixed] Fix go-redis v7 false detection bug. (#78)

    Fixed type mismatch error caused by whatapgoredis transformation being attempted on projects using go-redis v7. Only v8/v9 are supported.

- [Feature] Support package-level http.Client and client.Do() patterns. (#80)

    Supports package-level `var httpClient = &http.Client{}` transformation and `httpClient.Do(req)` pattern distributed tracing connection.

---

## Go auto-instrumentation v0.5.0

January 22, 2026

### Initial Release

Go AST-based source code auto-instrumentation CLI tool.

- [Feature] Build Wrapper mode (recommended)

    ```bash
    whatap-go-inst go build ./...
    ```

    Automatically injects monitoring code and builds without modifying original source.

- [Feature] inject/remove commands

    ```bash
    whatap-go-inst inject -s . -o ./output    # Inject instrumentation code
    whatap-go-inst remove -s . -o ./clean     # Remove instrumentation code (restore original)
    ```

- [Feature] Auto-instrumentation for 22 frameworks/libraries

    | Category | Supported |
    |----------|-----------|
    | Web (7) | Gin, Echo, Fiber, Chi, Gorilla Mux, net/http, FastHTTP |
    | Database (4) | database/sql, sqlx, GORM, jinzhu/gorm |
    | NoSQL/Cache (5) | MongoDB, go-redis v8/v9, Redigo, Aerospike |
    | External (3) | Sarama (Kafka), gRPC, Kubernetes |
    | Logging (4) | fmt, log, logrus, zap |

- [Feature] Config file support (`.whatap/config.yaml`)

    Supports presets, per-package enable/disable, and custom rule configuration.

- [Feature] Perfect inject → remove round-trip

    Guarantees 100% identical code to the original after instrumentation removal.
