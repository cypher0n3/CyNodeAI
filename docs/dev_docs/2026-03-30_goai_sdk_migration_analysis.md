# GoAI SDK Migration Analysis: Langchaingo => GoAI

- [Executive Summary](#executive-summary)
- [GoAI SDK Profile](#goai-sdk-profile)
  - [Project Maturity](#project-maturity)
  - [Core Capabilities](#core-capabilities)
  - [What GoAI Does Not Have](#what-goai-does-not-have)
- [CyNodeAI langchaingo Usage Surface](#cynodeai-langchaingo-usage-surface)
  - [PMA (Project Manager Agent)](#pma-project-manager-agent)
  - [SBA (Sub-Agent)](#sba-sub-agent)
  - [Shared MCP Tool Bridge](#shared-mcp-tool-bridge)
- [Feature-by-Feature Compatibility](#feature-by-feature-compatibility)
  - [Ollama Integration](#ollama-integration)
  - [Tool Calling and Agent Loop](#tool-calling-and-agent-loop)
  - [Streaming Architecture](#streaming-architecture)
  - [MCP Client](#mcp-client)
  - [Structured Output](#structured-output)
  - [Thinking/Reasoning Separation](#thinkingreasoning-separation)
  - [Error Handling and Fallback Patterns](#error-handling-and-fallback-patterns)
- [Migration Effort Estimate](#migration-effort-estimate)
  - [PMA Migration (High Effort)](#pma-migration-high-effort)
  - [SBA Migration (Very High Effort)](#sba-migration-very-high-effort)
  - [MCP Tool Bridge (Medium Effort)](#mcp-tool-bridge-medium-effort)
  - [Test Infrastructure (High Effort)](#test-infrastructure-high-effort)
  - [Total Estimate](#total-estimate)
- [Risk Analysis](#risk-analysis)
  - [Risks of Switching to GoAI](#risks-of-switching-to-goai)
  - [Risks of Staying on langchaingo](#risks-of-staying-on-langchaingo)
- [Recommendation](#recommendation)
  - [Near-Term Strategy](#near-term-strategy)
  - [Re-Evaluation Triggers](#re-evaluation-triggers)

## Executive Summary

This report evaluates replacing `github.com/tmc/langchaingo v0.1.14` with `github.com/zendev-sh/goai` as the LLM SDK for CyNodeAI's PMA and SBA agents.
GoAI is a modern, well-designed Go SDK with generics, channel-based streaming, native MCP support, and zero external dependencies.
However, it is extremely new (14 stars, 2 contributors, pre-1.0), and CyNodeAI's agent architecture relies on langchaingo abstractions (agent executors, ReAct parsing, `tools.Tool` interface) that GoAI deliberately does not replicate.
**The migration is technically feasible but not advisable now.**
The effort-to-benefit ratio is poor given CyNodeAI's current needs and GoAI's maturity.

## GoAI SDK Profile

GoAI (`github.com/zendev-sh/goai`) is an open-source Go SDK for building AI applications, inspired by the Vercel AI SDK.

### Project Maturity

- **Version**: v0.5.2 (pre-1.0, no stability guarantees)
- **GitHub**: 14 stars, 3 forks, 2 identifiable contributors (`vietanh85` + `github-actions`)
- **First release**: early 2026; the project is approximately 2-3 months old
- **License**: MIT
- **Dependencies**: stdlib only (except `golang.org/x/oauth2` for Vertex AI)
- **Go requirement**: Go 1.25+
- **Production adoption**: zero known public production deployments found in search
- **Community feedback**: no third-party reviews, experience reports, or blog posts found

### Core Capabilities

GoAI provides seven core functions with a unified API across 22+ providers.

- `GenerateText` / `StreamText` for text generation and real-time streaming via Go channels
- `GenerateObject[T]` / `StreamObject[T]` for type-safe structured output using Go generics
- `Embed` / `EmbedMany` for text embeddings
- `GenerateImage` for image generation
- Built-in tool calling with auto-loop via `WithMaxSteps(n)`
- `OnStepFinish` and `OnToolCall` hooks for observability
- Native MCP client with stdio, HTTP, and SSE transports
- Prompt caching for Anthropic and OpenAI
- Provider-defined tools (web search, code execution, computer use)
- One-line provider switching (all providers implement the same `provider.LanguageModel` interface)

### What GoAI Does Not Have

These gaps are relevant to CyNodeAI's architecture.

- **No agent executor abstraction**: there is no `OneShotAgent`, `OpenAIFunctionsAgent`, or `Executor` pattern.
  The "agent loop" is implicit inside `GenerateText`/`StreamText` when `WithMaxSteps > 1`.
- **No ReAct parser**: GoAI relies entirely on the provider's native tool-calling API.
  Models that emit `Action:` / `Action Input:` text (langchaingo's `OneShotAgent` format) would not be parsed.
- **No `tools.Tool` interface**: tools are defined as `goai.Tool` structs with an `Execute func(ctx, json.RawMessage) (string, error)` handler.
  The `Name() string` / `Description() string` / `Call(ctx, string) (string, error)` interface from langchaingo does not exist.
- **No `schema.AgentStep` equivalent**: intermediate steps are accessed via `result.Steps []StepResult` but with a different structure (no `Action.Tool`, no `Observation` fields).
- **No error sentinels for agent flow**: `agents.ErrNotFinished` and `agents.ErrAgentNoReturn` do not exist.
  CyNodeAI uses these to trigger fallback paths.

## CyNodeAI Langchaingo Usage Surface

The project currently depends on `github.com/tmc/langchaingo v0.1.14` in `agents/go.mod`.
14 files import langchaingo packages across 6 upstream packages.

### PMA (Project Manager Agent)

PMA uses the **OpenAI-compatible function-calling path** via langchaingo.

- [`agents/internal/pma/langchain.go`](../../agents/internal/pma/langchain.go): Core agent loop.
  Creates `openai.New()` pointed at Ollama's `/v1` endpoint, wraps it in `agents.NewOpenAIFunctionsAgent`, and runs `agents.NewExecutor().Call()`.
  Extracts output from `outputs["output"]`, separates `<think>` blocks, and runs `tryRepairTextualMCPCalls` to handle Ollama's JSON-in-content quirk.
  This is the deepest coupling point.
- [`agents/internal/pma/streaming.go`](../../agents/internal/pma/streaming.go): Streaming LLM wrapper.
  Implements `llms.Model` interface to intercept `GenerateContent` calls and emit NDJSON (`iteration_start`, `delta`, `thinking`, `done`) to the HTTP response writer.
  Uses `llms.WithStreamingFunc` for per-token callbacks.
- [`agents/internal/pma/chat.go`](../../agents/internal/pma/chat.go): Fallback logic.
  Catches `agents.ErrNotFinished` and `agents.ErrAgentNoReturn` to route to direct Ollama inference when the agent loop fails.
- [`agents/internal/pma/mcp_tools.go`](../../agents/internal/pma/mcp_tools.go): Returns `tools.Tool` via the `mcpclient` bridge.

### SBA (Sub-Agent)

SBA uses the **ReAct one-shot agent** path.

- [`agents/internal/sba/agent.go`](../../agents/internal/sba/agent.go): Core agent loop.
  Creates `ollama.New()` with native Ollama API, wraps it in `agents.NewOneShotAgent`, runs `agents.NewExecutor().Call()`.
  Processes `schema.AgentStep` from `outputs["intermediateSteps"]` to build `sbajob.StepResult` entries.
  Falls back to direct `/api/chat` for small models.
  Uses `llms.GenerateFromSinglePrompt` for the direct path.
- [`agents/internal/sba/agent_tools.go`](../../agents/internal/sba/agent_tools.go) / [`agents/internal/sba/mcp_tools.go`](../../agents/internal/sba/mcp_tools.go): Local and MCP tools implementing `tools.Tool`.
- [`agents/internal/sba/mock_llm.go`](../../agents/internal/sba/mock_llm.go): Test double implementing `llms.Model`.

### Shared MCP Tool Bridge

- [`agents/internal/mcpclient/langchain_tool.go`](../../agents/internal/mcpclient/langchain_tool.go): `LangchainTool` implements `tools.Tool` to forward JSON to the MCP gateway's `POST /v1/mcp/tools/call`.
  Used by both PMA and SBA.

## Feature-By-Feature Compatibility

This section maps each CyNodeAI requirement against GoAI SDK capabilities.

### Ollama Integration

- **langchaingo**: PMA uses `llms/openai` pointed at `<ollama>/v1` (OpenAI-compat API) because `llms/ollama` does not pass `WithFunctions`.
  SBA uses `llms/ollama` natively.
- **GoAI**: has a dedicated `provider/ollama` package wrapping the OpenAI-compatible endpoint at `localhost:11434/v1`.
  Supports `WithBaseURL` and `WithHTTPClient` for custom transports.
- **Gap**: GoAI's Ollama provider does not expose Unix Domain Socket (UDS) transport directly.
  CyNodeAI uses `http+unix://` URLs with custom `http.Client` dialer.
  This could be handled via `WithHTTPClient` on GoAI's `compat` provider.
  ✅ Technically compatible with minor adapter work.

### Tool Calling and Agent Loop

- **langchaingo PMA**: `OpenAIFunctionsAgent` + `Executor` with `WithMaxIterations(20)`.
  The executor drives the call => tool-result => re-call loop.
  Returns `map[string]any{"output": string}`.
- **langchaingo SBA**: `OneShotAgent` (ReAct) + `Executor` with `WithMaxIterations(50)`.
  Returns `intermediateSteps []schema.AgentStep`.
- **GoAI**: `GenerateText(ctx, model, WithTools(...), WithMaxSteps(n))`.
  The SDK internally runs the same loop (call => execute => re-call) but returns `*TextResult` with `Steps []StepResult`.
  Each step has `ToolCalls []ToolCall` and can be observed via `OnStepFinish`.
- **Gap 1 (PMA)**: GoAI's tool loop is functionally equivalent to `OpenAIFunctionsAgent` for models that support native tool calling.
  The `tryRepairTextualMCPCalls` workaround for Ollama's JSON-in-content quirk would need to be reimplemented using `OnStepFinish` hooks or post-processing.
  Migration is medium complexity.
- **Gap 2 (SBA)**: GoAI has **no ReAct parser**.
  SBA's `OneShotAgent` parses `Action:` / `Action Input:` text output from models that do not support native function calling.
  Models that only produce text-format tool calls would break.
  This is a **hard blocker** for any SBA model that relies on ReAct-style output parsing.
  For models with native tool calling, GoAI's `WithMaxSteps` would work.
- **Gap 3**: GoAI's `goai.Tool` uses `Execute func(ctx, json.RawMessage) (string, error)` instead of langchaingo's `Call(ctx, string) (string, error)`.
  All tool implementations would need signature changes.

### Streaming Architecture

- **langchaingo**: Callback-based via `llms.WithStreamingFunc(func(ctx, []byte) error)`.
  PMA's `streamingLLM` wraps the inner model and intercepts callbacks to write NDJSON.
- **GoAI**: Channel-based via `StreamText()` returning `*TextStream`.
  Consumer reads from `stream.TextStream()` (`<-chan string`) or `stream.Stream()` (`<-chan StreamChunk`).
  Chunk types include `ChunkText`, `ChunkReasoning`, `ChunkToolCall`, `ChunkStepFinish`, `ChunkFinish`.
- **Gap**: The streaming model is fundamentally different.
  CyNodeAI's NDJSON streaming pipeline (`streamingLLM`, `writeLangchainNDJSONStream`, `streamingClassifier`) hooks into langchaingo's callback model.
  With GoAI, the NDJSON emitter would consume from a Go channel instead, which is actually a **cleaner pattern** but requires a full rewrite of the streaming pipeline.
  GoAI's `ChunkReasoning` type natively separates thinking content, which could simplify `extractThinkBlocks`.
- **Net**: GoAI's streaming is architecturally better but requires complete rewrite of the PMA streaming layer.

### MCP Client

- **langchaingo**: No built-in MCP support.
  CyNodeAI implements its own MCP bridge via `mcpclient.LangchainTool` that forwards JSON to a gateway HTTP endpoint.
- **GoAI**: Native MCP client (`goai/mcp` package) with stdio, HTTP, and SSE transports.
  `mcp.ConvertTools(client, tools)` auto-converts MCP tools to `goai.Tool` values.
- **Assessment**: CyNodeAI's MCP architecture uses a gateway HTTP API (`POST /v1/mcp/tools/call`), not direct MCP protocol.
  GoAI's MCP client speaks native MCP protocol to servers directly.
  These are **different integration patterns**.
  CyNodeAI would still need a thin HTTP-to-`goai.Tool` adapter for its gateway, similar to the current `LangchainTool` but with GoAI's `Execute` signature.
  GoAI's native MCP client could be useful for future direct-to-server integrations but does not replace the existing gateway pattern.

### Structured Output

- **langchaingo**: Manual JSON schema + unmarshaling.
- **GoAI**: `GenerateObject[T]` with Go generics for type-safe structured output.
- **Assessment**: CyNodeAI does not currently use structured output from langchaingo (PMA/SBA extract plain text from `outputs["output"]`).
  This is a future benefit, not a migration driver.

### Thinking/Reasoning Separation

- **langchaingo**: No built-in support.
  CyNodeAI implements `extractThinkBlocks()` to parse `<think>...</think>` XML tags from model output.
- **GoAI**: Native `ChunkReasoning` stream chunk type for models that support extended thinking.
  `Reasoning` capability flag in `ModelCapabilities`.
- **Gap**: GoAI's reasoning separation works with providers that use their API's reasoning fields (Anthropic, OpenAI).
  Ollama models like Qwen3 emit `<think>` tags in the content field rather than via a separate API field.
  GoAI's `ChunkReasoning` may or may not capture Ollama `<think>` tags (depends on whether Ollama's OpenAI-compat API surfaces them separately).
  CyNodeAI's manual `<think>` parsing would likely still be needed for Ollama models.

### Error Handling and Fallback Patterns

- **langchaingo**: `agents.ErrNotFinished` and `agents.ErrAgentNoReturn` sentinel errors used by PMA's `chat.go` for fallback routing.
- **GoAI**: No equivalent sentinels.
  When `MaxSteps` is exhausted, `GenerateText` returns the last step's result rather than an error.
- **Gap**: PMA's fallback path (agent loop fails => direct Ollama call) would need to be reimplemented using different detection logic (e.g., checking if `result.FinishReason` indicates tool-call exhaustion, or checking step count vs max steps).

## Migration Effort Estimate

The following estimates assume a developer familiar with both SDKs and the CyNodeAI agent codebase.

### PMA Migration (High Effort)

- Rewrite `langchain.go`: replace `OpenAIFunctionsAgent` + `Executor` with `goai.GenerateText` + `WithTools` + `WithMaxSteps`.
  Reimplement `tryRepairTextualMCPCalls`.
  Adapt `extractOutput` to GoAI's `TextResult`.
  Estimated 3-4 days.
- Rewrite `streaming.go`: replace callback-based `streamingLLM` with channel-consuming NDJSON emitter reading from `StreamText`.
  Adapt the `streamingClassifier` pipeline.
  Estimated 2-3 days.
- Update `chat.go`: replace `ErrNotFinished` / `ErrAgentNoReturn` fallback with GoAI-compatible detection.
  Estimated 1 day.
- Update `mcp_tools.go`: adapt `NewMCPTool` to return `goai.Tool` instead of `tools.Tool`.
  Estimated 0.5 days.

### SBA Migration (Very High Effort)

- Rewrite `agent.go`: replace `OneShotAgent` (ReAct) with `goai.GenerateText` + `WithMaxSteps`.
  **Must verify that all target models support native tool calling through Ollama's OpenAI-compat API**, because GoAI has no ReAct text parser.
  Rewrite `processStepsToResult` to map GoAI's `StepResult` to `sbajob.StepResult`.
  Replace `ollama.New()` with GoAI's `ollama.Chat()`.
  Estimated 3-4 days.
- Rewrite `agent_tools.go` and `mcp_tools.go`: convert all `tools.Tool` implementations to `goai.Tool` structs with `Execute` handlers.
  Estimated 1-2 days.
- Rewrite `mock_llm.go` and all test files: GoAI's model interface is different from `llms.Model`.
  Test doubles need complete replacement.
  Estimated 2-3 days.

### MCP Tool Bridge (Medium Effort)

- Rewrite `langchain_tool.go`: change from `tools.Tool` to `goai.Tool`.
  The HTTP forwarding logic stays the same; only the interface changes.
  Estimated 0.5 days.

### Test Infrastructure (High Effort)

- All existing tests (`langchain_test.go`, `chat_stream_test.go`, `mock_llm_test.go`, `agent_test.go`, `runner_test.go`) depend on langchaingo types for mocking and assertions.
  Every test would need updating.
  Estimated 3-4 days.
- BDD step files (`pma_streaming_steps.go`) reference langchaingo behavior in narrative text but do not import it.
  Minor updates.

### Total Estimate

- **Development**: 14-22 person-days
- **Testing and verification**: 5-8 person-days
- **Risk buffer**: 5-8 person-days (new SDK, undiscovered edge cases)
- **Total**: approximately 24-38 person-days

## Risk Analysis

Both options carry risk; the question is which risk profile better fits CyNodeAI's current phase.

### Risks of Switching to GoAI

- **Extreme bus factor**: 14 stars, 2 contributors.
  If the maintainer stops, the project dies.
  No corporate backing.
  CyNodeAI would inherit an abandoned dependency.
- **Pre-1.0 API instability**: GoAI is at v0.5.x.
  Breaking changes are expected.
  Every GoAI release could require CyNodeAI code changes.
- **Zero production track record**: no known production deployments.
  Edge cases, performance under load, and provider-specific quirks are undiscovered.
- **No ReAct agent pattern**: SBA models that rely on text-format tool calls (`Action:` / `Action Input:`) would break.
  This constrains SBA to models with native function calling support.
- **Ollama `<think>` tag handling**: unclear whether GoAI's Ollama provider surfaces `<think>` content via `ChunkReasoning` or as regular text.
  May need custom parsing regardless.
- **Go 1.25+ requirement**: CyNodeAI would need to stay on recent Go versions.
  This is likely acceptable but is a dependency to track.
- **Migration regression risk**: 14-22 days of rewriting core agent infrastructure across both PMA and SBA.
  High chance of introducing regressions in tool execution, streaming, error handling, and secret redaction.

### Risks of Staying on Langchaingo

- **Slow upstream development**: `tmc/langchaingo` has merged few PRs in 2025-2026.
  The maintainer is responsive but the velocity is low.
- **Pre-1.0 API**: langchaingo is at v0.1.14, also pre-1.0.
  However, its API surface has been stable for the past year in practice.
- **No Go generics**: langchaingo uses `interface{}` patterns rather than generics.
  This is a code quality issue but not a functional blocker.
- **Known Ollama quirks**: langchaingo's `llms/ollama` does not pass `WithFunctions`, requiring the OpenAI-compat workaround.
  This is already handled and well-understood.
- **Dependency tree**: langchaingo pulls in a significant transitive dependency set.
  Manageable with pinning but larger than GoAI's zero-dep approach.

## Recommendation

**Do not switch to GoAI SDK at this time.**

The costs and risks significantly outweigh the benefits.

- GoAI is architecturally modern and well-designed, but it is 2-3 months old with 14 stars and no production users.
  Betting CyNodeAI's agent infrastructure on it would be premature.
- The migration requires 24-38 person-days of work across PMA, SBA, MCP tools, and tests.
  This is a large investment for what is primarily a dependency swap, not a feature gain.
- The SBA's ReAct agent pattern has no GoAI equivalent, which would constrain model choices or require building a custom ReAct parser on top of GoAI.
- langchaingo v0.1.14 is stable enough for CyNodeAI's pinned, well-understood usage.
  Its quirks are already worked around.

### Near-Term Strategy

- **Stay on `tmc/langchaingo v0.1.14`** with the existing pin.
- **Continue abstracting langchaingo behind internal interfaces** where practical (the `tools.Tool` bridge in `mcpclient` is already a good example).
  This reduces future migration cost regardless of which SDK CyNodeAI eventually moves to.
- **Evaluate GoAI SDK again when it reaches v1.0** or achieves meaningful community adoption (100+ stars, multiple contributors, known production users).
- **Consider GoAI SDK's MCP client** as a standalone addition if CyNodeAI needs direct MCP server connections (not through the gateway).
  GoAI's `mcp` package could potentially be imported independently.

### Re-Evaluation Triggers

Any of the following should trigger a new assessment.

- GoAI SDK reaches v1.0 with stability commitments
- GoAI SDK reaches 200+ stars with 5+ contributors
- langchaingo is officially abandoned or its API breaks in a way that affects CyNodeAI
- CyNodeAI needs native structured output (`GenerateObject[T]`) or provider-defined tools (web search, code execution) that GoAI provides and langchaingo does not
- CyNodeAI's SBA moves entirely to models with native function calling, removing the ReAct dependency
