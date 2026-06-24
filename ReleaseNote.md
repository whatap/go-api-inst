# Go Auto-Instrumentation Release Notes

## Go auto-instrumentation v0.6.1

June 24, 2026

- [Fixed] `go install` of the CLI restored — `go install github.com/whatap/go-api-inst/cmd/whatap-go-inst@latest` and `go install github.com/whatap/go-api-inst/cmd/goinst@latest` work again. The `cmd/` entry points were unintentionally dropped in v0.6.0; the recommended tarball install (`whatap-go-inst_<os>_<arch>.tar.gz`) was unaffected. The `version` command now also reports the correct version for `go install` builds.

- [Change] Release pipeline pinned — the GitHub Actions release now pins GoReleaser to `~> v2` for reproducible builds (prevents an unattended major-version jump from breaking the release).

---

## Go auto-instrumentation v0.6.0

June 24, 2026

- [New] Custom `add` rules (fast mode) — Fast-mode builds now support `add` rules that create a new file in the target package before the build and remove it afterward, so your source tree is left unchanged (the created files are also saved under `whatap-instrumented/` for reproducible output, and existing files are never overwritten). The old `append: true` flag is removed; migrate appends to a new file in the same package:

    ```yaml
    # Before (v0.5.4 — rejected by the v0.6.0 loader)
    add:
      - package: "pkg/user"
        file: "user.go"
        append: true
        content: |
          func GetUserWithTrace(id int) (*User, error) { ... }

    # After (v0.6.0)
    add:
      - package: "pkg/user"
        file: "whatap_user_ext.go"   # must be a new filename
        content: |
          package user                 # full Go file — declare package + imports
          import "fmt"
          func GetUserWithTrace(id int) (*User, error) { ... }
    ```

    Avoid filenames matching the default exclude patterns (`**/*_generated.go`, `**/*_gen.go`, `**/*_test.go`) — recommended convention: `whatap_<name>_ext.go`. See `docs/custom-instrumentation.md` §11 for the full migration guide.

- [Feature] vendor and external-module builds (fast mode) — `vendor/` projects are auto-detected (the build runs `go mod vendor` when needed), and `--external-module` instruments modules from the module cache.

    ```bash
    whatap-go-inst --external-module github.com/company/lib go build ./...
    ```

- [Change] Build workflow simplified — `whatap-go-inst go build ./...` is the single build command. The `inject` / `generate` / `init` / `uninit` subcommands and the `--wrap` / `--no-output` flags were removed.

    - **The build command does not change** — `whatap-go-inst go build` has been the build command since the first release and is independent of `init`, so most users are unaffected.
    - Only the legacy `init`-based or `go:generate` workflow needs to switch to `whatap-go-inst go build ./...` (and remove any leftover `whatap_inst.tool.go` / `whatap_inst_generate.go`).
    - Add `--output=DIR` to dump the transformed source while building. `--fast` is kept as a hidden no-op for old scripts.

- [Change] `whatap-go-inst remove` redefined — `remove` now has a single purpose: strip whatap/go-api calls and imports that you wrote **by hand**, typically when migrating from a manual integration to the build wrapper. It also restores wrapped calls to their originals (e.g. `whatapsql.Open(...)` → `sql.Open(...)`) across the common SQL / sqlx / GORM / Redis / MongoDB / fmt patterns and re-adds the original imports.

    - Build-wrapper (auto-instrumented) users are unaffected — the build wrapper never changes your source tree, so there is nothing to undo.
    - The `--all` flag is now a deprecated no-op.

    ```bash
    whatap-go-inst remove --src . --output ./cleaned
    ```

- [Change] `fmt` instrumentation is opt-in by default — `fmt.Print` / `fmt.Printf` / `fmt.Println` instrumentation is disabled by default. Enable it with `enabled_packages: [fmt]`; on high-frequency log apps (Loki, Promtail) this may add noticeable overhead. Ordinary web/API apps are not affected.

- [Fixed] Auto-instrumentation stability — fixed cases where auto-instrumentation could break the build or runtime: a startup panic when the application already configured a gRPC interceptor (now uses additive interceptor chaining, so it coexists with yours), a build error inside certain nested code blocks, and duplicate transaction creation in some paths.

- [Fixed] Build reliability — fixed transitive-dependency conflicts during dependency resolution, a fast-mode build failure when `go.sum` was missing, and a case where `log/slog` was misidentified as the `log` package.

- [New] LLM SDK auto-instrumentation — the build wrapper now auto-instruments four major Go LLM SDKs (no manual wrap calls). LLM external calls, token usage, and streaming latency appear on the WhaTap dashboard.

    - `sashabaranov/go-openai`
    - `cloudwego/eino-ext` (openai / claude) — both the constructor and the compose pipeline (`AppendChatModel`, `Generate` / `Stream`) are covered
    - `anthropics/anthropic-sdk-go`
    - `openai/openai-go` (official SDK)

    Enable with `llm_enabled=true` in `whatap.conf`. The build wrapper detects these SDKs in your `go.mod` and adds the LLM nested module automatically — no action needed. If you import the adapters directly, fetch the nested module:

    ```bash
    go get github.com/whatap/go-api/instrumentation/llm@v0.6.0
    ```

    The LLM adapters live in a nested module so the main `go-api` module stays on `go 1.18`. LLM monitoring is published as a separate release track — see [`llm-agent/ReleaseNote.md`](./llm-agent/ReleaseNote.md).

