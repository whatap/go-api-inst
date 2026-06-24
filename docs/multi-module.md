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

### Approach 2: Dump each module's transformed source (`--output`)

If you need a self-contained, pre-instrumented source tree per module (e.g. for air-gapped builds or CI artifacts), run `whatap-go-inst --output=…` inside each module directory. Fast mode will compile the module while dumping the transformed source to the given path.

```bash
# 1. Transform each module (fast mode + --output)
cd ../db-lib && whatap-go-inst --output=./instrumented go build ./...
cd ../web-lib && whatap-go-inst --output=./instrumented go build ./...
cd ./myapp && whatap-go-inst --output=./instrumented go build ./...

# 2. The output directories are buildable on their own
cd ./myapp/instrumented
go build ./...
```

> Legacy `whatap-go-inst inject -s ... -o ...` CLI was removed in v0.6.0. The fast-mode `--output` flag above replaces it — see [Inspect the Transformed Source](./source-inject.md).

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

## Approach Comparison

| | fast (default) | fast + `--external-module` | fast + `--output` per module |
|---|---|---|---|
| Main module | Instrumented | Instrumented | Instrumented |
| replace modules | Skipped | Skipped | Transform each separately |
| External modules (GOMODCACHE) | Skipped | **Instrumented** | Transform each separately |
| Builds binary | Yes | Yes | Yes (one binary per module command) |
| replace directives | — | Automatic | Manual (one per output tree) |

---

## Dependency Resolution Mode Support

Go manages external libraries in multiple ways. whatap-go-inst behavior varies depending on the mode.

### Support Matrix

| Mode | Description | Status |
|------|-------------|--------|
| **GOMODCACHE** (default) | Installed via `go get`, cached in `$GOPATH/pkg/mod` | Supported |
| **vendor** (`go mod vendor`) | Dependencies copied to `vendor/` directory | Supported (auto-detected, `go mod vendor` synced) |
| **go.work** (workspace) | Multiple modules in a single workspace | Planned |
| **Local replace** (`=> ../path`) | go.mod redirects to local path | Supported (path auto-adjusted) |
| **GOPROXY=off** (air-gapped) | No network access | Not supported (network required) |
| **GOPATH mode** (legacy) | `GO111MODULE=off`, no go.mod | Not supported |
| **Bazel/Nix** (external build) | Go build system not used | Not supported (incompatible) |

### vendor Projects

Projects using `vendor/` directory are automatically detected and supported.

```bash
# vendor projects work out of the box
whatap-go-inst go build ./...
whatap-go-inst --external-module github.com/gin-gonic/gin go build ./...
```

#### Vendor Detection

Uses the same auto-vendor logic as Go commands:
- Checks `vendor/modules.txt` existence
- Validates `go.mod` has `go >= 1.14` directive
- Respects GOFLAGS/CLI `-mod=` overrides

#### How vendor projects are handled (fast mode, default)

Fast mode adds whatap imports **at compile time**, so whatap packages must be in `vendor/` before the build starts.

**Approach** (prepare `vendor/` before the build):

```
1. go get github.com/whatap/go-api       — add to go.mod
2. Create tool file (//go:build tools)   — prevents tidy from pruning go-api
3. go mod tidy                           — unify transitive dependency versions
4. go mod vendor                         — include whatap packages in vendor/
5. Delete tool file
6. Locate packages (vendor mode)         — find the compiled whatap packages in vendor/
7. Build (vendor mode)                   — adds imports and instrumentation at compile time
8. Rollback (go.mod, go.sum, vendor/)    — restore project to original state
```

**Why `//go:build tools` tag?**
- Excluded from `go build` (no build errors)
- Recognized by `go mod tidy` (prevents go-api from being pruned)
- Makes `go mod vendor` copy whatap packages into vendor/

**Why vendor mode throughout:**
- Both the package lookup and the build run in vendor mode, using the same `vendor/` packages
- Using the same mode for both keeps the package builds consistent, so the linker does not reject them

**New standard-library imports:**
- Instrumentation may add new standard-library imports (`os`, `context`, etc.) that were not in the original source
- The tool also locates common standard-library packages (`os`, `context`, `fmt`, `log`, `net/http`)
- All newly added packages (not just whatap) are made available to the linker

#### --external-module with vendor

With `--external-module`, packages inside vendor/ can also be instrumented:

```bash
# Instruments mux.NewRouter() inside vendor/github.com/grafana/dskit/
whatap-go-inst --external-module github.com/grafana/dskit go build ./...
```

**Real-world examples:**
- **Grafana Loki** (28k stars): gorilla/mux created inside dskit library → `--external-module github.com/grafana/dskit` enables HTTP+gRPC transaction collection
- **OpenFaaS** (26k stars): vendor project, both wrap and fast mode WA-TX collection successful

### go.work Workspaces

Multi-module workspaces using `go.work` are not yet supported.

```
workspace/
├── go.work          # use ./service-a; use ./service-b
├── service-a/
│   └── go.mod
└── service-b/
    └── go.mod
```

**Notes:**
- `go.work` `replace` directives **override** individual `go.mod` `replace` directives
- `replace` directives added by `--external-module` may be ignored
- `go.work` is typically `.gitignore`'d — local and CI behavior may differ

**Current workaround:**
```bash
# Ignore go.work and build
GOWORK=off whatap-go-inst go build ./...
```

### Air-gapped / Offline Environments

When `GOPROXY=off` or network is blocked, `go get github.com/whatap/go-api` will fail.

**Workarounds:**
1. Run `go mod download` in a network-accessible environment first
2. Copy GOMODCACHE to the air-gapped environment
3. In Docker, run whatap-go-inst in a build stage with network access

---

## Important Notes

### replace modules are not instrumented

Build wrapper adjusts replace paths but does **not** instrument the target module itself.

```
# go.mod
replace mycompany/db-lib => ../db-lib

# build wrapper: path adjusted, but db-lib code is NOT instrumented
whatap-go-inst go build ./...
```

**Solution:** Use `--external-module` to instrument the replaced module.

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
- [Inspect Transformed Source (`--output`)](./source-inject.md)
- [Version Filtering](./rules/versions.md)
- [Config Settings](./config.md)
