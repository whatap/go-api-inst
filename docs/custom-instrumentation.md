# Custom Instrumentation Guide

Define custom instrumentation rules for in-house libraries or legacy code that the 115 built-in rules don't cover. Rules are declared in `.whatap/config.yaml` under the `rules:` array and are applied by the **same engine** as the built-in rules.

> **Status (2026-04-14)**: Unified schema. The legacy `custom: { inject:/hook:/replace:/transform: }` block has been removed; see §11 *Migrating from the legacy schema*.

---

## 1. Quick start

```yaml
# .whatap/config.yaml
version: 1

instrumentation:
  # v0.6.0 (breaking) — the `preset` field has been removed. Built-in rules now
  # auto-match on the packages your project actually imports. Leave
  # `enabled_packages` / `disabled_packages` empty unless you need to
  # override the defaults:
  # enabled_packages: [fmt]                          # activate an opt-in rule
  # disabled_packages: [github.com/labstack/echo/v4] # skip a specific package
  debug: true

importAliases:
  whatapsql: "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"

rules:
  - type: replace
    target: "database/sql.Open"
    with: "whatapsql.Open"
```

```bash
# fast mode (default)
whatap-go-inst go build -o myapp .
```

---

## 2. Core concepts

| Concept | Description |
|---|---|
| **Single engine** | Built-in 115 rules and your custom rules are applied by the same engine in one pass. The precise type-based matching and every other safety net the built-ins enjoy applies to your rules automatically. |
| **One `rules:` array** | Every rule is an entry in the `rules:` array. The `type:` discriminator picks one of 13 kinds. |
| **`add:` is top-level** | File-creation (`add`) is processed *outside* the engine, so it lives in a top-level `add:` array — **not** inside `rules:`. |
| **Target string** | `pkg.Func` (call), `decl:pkgpath.Func` (function declaration), `lit:pkg.Type{}` (composite literal). Same notation the built-in 115 rules use. |
| **Last-write-wins** | If two rules share the same target, only the **last** rule applies. The legacy "all rules accumulate" behaviour is gone. |
| **Exact beats wildcard** | When an exact target and a wildcard both match the same function, the exact rule wins. |

---

## 3. The 13 rule types

10 are shared with the built-in catalogue. 3 (`hook`/`inject`/`add`) are user-only.

### 3.1 Call-site transformations

| type | Purpose | Example target |
|---|---|---|
| `replace` | Swap one call for another (alias change only) | `database/sql.Open` |
| `replace-with-ctx` | Replace + inject `ctx` as the first argument | `net/http.Get`, `net/http.DefaultClient.Get` |
| `wrap-call` | Wrap the entire call expression in another function | `github.com/gin-gonic/gin.Default` |
| `arg-wrap` | Wrap one argument with another function (`argIndex: 0` or `-1` = last) | `log.New` |
| `arg-insert` | Append new arguments to a variadic call (e.g. gRPC interceptors) | `google.golang.org/grpc.NewServer` |
| `code-insert` | Insert a separate statement before/after the call | `k8s.io/client-go/kubernetes.NewForConfig` |
| `main-insert` | Wrap a one-shot call inside `main()` (e.g. `log.SetOutput`) | `log.SetOutput` |
| `transform` | Free-form template-driven transformation (closure wrap, IIFE, …) | `github.com/aerospike/aerospike-client-go/v6.Client.Put` |
| `hook` | Insert statements **before/after** the call line — the user-defined workhorse | `mypkg.fetchData`, `os.Getenv` |

### 3.2 Function declaration transformations

| type | Purpose |
|---|---|
| `inject` | Insert code at the start/end of a function body. Targets functions in your own module only — Go stdlib and external packages are never modified. |

### 3.3 Composite literal transformations

| type | Purpose |
|---|---|
| `field-wrap` | Wrap a field value inside `Type{Field: x}` |
| `field-wrap-or-insert` | Wrap if the field exists, insert if it doesn't (one rule, two paths) |

### 3.4 File creation (engine-external)

| type | Location | Purpose |
|---|---|---|
| `add` | top-level `add:` array | Create a new Go file in the target package (append mode was removed in v0.6.0 — see §11) |

---

## 4. Target string syntax

Identical notation to the built-in rules. The yaml loader interprets the prefixes.