---

## Go auto-instrumentation v0.5.4

February 25, 2026

- [Feature] Support selective instrumentation of external modules (GOMODCACHE).

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

- [Feature] Auto-detect `http.Server{Handler: ...}` struct literal pattern.

    ```go
    // Automatic transformation
    s := &http.Server{Handler: mux}
    // → s := &http.Server{Handler: whataphttp.WrapHandler(mux)}
    ```

- [Feature] Support `http.Handle()` transformation.

    ```go
    // Automatic transformation
    http.Handle("/api", handler)
    // → http.Handle("/api", whataphttp.WrapHandler(handler))
    ```

- [Feature] Auto-detect `fasthttp.Server{Handler: ...}` struct literal pattern.

    ```go
    // Automatic transformation
    s := &fasthttp.Server{Handler: myHandler}
    // → s := &fasthttp.Server{Handler: whatapfasthttp.WrapHandler(myHandler)}
    ```

- [Fixed] Fix anonymous function (FuncLit) code not being transformed.

    **Problem**
    - Framework code inside anonymous functions like Cobra Command's `Run: func() {...}` was not being transformed

    **Resolution**
    - Traverse all nodes with `dst.Inspect` to handle both `FuncDecl` and `FuncLit`
    - Correctly instruments Cobra-based CLI apps like alist and 1Panel

- [Fixed] Fix bug where only imports are added to empty main() functions.

    **Problem**
    - Empty `func main() {}` files had trace imports added, causing "imported and not used" compile error

    **Resolution**
    - Detects empty main functions with `FindNonEmptyMainFunc()` helper and skips them

- [Fixed] Fix logger `.New()` instance instrumentation not being applied.

    **Problem**
    - Logger instances created with `logrus.New()`, `log.New()`, etc. were not instrumented

    **Resolution**
    - `logrus.New()` → `whataplogrus.WrapLogger(logrus.New())` automatic transformation
    - `log.New(w, prefix, flag)` → `log.New(logsink.GetTraceLogWriter(w), prefix, flag)` automatic transformation

- [Fixed] Fix `framework.New()` pattern not detected inside struct field initialization.

    **Problem**
    - `fiber.New()`, `gin.New()`, etc. inside struct literals were not detected

    **Resolution**
    - Changed 7 transformers to use `dst.Inspect` + in-place CallExpr wrapping
    - `svc := &Service{App: fiber.New()}` → `svc := &Service{App: whatapfiber.WrapApp(fiber.New())}` automatic transformation

- [Change] Change logrus instrumentation pattern to Hook-based approach.

    **Before**: `logrus.SetOutput(logsink.GetTraceLogWriter(os.Stderr))`
    **After**: `import _ "whataplogrus"` (blank import → Hook auto-registered in init())

    No conflicts with the app's logrus output settings.

---

## Go auto-instrumentation v0.5.3

February 5, 2026

- [Fixed] Fix Aerospike v6 Close() method wrapping bug.

    Fixed compile error when wrapping `Close()` method in Aerospike v6 where it returns void.

- [Fixed] Fix Sarama NewConfig() duplicate declaration bug.

    Fixed `whatapInterceptor` duplicate declaration error when calling `sarama.NewConfig()` more than once in the same function.

- [Change] Add extensive test patterns.

    Added a total of 410 test cases covering various usage patterns for all modules.

---

## Go auto-instrumentation v0.5.2

February 4, 2026

- [Fixed] Fix bug where -tags flag value is converted to build target in wrap mode.

    **Problem**
    - Running `whatap-go-inst go build -tags sqlite .` converted `sqlite` to `./sqlite`

    **Resolution**
    - Registered go flags that take values (`-tags`, `-ldflags`, `-gcflags`, `-o`, etc.) to skip conversion

- [Fixed] Fix version info auto-detection for go install.

    Uses `runtime/debug.ReadBuildInfo()` to automatically detect module version and VCS information.

---

## Go auto-instrumentation v0.5.1

January 25, 2026

- [Fixed] Fix go-redis v7 false detection bug.

    Fixed type mismatch error caused by whatapgoredis transformation being attempted on projects using go-redis v7. Only v8/v9 are supported.

- [Feature] Support package-level http.Client and client.Do() patterns.

    Supports package-level `var httpClient = &http.Client{}` transformation and `httpClient.Do(req)` pattern distributed tracing connection.

---

## Go auto-instrumentation v0.5.0

January 22, 2026

### Initial Release

Go AST-based automatic instrumentation — adds WhaTap monitoring to your Go application at build time, with no source code changes.

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
