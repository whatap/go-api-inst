# LLM Monitoring

WhaTap's Go SDK monitors LLM API calls through four paths. Pick the one that matches how your application reaches the LLM:

| Path | Code required | When to use |
|---|---|---|
| **A. Auto-inject** | 0 lines | Your project imports one of the supported SDKs (sashabaranov / Eino / Anthropic / openai-go). The build wrapper rewrites SDK entry points at compile time. |
| **B. Explicit adapter wrap** | 1 line per call site | You want the wrapping to be visible in source. Same runtime result as Path A. |
| **C. Manual API (`llm.Start`)** | 5–10 lines | None of the supported SDKs covers your case (custom HTTP endpoint, internal model, novel SDK). |
| **D. URL auto-match** | 0 lines | Independent of A/B/C. Any HTTP call through `whataphttp.NewRoundTrip` whose host matches a known LLM provider is automatically attributed to LLM metrics. |

All four paths require **one configuration line**: `llm_enabled=true` in `whatap.conf`. LLM data ships over the same TCP 6600 channel as regular monitoring.

---

## 1. Enable LLM monitoring

```ini
# whatap.conf
llm_enabled=true
```

Without this flag, LLM-specific metrics and LogSinkPacks are not published even if the SDK call itself is captured as a regular `httpc` external call.

---

## 2. Path A — Auto-inject (recommended)

If your project already imports one of the SDKs below, you do not need to change any code. `whatap-go-inst go build` recognises the SDK and wraps the relevant entry points during the build.

### Supported SDKs (v0.6.0)

| Go LLM SDK | go.mod path | Adapter |
|---|---|---|
| sashabaranov OpenAI SDK | `github.com/sashabaranov/go-openai` | `whatapopenai` |
| Eino — OpenAI provider | `github.com/cloudwego/eino-ext/components/model/openai` | `whatapeino` |
| Eino — Claude provider | `github.com/cloudwego/eino-ext/components/model/claude` | `whatapeino` |
| Anthropic SDK | `github.com/anthropics/anthropic-sdk-go` | `whatapanthropic` |
| OpenAI official SDK | `github.com/openai/openai-go` | `whatapopenaigo` |

Build:

```bash
whatap-go-inst go build ./...
```

The build wrapper injects:

- A wrapping call around the SDK's chat / completion / embedding entry point.
- A `whataphttp.NewRoundTrip` transport replacement so each HTTP call produces an `httpc` step **and** a paired LLM step.
- Token usage, TTFT, TPOT, provider, model, and operation type extraction from the SDK response.

The auto-inject rules pull in `github.com/whatap/go-api/instrumentation/llm` (a nested module) automatically — you do not need to `go get` it yourself.

---

## 3. Path B — Explicit adapter wrap

Same runtime result as Path A, but the wrap is written by hand. Use this when you want the instrumentation to be visible at review time or when your build does not run through `whatap-go-inst`.

### sashabaranov go-openai

```go
import (
    "context"
    "net/http"

    openai "github.com/sashabaranov/go-openai"
    whataphttp "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
    whatapopenai "github.com/whatap/go-api/instrumentation/llm/github.com/sashabaranov/go-openai/whatapopenai"
)

func chat(ctx context.Context) (string, error) {
    cfg := openai.DefaultConfig("YOUR_API_KEY")
    cfg.HTTPClient = &http.Client{
        Transport: whataphttp.NewRoundTrip(ctx, http.DefaultTransport),
    }
    client := whatapopenai.WrapClient(openai.NewClientWithConfig(cfg))

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    "gpt-4o",
        Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "what is 6 * 7?"}},
    })
    if err != nil {
        return "", err
    }
    return resp.Choices[0].Message.Content, nil
}
```

Two things matter:

1. `cfg.HTTPClient.Transport` is wrapped with `whataphttp.NewRoundTrip` so each HTTP call goes through whatap.
2. The SDK client is wrapped with `whatapopenai.WrapClient` so the adapter can extract token usage and message content from the SDK's structured response.

Manual wrappers need to pull in the nested module explicitly:

```bash
go get github.com/whatap/go-api/instrumentation/llm@v0.6.0
```

### cloudwego Eino