| Form | Matches | Example |
|---|---|---|
| `pkg.Func` | Function call (`call:` is the implicit default) | `database/sql.Open`, `os.Getenv` |
| `pkg.Var.Func` | Method call on a package-level variable | `net/http.DefaultClient.Get` |
| `pkg.Type.Method` | Method call (pointer receivers handled automatically) | `github.com/aerospike/.../v6.Client.Put` |
| `lit:pkg.Type{}` | Composite literal | `lit:net/http.Server{}` |
| `decl:pkgpath.Func` | Function declaration in your module | `decl:myapp/service.ProcessOrder` |
| `decl:pkgpath.Type.Method` | Method declaration | `decl:net/http.Server.ListenAndServe` |

### 4.1 `decl:` wildcards

`decl:` targets accept a single `*` wildcard.

| Pattern | Meaning | Example matches |
|---|---|---|
| `decl:*` | Any function declaration | All user functions |
| `decl:pkgpath.*` | All functions in a package | `decl:myapp.Foo`, `decl:myapp.Bar` |
| `decl:pkgpath.Pre*` | Function-name prefix | `decl:myapp.ProcessOrder`, `decl:myapp.Process` |
| `decl:pkgpath.*Suf` | Function-name suffix | `decl:myapp.RequestHandler`, `decl:myapp.HTTPHandler` |
| `decl:pkgpath.A*Z` | Single middle wildcard (added in 2026-04-14) | `decl:myapp.GetUserDB`, `decl:myapp.GetOrderDB` |

> **Call-site wildcards are NOT supported.** `pkg.Func*` style wildcards on the call side are rejected by the engine — enumerate them instead.

### 4.2 Bare-identifier hooks (calls inside the same package, added 2026-04-14)

You can hook on a bare-identifier call — `fetchData()` rather than `pkg.fetchData()` — when the function lives in the *same* package as the call. The tool resolves the function's full import path from Go's type information and matches it as `<importpath>.<funcname>`.

```yaml
rules:
  - type: hook
    target: "myapp.fetchData"     # main package's fetchData() calls
    before: 'log.Println(">>> fetchData starting")'
    after:  'log.Println("<<< fetchData done")'
    imports: ["log"]
```

> The `target` uses the function's **import path**. For the `main` package that's the module path itself (the `module` line in `go.mod`); for sub-packages it's `<module>/<sub>`.

### 4.3 stdlib call-site hooks

Hooks can target stdlib calls like `os.Getenv` or `strings.Contains`. **The stdlib package is never modified** — the inserted code lives only in the user files that *call* the function.

```yaml
rules:
  - type: hook
    target: "os.Getenv"
    before: 'fmt.Println("[ENV] >>> Getenv called")'
    after:  'fmt.Println("[ENV] <<< Getenv done")'
    imports: ["fmt"]
```

Result (in your file):
```go
fmt.Println("[ENV] >>> Getenv called")
val := os.Getenv("FOO")
fmt.Println("[ENV] <<< Getenv done")
```

---

## 5. imports / importAliases

stdlib packages and external/whatap packages are split into two fields.

| Field | Form | Used for |
|---|---|---|
| `imports` | array of paths | stdlib or any package whose default alias (last segment of the path) is good enough — `fmt`, `time`, `context`, etc. |
| `importAliases` | map alias → path | Packages that need an explicit alias different from the path's last segment (most whatap packages) |

### 5.1 File-global vs rule-level

Both fields can be declared at the **file global** level and at the **rule** level.

```yaml
version: 1

# file global
imports: ["context", "time"]
importAliases:
  whatapsql: "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"

rules:
  - type: replace
    target: "database/sql.Open"
    with: "whatapsql.Open"
    # uses the global alias

  - type: replace
    target: "github.com/jinzhu/gorm.Open"
    with: "whatapgorm.Open"
    # rule-level override (even if a global whatapgorm exists, this rule sees the override)
    importAliases:
      whatapgorm: "github.com/whatap/go-api/instrumentation/github.com/jinzhu/gorm/whatapgorm"
```

Merge rules:
- `imports`: global ∪ rule-level (union, deduplicated)
- `importAliases`: global + rule-level merged; **the same alias is overridden by the rule level**

### 5.2 Alias collision case (gorm/redis/sarama/echo)

The built-in 115 rules have collision cases where one alias name (`whatapgorm`, `whatapgoredis`, `whatapsarama`, `whatapecho`) maps to different packages. If you need the same pattern in your user rules, declare one path globally and override the other at the rule level.

---

## 6. The `with` field — `"alias.Func"` everywhere

For most types the whatap function is given as a single `with: "alias.Func"` string. The loader looks the alias up in `importAliases` to recover the full path and distributes it across the internal struct fields (`WhatapPkg`/`WhatapAlias`/`WhatapFunc`).

