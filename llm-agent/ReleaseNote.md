# Go Auto-Instrumentation — LLM Release Notes (Separate Track)

> **Separate track**: LLM SDK auto-instrumentation has its own release notes and is opt-in (`llm_enabled=true`), but follows the main version (lockstep). See [`../ReleaseNote.md`](../ReleaseNote.md) for the main auto-instrumentation release notes.

---

## Go auto-instrumentation — LLM v0.6.0 (MVP)

June 24, 2026

- [New] LLM SDK auto-instrumentation.

    User code calling `sashabaranov/go-openai`, `cloudwego/eino-ext`, `anthropics/anthropic-sdk-go`, or `openai/openai-go` (the official OpenAI Go SDK) is rewritten to route every HTTP call through a WhaTap-wrapped transport, so LLM metrics, token counts, prompts/responses, and per-transaction `is-llm=1` markers are emitted without manual code changes. Requires `llm_enabled=true` in `whatap.conf` at runtime.

    **Auto-converted libraries**

    | SDK | Auto-injected scope |
    |-----|---------------------|
    | `github.com/sashabaranov/go-openai` (v1.40.5+) | `NewClient` / `NewClientWithConfig` + `Client.CreateChatCompletion` / `CreateChatCompletionStream` / `CreateCompletion` / `CreateEmbeddings` |
    | `github.com/cloudwego/eino-ext/components/model/openai` | `NewChatModel` (constructor only) |
    | `github.com/cloudwego/eino-ext/components/model/claude` | `NewChatModel` (constructor only) |
    | `github.com/anthropics/anthropic-sdk-go` (v1.26.0+) | `NewClient` (constructor) + `MessageService.New` / `MessageService.NewStreaming` |
    | `github.com/openai/openai-go` (v1.12.0+) | `NewClient` (constructor) + `ChatCompletionService.New` / `ChatCompletionService.NewStreaming` |

    For eino-ext, the constructor is auto-injected; `Generate` / `Stream` method metadata still needs `whatapeino.WrapChatModel(cm)` because the upstream returns an interface, which cannot be auto-matched.

    For Anthropic and openai-go, the `*Service.New` / `NewStreaming` rules use `Signature{MaxArgs: 2}` so the canonical `client.<Service>.New(ctx, params)` form is auto-converted while call sites that pass trailing `option.RequestOption` arguments are skipped — use the corresponding `whatapanthropic.WrapAndNewMessage(...)` / `whatapopenaigo.WrapAndNewChatCompletion(...)` helpers directly for those.

    **Built on the RoundTrip single entry point design**

    The fourteen new `Transform` rules (six sashabaranov + two eino-ext + three Anthropic + three openai-go) replace the SDK's HTTPClient with `&http.Client{Transport: whataphttp.NewLLMRoundTrip(ctx, base)}` (for Anthropic and openai-go, this is prepended as an `option.WithHTTPClient(...)` request option; the SDK's last-write-wins order means user-supplied HTTPClient options still take precedence). The wrapped transport marks every call as an LLM API call regardless of URL host, so mock servers (`localhost`) and self-hosted endpoints are classified correctly. When `llm_enabled=false`, the wrapper falls back to the regular HTTPC path with zero behavioural change.

    Fourteen new rules bring the total to 106 built-in rules across 10 advice types.

---