```go
import (
    "github.com/cloudwego/eino/components/model"
    einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
    whataphttp "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
    whatapeino "github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino"
)

func chat(ctx context.Context) (string, error) {
    cm, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
        APIKey:     "YOUR_API_KEY",
        Model:      "gpt-4o",
        HTTPClient: &http.Client{Transport: whataphttp.NewRoundTrip(ctx, http.DefaultTransport)},
    })
    if err != nil {
        return "", err
    }
    var wrapped model.ChatModel = whatapeino.WrapChatModel(cm)
    out, err := wrapped.Generate(ctx, messages)
    if err != nil {
        return "", err
    }
    return out.Content, nil
}
```

---

## 4. Path C — Manual API (`llm.Start`)

When you call an LLM API that is not on the supported-SDK list (a custom HTTP endpoint, an internal model gateway, a SaaS without a Go SDK), use the manual API.

### Minimal example

This mirrors [`go-api-example/llm/main.go`](https://github.com/whatap/go-api-example/tree/main/llm):

```go
import (
    "github.com/whatap/go-api/llm"
    whataphttp "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
)

func chatHandler(c *gin.Context) {
    ctx, step := llm.Start(c.Request.Context(), llm.Config{
        Provider:      "openai",
        Model:         "gpt-4o",
        OperationType: "chat",
    })
    defer step.End()

    step.AddSystemMessage("You are a helpful assistant.")
    step.AddInputMessage(userPrompt)

    // The HTTP transport MUST be wrapped — the LLMState that llm.Start put on
    // ctx is attached by the next httpc.Start inside the wrapped RoundTripper.
    client := &http.Client{Transport: whataphttp.NewRoundTrip(ctx, http.DefaultTransport)}
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.example.com/v1/chat", body)
    resp, err := client.Do(req)
    if err != nil {
        step.SetError(err, llm.ErrorTypeAPI)
        return
    }

    // Extract token usage from the parsed response.
    step.SetTokens(llm.Tokens{Input: 50, Output: 12})
    step.AddOutputMessage(parsedContent)
}
```

### How it works

1. `llm.Start` returns a `ctx` carrying a pending `LLMState`.
2. The next `httpc.Start` in the call chain — typically inside the wrapped `RoundTripper` — finds the pending state, attaches it to the `HttpcCtx`, and fills in the real request URL.
3. `httpc.End` (called by the `RoundTripper` when the HTTP response is finished) publishes both the HTTP step and the LLM metrics / LogSinkPack.
4. `step.End()` is a defer-friendly no-op once the `RoundTripper` has already closed the step (the common case). It exists so the call site reads like other Go resource patterns.

If your SDK does **not** route through `whataphttp.NewRoundTrip`, no `httpc` step is produced and the LLM step is dropped. The wrapped transport is mandatory.

### `Step` methods

| Method | Purpose |
|---|---|
| `AddSystemMessage(content string)` | Record the system prompt. |
| `AddInputMessage(content string)` | Record the user input. |
| `AddOutputMessage(content string)` | Record the model response. |
| `SetTokens(llm.Tokens{Input, Output, ...})` | Record token usage. Fields are `int64`. |
| `SetCost(llm.Cost{...})` | Record cost in USD (optional — pricing.go is on the roadmap). |
| `SetError(err error, errType string)` | Record an error. Use `llm.ErrorTypeAPI` for upstream failures and `llm.ErrorTypeProgram` for local bugs. |
| `MarkStream()` | Signal that this call returned a streaming response so TTFT / TPOT are computed. |
| `End()` | Close the step. Safe to defer — no-op when the RoundTripper already closed it. |

### Adapter / auto-inject path — `llm.Bind`

If you manage `httpc.Start` / `End` yourself and only need to attach LLM state to an existing `HttpcCtx`:

```go
hc, _ := httpc.Start(ctx, url)
defer hc.End(status, "", err)
state := llm.Bind(hc, llm.Config{Provider: "ollama", Model: "llama3", OperationType: "chat"})
state.SetTokens(...)
```

The auto-inject rules use this entry point internally — when `whataphttp.NewRoundTrip` recognises a known LLM URL, it calls `llm.Bind` itself.

---

## 5. Path D — URL auto-match

With `llm_enabled=true`, any HTTP call routed through `whataphttp.NewRoundTrip` that targets a known LLM host is automatically attributed to LLM metrics — no adapter call, no `llm.Start` required. Provider and OperationType are inferred from the URL.

### Recognised hosts (v0.6.0)

- `api.openai.com`
- `api.anthropic.com`
- `*.azure.openai.com` (Azure OpenAI)
- `generativelanguage.googleapis.com` (Gemini)
- `bedrock-runtime.*.amazonaws.com` (AWS Bedrock)
- Plus a small number of others — see `go-api/agent/agent/llm/url_match.go` for the authoritative list.

If the URL matcher gets the Provider wrong for your case, override it via Path B (adapter wrap) or Path C (`llm.Start` with explicit `Provider`).

---

## 6. What gets collected

### Metrics (15-minute meter window)

| Category | Items |
|---|---|
| `active` | Number of in-flight LLM calls. |
| `api_status` | Success / failure counts per provider × model. |
| `error` | Error counts by type (`api` / `program`). |
| `feature` | Call frequency by provider × model × operation_type. |
| `perf` | TTFT (Time To First Token), TPOT (Time Per Output Token), end-to-end latency. |
| `token_usage` | Input / output / total token sums. |
| `tx_status` | Status distribution of transactions that contain at least one LLM call. |

### LogSinkPack (per call)

Each LLM call publishes seven log rows under the `#LlmCallLog` category (request, response, metadata, token, error, etc.). Payloads larger than 20 KB are split on UTF-8 boundaries.

### Transaction marking

Any transaction that contains an LLM call is marked with `is-llm=1` (ExtraField) and the corresponding `HttpcStepX.Driver` is set to `"LLM API"`, so the Argus dashboard can filter LLM-bearing transactions cleanly.

---

## 7. Known limitations

| Item | Status |
|---|---|
| Automatic cost calculation (40+ models) | Not yet — `pricing.go` port is a separate ticket. |
| `perf` KLL sketch backend | Under review. |
| Multimodal payloads (images / audio) for Anthropic and openai-go | Partial — recorded as a `placeholder` only; the raw bytes are not collected. |
| `langchaingo` adapter | Backlog (priority follows Argus demand). |

---

## 8. Troubleshooting

| Symptom | Likely cause | Check |
|---|---|---|
| Transactions appear but no LLM metrics | `llm_enabled=true` missing | `whatap.conf` |
| LLM calls captured but `token_usage = 0` | Response-shape mismatch in the adapter | Look for `WA-LLM` lines in debug logs |
| Provider shown as `"unknown"` | URL auto-match failed and `llm.Config.Provider` not set | Set `Provider` explicitly when calling `llm.Start` |
| Auto-inject didn't wrap the SDK | Nested module require missing (usually added automatically) | Look for `github.com/whatap/go-api/instrumentation/llm` in `go.mod` |
| Streaming response but `TPOT = 0` | RoundTripper isn't seeing the SSE chunks | Switch to Path B (adapter) or Path C (manual API) |

---

## 9. Reference examples

Runnable examples live in the [go-api-example](https://github.com/whatap/go-api-example) repository:

- [`llm/`](https://github.com/whatap/go-api-example/tree/main/llm) — Path C manual API (`llm.Start`) for an arbitrary HTTP LLM endpoint.
- [`github.com/cloudwego/eino`](https://github.com/whatap/go-api-example/tree/main/github.com/cloudwego/eino) — Path B explicit wrap (`whatapeino.WrapChatModel`).
- [`github.com/sashabaranov/go-openai`](https://github.com/whatap/go-api-example/tree/main/github.com/sashabaranov/go-openai) — Path B explicit wrap (`whatapopenai.WrapClient`).
- [`github.com/anthropics/anthropic-sdk-go`](https://github.com/whatap/go-api-example/tree/main/github.com/anthropics/anthropic-sdk-go) — `whatapanthropic.WrapAndNewMessage`.
- [`github.com/openai/openai-go`](https://github.com/whatap/go-api-example/tree/main/github.com/openai/openai-go) — `whatapopenaigo.WrapAndNewChatCompletion`.

> Path A (auto-inject) requires no source change — `whatap-go-inst go build` instruments the SDK calls above automatically.