```yaml
- type: replace
  target: "database/sql.Open"
  with: "whatapsql.Open"   # alias=whatapsql, func=Open
```

**Exceptions — types that need two functions:**

| type | Split fields | Why |
|---|---|---|
| `field-wrap-or-insert` | `wrapWith` + `insertWith` | One function for the "field exists" case, another for "insert". **Both must use the same alias** — the internal struct holds a single `WhatapPkg`/`WhatapAlias` pair. |
| `arg-insert` | `whatapAlias` + `insertArgs[].{wrapFunc, innerFunc}` | `wrapFunc` lives on the *target* package, `innerFunc` lives on the *whatap* package — two packages at once. |

---

## 7. Template variables (transform / hook / inject)

Standard `text/template` syntax: `{{.Variable}}`. The same variable set is available in `transform`, `hook`, and `inject`.

| Variable | Type | Meaning | Available in |
|---|---|---|---|
| `{{.Original}}` | string | Full original call expression (`client.Put(p, k, b)`) | transform, hook |
| `{{.Var}}`, `{{.Var1}}` | string | First/second LHS variable when the call is in an assignment (`r := gin.Default()` → `r`) | transform |
| `{{.Args}}` | string | All argument text (`"a, b, c"`) | transform, hook |
| `{{.Arg0}}`–`{{.Arg3}}` | string | Individual arguments (up to 4) | transform, hook |
| `{{.ArgsList}}` | []string | Arguments as a slice for iteration | transform, hook |
| `{{.Args1Plus}}` | string | Arguments after `Arg0`, comma-joined. The original call's spread form (`hosts...`) is preserved automatically so the substituted call still compiles for both spread and individual-arg call sites | transform, hook |
| `{{.IsSpread}}` | bool | Whether the original call passed its last argument with `...` (for `{{if .IsSpread}}` branching) | transform, hook |
| `{{.ArgCount}}` | int | Argument count | all |
| `{{.ArgAt N}}` | func(int) string | Arbitrary index access (`{{.ArgAt 4}}`) | transform, hook |
| `{{.HasCtx}}` | bool | call: a `ctx` argument is present / decl: the function parameters include `context.Context` | all |
| `{{.Receiver}}` | string | Method receiver variable (`client.Put(...)` → `client`) | transform, hook |
| `{{.FuncName}}` | string | Function/method name (`Put`) | all |
| `{{.PkgName}}` | string | Caller's local package alias | transform, hook |
| `{{.Ctx}}` | string | Detected `ctx` expression (or `context.Background()`) | transform, hook |
| `{{.TargetPkg}}` | string | Alias resolved from the target's import path | transform |
| `{{.File}}` | string | Matched file path | inject (declaration context) |

### 7.1 inject's `{{.HasCtx}}`

For `inject` rules, `{{.HasCtx}}` checks the **function declaration's parameter list** for `context.Context`. This is different from `transform`/`hook`'s `{{.HasCtx}}` (which checks for a `ctx` argument at the call site).

```yaml
- type: inject
  target: "decl:myapp/service.*"
  start: |
    {{if .HasCtx}}ctx = trace.StartMethod(ctx, "{{.FuncName}}")
    defer trace.EndMethod(ctx, nil){{end}}
```

Only functions that take `ctx` get the trace calls; others are left untouched.

### 7.2 No `template_file:`

The new schema only supports inline `template:` strings. The legacy `template_file:` field is gone — use a yaml literal block (`|`) for large templates.

---

## 8. Precise matching filters (optional)

Like the built-in rules, you can attach precise-match filters. All are optional; nil = pass.

```yaml
- type: replace
  target: "database/sql.Open"
  with: "whatapsql.Open"

  # argument count / type matching
  signature:
    args:
      - { package: "", name: "string" }
      - { package: "", name: "string" }
    results:
      - { package: "database/sql", name: "*DB" }
      - { package: "", name: "error" }
    minArgs: -1     # -1 = derived from len(args)
    maxArgs: -1     # -1 = unbounded (variadic)

  # method receiver type
  receiver:
    package: "database/sql"
    name: "*DB"

  # composite literal field requirement
  fields:
    - { name: "Handler", required: true }
```

Use these when you have lots of overloaded/look-alike functions and want to avoid false positives. Keep simple cases simple.

### 8.1 Field typo detection (strict decoding)

