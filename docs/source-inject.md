# Inspect the Transformed Source (`--output`)

> **v0.6.0 notice**: the legacy `whatap-go-inst inject` / `whatap-go-inst generate` CLI subcommands were **removed**. The same "look at the transformed source" workflow is now done with `whatap-go-inst --output` during a normal `go build`. This page documents the replacement. (`whatap-go-inst remove` is still available for manually stripping monitoring code; it is unrelated to the build-wrapper flow because the originals are never modified.)

## Overview

Use `--output` (or the `GO_API_AST_OUTPUT_DIR` environment variable) to dump the instrumented source tree while `go build` runs. The original source is not modified.

```bash
# Dump transformed source to whatap-instrumented/ (default)
whatap-go-inst --output go build ./...

# Dump to a custom directory
whatap-go-inst --output=./instrumented go build ./...

# Via environment variable
GO_API_AST_OUTPUT_DIR=./instrumented whatap-go-inst go build ./...
```

---

## What gets saved

The output directory is a **complete, buildable Go project**:

- Transformed `.go` files (project sources + any `--external-module` targets)
- `go.mod` with the `github.com/whatap/go-api` dependency added
- `go.sum`
- `_modules/` subtree with `replace` directives (when `--external-module` is used)

You can change into the output directory and run `go build` directly — it will compile without needing `whatap-go-inst` again. This is useful for inspection, code review, CI artifact, or air-gapped builds.

---

## Example

**Project layout:**
```
myapp/
├── go.mod
├── main.go
└── handler.go
```

**Build + dump:**
```bash
whatap-go-inst --output=./instrumented go build -o myapp .
```

**Result:**
```
instrumented/
├── go.mod        # includes github.com/whatap/go-api
├── go.sum
├── main.go       # with trace.Init / middleware injected
└── handler.go    # with httpc / fmt / log transforms applied
```

**Inspect diffs:**
```bash
diff -r . ./instrumented | head -40
# or per-file:
diff main.go instrumented/main.go
```

---

## With `--external-module`

```bash
whatap-go-inst --output=./instrumented \
    --external-module="mycompany.com/internal/*" \
    go build ./...
```

Matching GOMODCACHE modules are copied into `instrumented/_modules/<sanitized_name>/` with whatap transforms applied, and `instrumented/go.mod` gets matching `replace` directives so the output tree is self-contained. See [Multi-Module Projects](./multi-module.md).

---

## Verifying 1-to-1 restoration

Under the build-wrapper flow, the original tree is never written to, so a 1-to-1 "remove" pass is unnecessary — the `whatap-go-inst remove` subcommand exists to clean up **manually written** `go-api` calls (typical use case: migrating from hand-rolled instrumentation to the auto build-wrapper), not to undo auto-injected changes. You can prove the wrapper leaves originals alone by asserting the source tree is untouched:

```bash
# Before build
sha256sum $(find ./myapp -name '*.go') > /tmp/before.sha

# Instrumented build
whatap-go-inst --output=./instrumented go build -o /tmp/myapp ./myapp

# After build — originals must be unchanged
sha256sum $(find ./myapp -name '*.go') > /tmp/after.sha
diff /tmp/before.sha /tmp/after.sha    # must be empty
```

The transformed copy lives in `./instrumented/`; the original `./myapp/` is byte-identical after the build.

---

## Next Steps

- [Build Wrapper Mode](./build-wrapper.md) — the default workflow
- [Multi-Module Projects](./multi-module.md) — `--external-module` details
- [Configuration Guide](./config.md) — `.whatap/config.yaml` fields
