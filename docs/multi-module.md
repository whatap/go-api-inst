# Multi-Module Project Instrumentation

How to use whatap-go-inst with projects composed of multiple Go modules.

---

## How Packages Are Handled

| Package Type | Location | Instrumented? |
|--------------|----------|---------------|
| Go standard library | `$GOROOT/src/...` | No |
| External modules (go get) | `$GOMODCACHE/...` | No by default (opt-in via `--external-module`) |
| Your project code | Local path | **Yes** |
| replace local modules | Local path (`../`) | No (requires separate inject) |

---

## Approaches

### Approach 1: --external-module (Recommended)

> Requires v0.5.4+

Instruments external modules from GOMODCACHE automatically — copies, injects, and sets up replace directives in a single command.

```bash
# Single module
whatap-go-inst --external-module=mycompany/db-lib go build ./...

# Multiple modules (comma-separated)
whatap-go-inst --external-module=mycompany/db-lib,mycompany/web-lib go build ./...

# Wildcard pattern — all modules from an organization
whatap-go-inst --external-module="mycompany.com/internal/*" go build ./...
```

**How it works:**

1. Runs `go mod download -json` to locate modules in GOMODCACHE
2. Copies module to `<tempdir>/_modules/<module-name>/` with write permissions
3. Injects instrumentation code into the copy
4. Adds `replace module@version => ./_modules/...` to main go.mod
5. Adds `require github.com/whatap/go-api` to copy's go.mod
6. Builds

**Notes:**
- Original GOMODCACHE is never modified (works on copies)
- Modules must exist in GOMODCACHE — run `go mod download` first if needed
- Unsupported framework versions are automatically skipped ([Version Filtering](./rules/versions.md))

---

### Approach 2: Separate inject per Module

For modules not in GOMODCACHE (local directory modules with `replace` directives).

```bash
# 1. Inject each module
whatap-go-inst inject -s ../db-lib -o ../db-lib-instrumented
whatap-go-inst inject -s ../web-lib -o ../web-lib-instrumented
whatap-go-inst inject -s . -o ./instrumented

# 2. Set up replace directives (instrumented/go.mod)
#    replace mycompany/db-lib => ../db-lib-instrumented
#    replace mycompany/web-lib => ../web-lib-instrumented

# 3. Build
cd instrumented
go get github.com/whatap/go-api@latest
go build ./...
```

---

### Approach 3: Single Module (New Projects)

For new projects, a single-module structure is simplest — no multi-module setup needed.

```
myproject/
├── go.mod              # module mycompany/myproject
├── cmd/main/main.go
├── internal/
│   ├── db/connection.go
│   └── web/server.go
└── pkg/shared/utils.go
```

```bash
whatap-go-inst go build ./cmd/main
```

---

## Mode Comparison

| | wrap (default) | wrap + --external-module | inject |
|---|---|---|---|
| Main module | Instrumented | Instrumented | Instrumented |
| replace modules | Skipped | Skipped | Separate inject needed |
| External modules (GOMODCACHE) | Skipped | **Instrumented** | Separate inject needed |
| Builds binary | Yes | Yes | No (source only) |
| replace directives | — | Automatic | Manual |

---

## Important Notes

### replace modules are not instrumented

Build wrapper adjusts replace paths but does **not** instrument the target module itself.

```
# go.mod
replace mycompany/db-lib => ../db-lib

# wrap mode: path adjusted, but db-lib code is NOT instrumented
whatap-go-inst go build ./...
```

**Solution:** Use `--external-module` (if in GOMODCACHE) or separate inject.

### Libraries without main

Libraries can be injected — `trace.Init()` is only added in main packages.

| Pattern | Handling |
|---------|----------|
| `sql.Open()` → `whatapsql.Open()` | Transformed |
| `gin.Default()` + middleware | Added |
| `trace.Init()` / `trace.Shutdown()` | Not added (correct) |

### Dependency propagation

Modules that use instrumented libraries also need the whatap dependency:

```bash
cd main-app
go get github.com/whatap/go-api@latest
```

---

## Docker Example

```dockerfile
# Stage 1: Build with instrumentation
FROM golang:1.21-alpine AS builder

# Install whatap-go-inst
RUN wget -qO- https://github.com/whatap/go-api-inst/releases/latest/download/whatap-go-inst_linux_amd64.tar.gz | tar xz -C /usr/local/bin/

WORKDIR /app
COPY . .

# Build with external module instrumentation
RUN go mod download
RUN whatap-go-inst \
    --external-module="mycompany.com/*" \
    go build -o /app/server ./...

# Stage 2: Runtime
FROM alpine:latest

# Install WhaTap agent
RUN wget -qO- https://s3.ap-northeast-2.amazonaws.com/repo.whatap.io/alpine/x86_64/whatap-agent.tar.gz | tar xz -C /

WORKDIR /app
COPY --from=builder /app/server .

# WhaTap config
RUN echo "license=your-license-key" > whatap.conf && \
    echo "whatap.server.host=13.124.11.223" >> whatap.conf && \
    echo "app_name=myapp" >> whatap.conf

ENV WHATAP_HOME=/app

EXPOSE 8080
CMD ["/bin/sh", "-c", "/usr/whatap/agent/whatap-agent start && ./server"]
```

---

## Related Documents

- [Build Wrapper Mode](./build-wrapper.md)
- [Source Inject Mode](./source-inject.md)
- [Version Filtering](./rules/versions.md)
- [Config Settings](./config.md)