The loader applies **strict decoding** at every level: the **top-level** fields (`version` / `imports` / `importAliases` / `rules` / `add` + `add[i]` internals) and the **`rules[i]` internals** (`type` / `target` / `with` / `template` / `before` / `after` / `start` / `end` / `signature.*` / `receiver.*` / `fields[*].*` / `insertArgs[*].*`). Unknown field names are rejected with a clear error — typos never get silently dropped into a downstream "empty type" symptom.

```yaml
# ❌ typo example
rules:
  - tpye: replace           # "type" typo
    tagret: "foo.Bar"       # "target" typo
    wiht: "whatapfoo.Bar"   # "with" typo
```

**Error message**:
```
Error: rules[0]: yaml: unmarshal errors:
  line 1: field tpye not found in type ast.RuleSpec
  line 2: field tagret not found in type ast.RuleSpec
  line 3: field wiht not found in type ast.RuleSpec
```

Nested struct typos (inside `signature` / `insertArgs[*]` / etc.) are reported the same way:

```yaml
rules:
  - type: replace
    target: "foo.Bar"
    with: "whatapfoo.Bar"
    signature:
      minarg: 1             # "minArgs" typo
# → rules[0]: yaml: unmarshal errors:
#     line 5: field minarg not found in type ast.SignatureSpec
```

**Note**: Strict field validation re-parses each rule with a strict YAML decoder. The per-rule overhead (tens of μs) is negligible for realistic rule counts.

**Special case — `type: add`**: A `type: add` entry inside the `rules:` array is rejected **before** strict field checking with the message "handled outside the Engine — declare add rules in the top-level \"add:\" array". Add rules live in a dedicated top-level field.

---

## 9. Engine semantics — what changed from the legacy schema

After the schema unification (2026-04-14), the following behaviours differ from the old schema; please read carefully when migrating.

### 9.1 Last-write-wins

When several rules share the same target, **only the last one is registered**. The legacy schema accumulated all of them.

```yaml
rules:
  - type: inject
    target: "decl:myapp.ProcessData"
    start: 'log.Println("first")'

  - type: inject
    target: "decl:myapp.ProcessData"   # same target
    start: 'log.Println("second")'     # ← only this applies
```

If you genuinely want multiple actions, fold them into one rule.

### 9.2 Exact beats wildcard

When an exact target and a wildcard both match a function, **the exact rule wins**.

```yaml
rules:
  - type: inject
    target: "decl:myapp.ProcessData"  # exact
    start: 'log.Println("exact")'

  - type: inject
    target: "decl:myapp.Process*"     # wildcard
    start: 'log.Println("wild")'
```

`ProcessData` gets only `"exact"`. Other `Process*` functions (`ProcessOrder`, `ProcessRefund`, …) get `"wild"`.

### 9.3 No call-site wildcards

`hook`/`replace`/`transform` and the other call-site rules **do not accept wildcard targets**. Enumerate them.

```yaml
# ❌ unsupported
rules:
  - type: hook
    target: "myapp.DoTask*"

# ✅ enumerate
rules:
  - type: hook
    target: "myapp.DoTaskA"
  - type: hook
    target: "myapp.DoTaskB"
  - type: hook
    target: "myapp.DoTaskC"
```

(`decl:` wildcards in §4.1 *are* supported.)

---

## 10. Rule type support

Every rule type works under the fast build. (The legacy wrap/inject **build modes** were removed in v0.6.0, leaving fast as the only mode. The `inject` row below is the rule type that inserts code into a function body, not a build mode.)

| Rule type | Supported |
|---|:---:|
| `replace` | ✓ |
| `wrap-call` / `arg-wrap` / `arg-insert` etc. | ✓ |
| `transform` | ✓ |
| `hook` (call-site) | ✓ |
| `inject` (function body) | ✓ |
| `field-wrap` / `field-wrap-or-insert` | ✓ |
| `add` (file creation) | ✓ |

> **fast mode supports `add` rules.** `whatap-go-inst go build` creates the target file under the user's project directory **before** invoking `go build`, and `defer`-removes it after the build completes (success or failure), so the original source tree is restored. The created files are also persisted into `whatap-instrumented/` so the output is reproducible. Target files are **never overwritten** — if a file with the same path already exists, the build aborts with an error so the user can resolve the conflict. `content_file` paths are resolved relative to the directory containing `.whatap/config.yaml`.
>
> **v0.6.0 breaking change — `append: true` removed.** Migrate `append` rules to new-file `add` rules in the same package (see §11 for the migration guide).

---

## 11. Migrating from the legacy schema

The old `custom: { inject:/replace:/hook:/transform:/add: }` block is gone. The mapping is straightforward.

| Legacy | New schema |
|---|---|
| `custom: { replace: [{package, function, with, imports}] }` | `rules: [{type: replace, target: "<pkg>.<func>", with: "<alias>.<Func>", importAliases: {...}}]` |
| `custom: { hook: [{package, function, before, after, imports}] }` | `rules: [{type: hook, target: "<pkg>.<func>", before, after, imports}]` |
| `custom: { inject: [{package, function, start, end, imports}] }` | `rules: [{type: inject, target: "decl:<pkgpath>.<funcpattern>", start, end, imports}]` |
| `custom: { transform: [{package, function, template, imports}] }` | `rules: [{type: transform, target: "<pkg>.<func>", template, imports}]` |
| `custom: { add: [{package, file, content}] }` | `add: [{package, file, content}]` (top-level — **not** under `rules:`) |
| `custom: { add: [{package, file, content, append: true}] }` | **Removed in v0.6.0** — migrate to a new-file add rule (see §11.3) |

### 11.1 Translating inject targets

The legacy `package: "main"` referred to the **Go package name** (the `package main` declaration in the source). The new `decl:` form uses the **import path** instead.

| Legacy | New schema |
|---|---|
| `package: "main", function: "Process*"` | `decl:<module-path>.Process*` |
| `package: "user"` (a function in `myapp/pkg/user`) | `decl:myapp/pkg/user.<funcpattern>` |

`<module-path>` is the `module` line in `go.mod`. Even functions in the `main` package are addressed as `decl:<module-path>.<func>`.

### 11.2 Behavioural differences to watch for

- The legacy "all rules accumulate" semantics → new last-write-wins. If a legacy rule pair was meant to layer two pieces of code, fold them into a single rule.
- Legacy call-site wildcards (`Handle*` in a `hook`) → enumerate.
- Legacy `template_file:` → inline the template.

### 11.3 append → new-file add migration (v0.6.0 breaking)

The `append: true` flag on `add` rules has been removed. Migrate by creating a **new file in the same package** instead of editing an existing one. Go's package-scope semantics mean the code is functionally identical regardless of which file in the package declares it.

**Before (v0.5.4, now rejected by the loader)**:
```yaml
add:
  - package: "pkg/user"
    file: "user.go"
    append: true
    content: |
      func GetUserWithTrace(id int) (*User, error) {
        fmt.Println("[APPENDED]")
        return GetUser(id)
      }
```

**After (v0.6.0)**:
```yaml
add:
  - package: "pkg/user"
    file: "whatap_user_ext.go"          # must be a new filename, not user.go
    content: |
      package user                       # explicit package declaration

      import "fmt"                       # list every import used below

      func GetUserWithTrace(id int) (*User, error) {
        fmt.Println("[APPENDED]")
        return GetUser(id)               # same package — original symbols accessible
      }
```

**Rules**:
1. `file:` must be a **new filename**, different from any existing file in the package (targets are never overwritten).
2. `content:` is a **complete Go source file** — `package` declaration, `import` block, then the declarations. No auto-wrapping; what you write is what is placed on disk.
3. Avoid filenames matching the default exclude patterns (`**/*_generated.go`, `**/*_gen.go`, `**/*_test.go`, etc.) or fast mode will skip instrumenting the file. Recommended: `whatap_*_ext.go`.
4. All declarations in the existing package are accessible (Go's package scope), so helper functions / types / constants can be referenced directly.
5. If the loader encounters `append: true` it aborts the build with a migration message.

**Why it was removed**:
- Fast mode would need a separate build-time path to edit existing files without corrupting the user's source tree (high complexity, build-cache edge cases).
- Every real-world use case of append is "add a declaration to a package", which is equivalent to "add a new file to the same package" in Go.
- Simplification: one code path for wrap/inject/fast, one documentation page, zero edge cases.

### 11.4 preset removed + full-path enable/disable (v0.6.0, breaking change)

The legacy schema accepted `instrumentation.preset` together with short-name filters (`enabled_packages: [gin]`). v0.6.0 drops the preset concept entirely and switches the filter to **exact-match package paths**.

**Before** (legacy preset schema):

```yaml
instrumentation:
  preset: full                  # or minimal/web/database/custom
  enabled_packages: [gin]       # short name from the canonical PresetPackages table
  disabled_packages: [fmt]
```

**After** (current schema, v0.6.0):

```yaml
instrumentation:
  # preset field removed entirely
  enabled_packages:
    - fmt                       # stdlib keeps its short canonical path
  disabled_packages:
    - github.com/gin-gonic/gin  # external modules use the full go.mod require path
```

**Migration table**:

| Legacy (preset schema) | Current (v0.6.0) |
|---|---|
| `preset: full` (or unset) | Unset. Note `fmt.Print*` is now opt-in — add `enabled_packages: [fmt]` if you still want to collect it. |
| `preset: minimal` | Stop using `whatap-go-inst` and invoke `go build` directly. No replacement flag. |
| `preset: web` / `database` / `custom` | No 1-to-1 mapping. List the package paths you want to exclude under `disabled_packages` (copy template blocks from [instrumentation-rules.md](./instrumentation-rules.md)). |
| `enabled_packages: [gin]` | `enabled_packages: [github.com/gin-gonic/gin]` |
| `enabled_packages: [sql]` | `enabled_packages: [database/sql]` |
| `disabled_packages: [fmt]` | Unchanged (stdlib stays short). Redundant since fmt is already opt-in. |
| Rule-level `name:` field | Removed. Use `id:` for identifiers. |

**Behavioural differences**:
- Built-in rules now pre-load every package up front. If your project does not import a package, the engine skips it on its own — no need to pre-filter with a preset.
- Exact match — `disabled_packages: [github.com/labstack/echo]` only excludes v3. Add `github.com/labstack/echo/v4` on its own line to also exclude v4. There are no prefixes or wildcards.
- `fmt.Print/Printf/Println` (and only those three at the time of writing) are opt-in. They do not register unless `enabled_packages` lists `fmt`.
- Unknown paths warn on `stderr` (`[whatap-go-inst] unknown package in enabled_packages: "…"`) and the build continues. They do not fail.
- User-defined rules inside `rules:` are **not** filtered. They always register regardless of `disabled_packages`.

**What to change**:
1. Delete `instrumentation.preset` if it exists.
2. Replace any short names in `enabled_packages` / `disabled_packages` with full package paths (table above).
3. For "category exclude" use cases, copy one of the ready-made blocks from [instrumentation-rules.md](./instrumentation-rules.md).

**Why**:
- v2 Target-level matching already gives us project-based auto-instrumentation — `preset` had become a "turn off things the user is actually using" lever more than a useful pre-filter.
- The canonical short-name table (`PresetPackages`, 22 entries) was never published as stable API, so users guessed between `redis` / `goredis` / `go-redis`. Exact-match on the go.mod path removes that ambiguity.
- `default=full` silently imposed `whatapfmt` overhead on high-frequency log apps (Loki, Promtail). Making `fmt` opt-in resolves that specific overhead regression and generalises cleanly to future hot-path rules.
- Report JSON now uses the same full paths a user types in config, so the two can be diffed against each other.

---

## 12. End-to-end example

```yaml
# .whatap/config.yaml
version: 1

instrumentation:
  # enabled_packages / disabled_packages omitted — built-in rules register
  # automatically for every package the project imports.
  debug: true

importAliases:
  whatapsql: "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"
  whatapgin: "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
  trace:     "github.com/whatap/go-api/trace"

rules:
  # 1. Replace the database driver entry point.
  - type: replace
    target: "database/sql.Open"
    with: "whatapsql.Open"

  # 2. Wrap gin.Default() to attach our middleware.
  - type: transform
    target: "github.com/gin-gonic/gin.Default"
    template: |
      {{.Original}}
      {{.Var}}.Use(whatapgin.Middleware())

  # 3. Trace every Process* function in package main.
  - type: inject
    target: "decl:myapp.Process*"
    start: |
      ctx, span := trace.Start(ctx, "{{.FuncName}}")
      defer span.End()

  # 4. Bracket calls to the legacy fetchData helper.
  - type: hook
    target: "myapp.fetchData"
    before: 'log.Println(">>> fetchData")'
    after:  'log.Println("<<< fetchData")'
    imports: ["log"]

# Top-level — file creation is engine-external.
add:
  - package: "main"
    file: "whatap_init.go"
    content: |
      package main

      import "github.com/whatap/go-api/trace"

      func init() {
          trace.Init(nil)
      }
```

```bash
whatap-go-inst go build -o myapp .
```

---

## 13. Related documents

- [config.md](./config.md) — config file reference (enabled/disabled packages, env vars, search order)
- [instrumentation-rules.md](./instrumentation-rules.md) — supported packages catalog and yaml template blocks
- [user-guide.md](./user-guide.md) — overall user guide
