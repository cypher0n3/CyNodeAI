# LLM Routing and Model Handling: Draft Spec

- [1. Purpose and Status](#1-purpose-and-status)
- [2. Scope](#2-scope)
- [3. Model Catalogue](#3-model-catalogue)
  - [3.1 Local Models (Ollama)](#31-local-models-ollama)
  - [3.2 External Models (API Egress)](#32-external-models-api-egress)
  - [3.3 Fallback Policy for Unlisted Models](#33-fallback-policy-for-unlisted-models)
- [4. Model Capability Record](#4-model-capability-record)
  - [4.1 `ModelCapabilityRecord` Type](#41-modelcapabilityrecord-type)
  - [4.2 Capability Fields Reference](#42-capability-fields-reference)
- [5. Agent Inference Path Selection](#5-agent-inference-path-selection)
  - [5.1 `AgentInferencePath` Rule](#51-agentinferencepath-rule)
  - [5.2 `AgentInferencePath` Algorithm](#52-agentinferencepath-algorithm)
- [6. Thinking-Block Handling](#6-thinking-block-handling)
  - [6.1 `ThinkingBlockStrip` Rule](#61-thinkingblockstrip-rule)
- [7. Tool-Calling Protocol Selection](#7-tool-calling-protocol-selection)
  - [7.1 `ToolCallProtocol` Rule](#71-toolcallprotocol-rule)
- [8. Output Normalisation](#8-output-normalisation)
  - [8.1 `AgentOutputNormalise` Rule](#81-agentoutputnormalise-rule)
- [9. Shared Tooling Contract (PMA and SBA)](#9-shared-tooling-contract-pma-and-sba)
  - [9.1 `AgentModelTooling` Interface](#91-agentmodeltooling-interface)
- [10. References](#10-references)

## 1. Purpose and Status

**Status:** Draft (dev\_docs).
Not normative.
No code changes unless explicitly directed.

**Purpose:** Define per-model and per-provider routing decisions, tool-call protocol selection,
thinking-block handling, and output normalisation for all agents that perform inference
(currently PMA and SBA).
This draft is intended to be promoted to
`docs/tech_specs/llm_model_handling.md` once reviewed.

The existing codebase uses an `isCapableModel` heuristic in
`agents/internal/pma/langchain.go` that only distinguishes two paths (capable vs.
small/direct).
This spec replaces that two-level heuristic with a structured per-model
capability record and a deterministic path-selection algorithm that covers:

- all currently deployed local Ollama models,
- the major external providers (OpenAI, Anthropic, Google Gemini, xAI Grok),
- strong offline coding models,
- a safe fallback for any model not explicitly listed.

### 1.1. Traces To

- [REQ-PMAGNT-0100](../requirements/pmagnt.md#req-pmagnt-0100)
- [REQ-PMAGNT-0101](../requirements/pmagnt.md#req-pmagnt-0101)
- REQ-AGENTS-0010 (TBD - agents requirements not yet created)
- REQ-MODELS-0100 (TBD - models requirements not yet created)

---

## 2. Scope

This document covers:

- The catalogue of supported models and their capability records.
- The algorithm agents use to select an inference path (functions-agent, direct-generation,
  streaming-aware) for a given model name.
- Thinking-block handling: how `<think>…</think>`, `<thinking>…</thinking>`, and similar
  blocks are detected and stripped from agent-facing output.
- Tool-call protocol selection: OpenAI-functions API vs. ReAct text prompting vs.
  direct generation with no tools.
- Output normalisation rules shared across PMA and SBA.
- The Go interface that both agents must implement so the logic is not duplicated.

Out of scope: model loading workflows, model registry persistence, orchestrator
`SelectProjectManagerModel` algorithm (see
[`orchestrator.md`](../tech_specs/orchestrator.md#spec-cynai-orches-projectmanagermodelstartup)),
and API Egress credential management.

---

## 3. Model Catalogue

This catalogue enumerates all supported model families with their capability metadata.

### 3.1 Local Models (Ollama)

The table below lists every model family expected in a CyNodeAI deployment.
`tool_protocol` is defined in [Section 7](#7-tool-calling-protocol-selection);
`thinking` is defined in [Section 6](#6-thinking-block-handling).

- **Family prefix:** `qwen3:`
  - example tags: `qwen3:8b`, `qwen3:14b`, `qwen3:32b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: `xml_think`
  - notes: Native `<tool_call>` XML; `<think>` blocks
- **Family prefix:** `qwen3.5:`
  - example tags: `qwen3.5:9b`, `qwen3.5:35b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: `xml_think`
  - notes: Same family conventions as qwen3
- **Family prefix:** `qwen3.5:0.8b`
  - example tags: `qwen3.5:0.8b`
  - `tool_protocol`: `direct`
  - `thinking`: `xml_think`
  - notes: Too small for reliable tool calls; direct generation only
- **Family prefix:** `qwen3:1b`
  - example tags: `qwen3:1b`
  - `tool_protocol`: `direct`
  - `thinking`: `xml_think`
  - notes: Same
- **Family prefix:** `qwen2.5:`
  - example tags: `qwen2.5:14b`, `qwen2.5:32b`, `qwen2.5:72b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Stable tool calling; no thinking blocks
- **Family prefix:** `qwen2.5-coder:`
  - example tags: `qwen2.5-coder:7b`, `qwen2.5-coder:14b`, `qwen2.5-coder:32b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Strong offline coder; tool-call capable
- **Family prefix:** `deepseek-r1:`
  - example tags: `deepseek-r1:7b`, `deepseek-r1:14b`, `deepseek-r1:32b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: `xml_think`
  - notes: Reasoning model; thinking in `<think>` tags
- **Family prefix:** `deepseek-coder-v2:`
  - example tags: `deepseek-coder-v2:16b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Strong offline coder
- **Family prefix:** `codestral:`
  - example tags: `codestral:22b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Mistral offline coder
- **Family prefix:** `llama3.1:`
  - example tags: `llama3.1:8b`, `llama3.1:70b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Meta Llama 3.1; tool-call capable
- **Family prefix:** `llama3.2:`
  - example tags: `llama3.2:3b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Capable at >=3B for tools
- **Family prefix:** `llama3.3:`
  - example tags: `llama3.3:70b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: -
- **Family prefix:** `mistral:`
  - example tags: `mistral:7b`, `mistral-nemo:`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: -
- **Family prefix:** `mixtral:`
  - example tags: `mixtral:8x7b`, `mixtral:8x22b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: -
- **Family prefix:** `phi4:`
  - example tags: `phi4:14b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Microsoft Phi-4
- **Family prefix:** `phi3:`
  - example tags: `phi3:mini`
  - `tool_protocol`: `direct`
  - `thinking`: none
  - notes: Too small for reliable tools
- **Family prefix:** `tinyllama:`
  - example tags: `tinyllama:1.1b`
  - `tool_protocol`: `direct`
  - `thinking`: none
  - notes: -
- **Family prefix:** `gemma3:`
  - example tags: `gemma3:12b`, `gemma3:27b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Google Gemma 3 local
- **Family prefix:** `gemma3:1b`
  - example tags: `gemma3:1b`
  - `tool_protocol`: `direct`
  - `thinking`: none
  - notes: -
- **Family prefix:** `command-r:`
  - example tags: `command-r:35b`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Cohere Command R local

**Small-model rule:** Any model whose tag ends with `:0.5b`, `:0.6b`, `:0.8b`, `:1b`,
`:1.1b`, `:1.5b`, or `:1.8b` is classified `direct` regardless of family prefix.

**Unknown local model rule:** Any Ollama model name not matched by the above prefixes or
the small-model rule is classified as `openai_functions` with `thinking: none` as a best-effort
default (Ollama's function-call API will be tried; on parse error the agent falls back to
`direct`).
See [Section 3.3](#33-fallback-policy-for-unlisted-models).

### 3.2 External Models (API Egress)

External models are identified by their provider-namespaced string as sent over the OpenAI-compatible
`model` field via API Egress.
The table uses the canonical model ID prefix as documented by each
provider as of early 2026.

- **Provider:** **OpenAI**
  - model id prefix: `gpt-4o`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: GPT-4o family
- **Provider:** **OpenAI**
  - model id prefix: `gpt-4o-mini`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: -
- **Provider:** **OpenAI**
  - model id prefix: `gpt-4-turbo`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: -
- **Provider:** **OpenAI**
  - model id prefix: `gpt-4.5`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Preview / Turbo
- **Provider:** **OpenAI**
  - model id prefix: `o1`
  - `tool_protocol`: `direct`
  - `thinking`: `openai_think`
  - notes: o1 reasoning; no tool API in streaming; strip `<thinking>`
- **Provider:** **OpenAI**
  - model id prefix: `o3`
  - `tool_protocol`: `openai_functions`
  - `thinking`: `openai_think`
  - notes: o3 supports tools + thinking
- **Provider:** **OpenAI**
  - model id prefix: `o4-mini`
  - `tool_protocol`: `openai_functions`
  - `thinking`: `openai_think`
  - notes: -
- **Provider:** **Anthropic**
  - model id prefix: `claude-3-5-sonnet`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Claude 3.5 Sonnet
- **Provider:** **Anthropic**
  - model id prefix: `claude-3-5-haiku`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: -
- **Provider:** **Anthropic**
  - model id prefix: `claude-3-7-sonnet`
  - `tool_protocol`: `openai_functions`
  - `thinking`: `anthropic_think`
  - notes: Extended thinking; strip `<thinking>` blocks
- **Provider:** **Anthropic**
  - model id prefix: `claude-4`
  - `tool_protocol`: `openai_functions`
  - `thinking`: `anthropic_think`
  - notes: Future; same conventions
- **Provider:** **Google**
  - model id prefix: `gemini-2.0-flash`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Gemini 2.0 Flash via OpenAI-compat endpoint
- **Provider:** **Google**
  - model id prefix: `gemini-2.0-pro`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: -
- **Provider:** **Google**
  - model id prefix: `gemini-2.5-pro`
  - `tool_protocol`: `openai_functions`
  - `thinking`: `google_think`
  - notes: Gemini 2.5 Pro thinking; strip `<thinking>`
- **Provider:** **xAI**
  - model id prefix: `grok-2`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Grok-2 via xAI OpenAI-compat endpoint
- **Provider:** **xAI**
  - model id prefix: `grok-3`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: -
- **Provider:** **xAI**
  - model id prefix: `grok-3-mini`
  - `tool_protocol`: `openai_functions`
  - `thinking`: `xai_think`
  - notes: Grok-3 mini reasoning; strip `<thinking>`
- **Provider:** **Mistral**
  - model id prefix: `mistral-large`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Mistral Large API
- **Provider:** **Mistral**
  - model id prefix: `codestral-latest`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: Mistral Codestral API
- **Provider:** **Cohere**
  - model id prefix: `command-r-plus`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: -
- **Provider:** **DeepSeek**
  - model id prefix: `deepseek-chat`
  - `tool_protocol`: `openai_functions`
  - `thinking`: none
  - notes: DeepSeek V3 API
- **Provider:** **DeepSeek**
  - model id prefix: `deepseek-reasoner`
  - `tool_protocol`: `openai_functions`
  - `thinking`: `xml_think`
  - notes: DeepSeek R1 API; `<think>` blocks

**Unknown external model rule:** Any external model ID not matched by the table above is classified
as `openai_functions` with `thinking: none`.
See [Section 3.3](#33-fallback-policy-for-unlisted-models).

### 3.3 Fallback Policy for Unlisted Models

When a model name is not found in the catalogue:

1. Attempt `openai_functions` path (OpenAI-compatible tool-call API or Ollama function-call API).
2. If the response cannot be parsed as a tool-call or text completion, fall back to `direct`
   (single-pass generation without tool calls).
3. Log a warning that the model is unrecognised so operators can add it to the catalogue.
4. Never raise an unrecoverable error solely because of an unknown model name.

---

## 4. Model Capability Record

This section defines the runtime type agents use to represent resolved model capabilities.

### 4.1 `ModelCapabilityRecord` Type

- Spec ID: `CYNAI.MODELS.ModelCapabilityRecord` <a id="spec-cynai-models-modelcapabilityrecord"></a>
- Status: draft

A `ModelCapabilityRecord` is the runtime representation of the capability metadata
for one resolved model name.
Agents construct or look up a record before invoking
an LLM.
The record is immutable once constructed.

#### 4.1.1 `ModelCapabilityRecord` Fields

- `model_name` (string): Resolved model name as supplied to the inference backend.
- `tool_protocol` (string enum): One of `openai_functions`, `react`, `direct`.
  - `openai_functions`: The model supports the OpenAI tool-calling API (or Ollama's
    equivalent `<tool_call>` format).
    Use `OpenAIFunctionsAgent`.
  - `react`: The model produces reliable ReAct `Thought/Action/Final Answer` text output.
    Use `OneShotAgent`.
    This protocol is reserved for models explicitly validated to
    produce this format; no model in the current catalogue is assigned `react` as default.
  - `direct`: No tool calling.
    Use a single-pass generation call (`callInference`).
- `thinking` (string enum): One of `none`, `xml_think`, `anthropic_think`,
  `openai_think`, `google_think`, `xai_think`.
  Maps to the stripping rule in [Section 6](#6-thinking-block-handling).
- `max_context_tokens` (int, optional): Declared context window in tokens.
  0 means unknown.
- `supports_vision` (bool): Whether the model accepts image inputs.
- `supports_json_mode` (bool): Whether the model supports a native JSON-mode output constraint.
- `provider` (string enum): `local_ollama`, `openai`, `anthropic`, `google`,
  `xai`, `mistral`, `cohere`, `deepseek`, `unknown`.

### 4.2 Capability Fields Reference

The following table maps the `thinking` enum values to the concrete patterns stripped
from model output before the agent receives it.
Stripping is defined precisely in
[Section 6](#6-thinking-block-handling).

- **`thinking` value:** `none`
  - strip pattern: No stripping
- **`thinking` value:** `xml_think`
  - strip pattern: `<think>…</think>` (whole block, greedy, may be multi-line)
- **`thinking` value:** `anthropic_think`
  - strip pattern: `<thinking>…</thinking>` blocks as emitted in Anthropic streaming chunks
- **`thinking` value:** `openai_think`
  - strip pattern: `<thinking>…</thinking>` blocks in the `reasoning_content` field; absent in final text
- **`thinking` value:** `google_think`
  - strip pattern: `<thinking>…</thinking>` blocks in Gemini extended-thinking responses
- **`thinking` value:** `xai_think`
  - strip pattern: `<thinking>…</thinking>` blocks in xAI reasoning responses

---

## 5. Agent Inference Path Selection

This section defines how agents choose between the functions-agent, ReAct, and direct-generation paths.

### 5.1 `AgentInferencePath` Rule

- Spec ID: `CYNAI.AGENTS.AgentInferencePath` <a id="spec-cynai-agents-agentinferencepath"></a>
- Status: draft

Each agent (PMA, SBA) must resolve a `ModelCapabilityRecord` for the configured
`INFERENCE_MODEL` before invoking the LLM, then select the inference path according
to the algorithm in [Section 5.2](#52-agentinferencepath-algorithm).

#### 5.1.1 Inference Preconditions

- `INFERENCE_MODEL` is set to a non-empty string.
- The MCP gateway URL (`PMA_MCP_GATEWAY_URL` or equivalent) may or may not be set.

### 5.2 `AgentInferencePath` Algorithm

<a id="algo-cynai-agents-agentinferencepath"></a>

1. Resolve `record = lookupModelCapability(INFERENCE_MODEL)`. <a id="algo-cynai-agents-agentinferencepath-step-1"></a>
   - Match against the catalogue in [Section 3](#3-model-catalogue) using longest-prefix
     matching on the lowercased model name.
   - Apply the small-model rule before prefix matching.
   - If no match, construct a fallback record: `tool_protocol=openai_functions`,
     `thinking=none`, `provider=unknown`.

2. If `record.tool_protocol == direct` OR the MCP gateway URL is empty: <a id="algo-cynai-agents-agentinferencepath-step-2"></a>
   - Use the direct generation path (`callInference` / single-pass HTTP call).
   - Skip steps 3-5.

3. Construct the LLM client using the base URL and HTTP transport appropriate for the <a id="algo-cynai-agents-agentinferencepath-step-3"></a>
   deployment (UDS or TCP; see `resolveInferenceClient`).

4. Select agent executor: <a id="algo-cynai-agents-agentinferencepath-step-4"></a>
   - If `record.tool_protocol == openai_functions`: use `OpenAIFunctionsAgent`.
   - If `record.tool_protocol == react`: use `OneShotAgent`.

5. Execute the agent loop. <a id="algo-cynai-agents-agentinferencepath-step-5"></a>
   - On `unable to parse agent output` or equivalent parse error from `openai_functions`:
     log a warning and fall back to step 2 (direct generation) for this request only.
     Do not mutate the cached record.

6. Strip thinking blocks from the final output string using the rule for <a id="algo-cynai-agents-agentinferencepath-step-6"></a>
   `record.thinking` (see [Section 6](#6-thinking-block-handling)).

7. Return the stripped, trimmed output string. <a id="algo-cynai-agents-agentinferencepath-step-7"></a>
   If the result is empty after stripping, return an error indicating empty output.

---

## 6. Thinking-Block Handling

This section defines how agents detect and strip internal chain-of-thought blocks from model output.

### 6.1 `ThinkingBlockStrip` Rule

- Spec ID: `CYNAI.AGENTS.ThinkingBlockStrip` <a id="spec-cynai-agents-thinkingblockstrip"></a>
- Status: draft

Thinking blocks are internal chain-of-thought content emitted by reasoning models.
Agents must strip them from output before returning content to callers, because:

- They are not user-visible answers.
- They can be arbitrarily large and would inflate response sizes.
- They may contain sensitive intermediate reasoning the system should not expose.

#### 6.1.1 `ThinkingBlockStrip` Behaviour

The stripping function takes a string `s` and a `thinking` enum value and returns a
new string with all matching blocks removed, then whitespace-trimmed.

Stripping rules by enum value:

- `none`: Return `s` unchanged (no allocation required).
- `xml_think`: Remove all occurrences of `<think>…</think>` where `…` is any content
  including newlines (equivalent to the regular expression `(?s)<think>.*?</think>`).
  This covers Qwen3, DeepSeek R1, and any Ollama model that uses the same convention.
- `anthropic_think`: Remove `<thinking>…</thinking>` blocks.
  Anthropic's API may
  deliver these in streaming chunks; implementations must buffer until the closing tag
  is seen before discarding.
    For non-streaming (single-call) paths, apply the same
  regex pattern `(?s)<thinking>.*?</thinking>`.
- `openai_think`: OpenAI o-series reasoning content is delivered in a separate
  `reasoning_content` field, not in the `content` field.
    Implementations that use the
  OpenAI SDK will not see thinking content in the returned text.
    If the raw content
  string contains `<thinking>…</thinking>` (e.g. from a proxy that merges fields),
  strip it the same way as `anthropic_think`.
- `google_think`: Apply `(?s)<thinking>.*?</thinking>` to the assembled response content.
- `xai_think`: Apply `(?s)<thinking>.*?</thinking>` to the assembled response content.

#### 6.1.2 `ThinkingBlockStrip` Error Semantics

- The stripping function must not return an error; stripping is always best-effort.
- If a block is unterminated (opening tag without closing tag), leave the content
  from the opening tag to end-of-string in place and log a debug warning.
- Apply all stripping rules before whitespace trimming.

#### 6.1.3 `ThinkingBlockStrip` Reference - Go

<a id="ref-go-cynai-agents-thinkingblockstrip"></a>

```go
// StripThinkingBlocks removes model-internal chain-of-thought blocks from s.
// thinking is one of the ThinkingKind constants.
func StripThinkingBlocks(s string, thinking ThinkingKind) string
```

---

## 7. Tool-Calling Protocol Selection

This section maps the `tool_protocol` field to concrete langchaingo agent types and wire-level interaction patterns.

### 7.1 `ToolCallProtocol` Rule

- Spec ID: `CYNAI.AGENTS.ToolCallProtocol` <a id="spec-cynai-agents-toolcallprotocol"></a>
- Status: draft

The tool-calling protocol is determined solely by `ModelCapabilityRecord.tool_protocol`.
The following table maps the value to the concrete langchaingo agent type and Ollama/API
interaction pattern.

- **`tool_protocol`:** `openai_functions`
  - langchaingo agent: `agents.NewOpenAIFunctionsAgent`
  - ollama interaction: `llms.WithFunctions(…)` call option; Ollama maps to `<tool_call>` XML for local models
  - notes: Primary path for all capable models
- **`tool_protocol`:** `react`
  - langchaingo agent: `agents.NewOneShotAgent`
  - ollama interaction: Standard `GenerateFromSinglePrompt`
  - notes: Reserved; not assigned to any current model.
    Use only for models explicitly validated to produce `Thought/Action/Final Answer`.
- **`tool_protocol`:** `direct`
  - langchaingo agent: None (no executor)
  - ollama interaction: Direct HTTP POST to `/api/chat` with `stream: false`
  - notes: Small models, models without tool support, and fallback

#### 7.1.1 `ToolCallProtocol` `openai_functions` Requirements

When `tool_protocol == openai_functions`:

- The agent MUST pass all registered tools via `llms.WithFunctions`.
- The agent MUST use `OpenAIFunctionsAgent.ParseOutput` to interpret the LLM response.
- On a parse error (no tool-call choice and empty content), the agent MUST fall back to
  `direct` for the current request (see
  [Section 5.2 step 5](#algo-cynai-agents-agentinferencepath-step-5)).
- The agent MUST NOT interpret text content as ReAct format when `tool_protocol` is
  `openai_functions`.

#### 7.1.2 `ToolCallProtocol` `direct` Requirements

When `tool_protocol == direct`:

- The agent MUST call the Ollama `/api/chat` endpoint (or equivalent) directly with
  `stream: false`.
- The agent MUST NOT pass tool definitions in the request.
- The agent MUST return the `message.content` field of the response as the output.

---

## 8. Output Normalisation

This section defines the sequence of transformations applied to the raw LLM output before returning it to callers.

### 8.1 `AgentOutputNormalise` Rule

- Spec ID: `CYNAI.AGENTS.AgentOutputNormalise` <a id="spec-cynai-agents-agentoutputnormalise"></a>
- Status: draft

After the inference path completes and thinking blocks are stripped, all agents apply
these normalisation steps in order before returning the final content string:

1. **Strip thinking blocks** per `record.thinking` (see [Section 6](#6-thinking-block-handling)).
2. **Trim** leading and trailing ASCII whitespace (spaces, tabs, newlines, carriage returns).
3. **Empty-output check**: if the result is an empty string, return an error with code
   `empty_agent_output`.
    Do not return an empty success response to callers.
4. **No further mutation**: do not truncate, summarise, or reformat the content.
   Callers receive the raw trimmed model output.

The normalisation function is identical for PMA and SBA and must be implemented once in
the shared `agents/internal` package (see [Section 9](#9-shared-tooling-contract-pma-and-sba)).

---

## 9. Shared Tooling Contract (PMA and SBA)

This section defines the shared Go package and interface that both PMA and SBA must use for model capability lookup and output handling.

### 9.1 `AgentModelTooling` Interface

- Spec ID: `CYNAI.AGENTS.AgentModelTooling` <a id="spec-cynai-agents-agentmodeltooling"></a>
- Status: draft

To avoid code duplication, PMA (`agents/internal/pma`) and SBA (`agents/internal/sba`)
must share the following functions from a new package `agents/internal/modelutil`:

- `LookupModelCapability(name string) ModelCapabilityRecord`
  Returns the capability record for the given model name using the catalogue and fallback
  rules in [Section 3](#3-model-catalogue).

- `StripThinkingBlocks(s string, thinking ThinkingKind) string`
  Strips thinking blocks as defined in [Section 6](#6-thinking-block-handling).

- `NormaliseOutput(s string, thinking ThinkingKind) (string, error)`
  Applies full output normalisation per [Section 8](#8-output-normalisation).

- `SelectInferencePath(record ModelCapabilityRecord, mcpGatewayURL string) InferencePath`
  Returns the path enum (`FunctionsAgent`, `ReactAgent`, `Direct`) per the algorithm
  in [Section 5.2](#52-agentinferencepath-algorithm).

#### 9.1.1 `AgentModelTooling` Migration Plan

Current state:

- PMA has `isCapableModel` and `capableModels` in `agents/internal/pma/langchain.go`.
- SBA does not yet have equivalent logic.

Target state after promotion of this spec:

1. Create `agents/internal/modelutil/` package with the functions above.
2. Replace `isCapableModel` in PMA with `modelutil.LookupModelCapability` +
   `modelutil.SelectInferencePath`.
3. Wire SBA's inference path through the same `modelutil` package.
4. Delete the `capableModels` slice and `smallModelSuffixes` slice from PMA once
   `modelutil` is the sole source of truth.

#### 9.1.2 `AgentModelTooling` Reference - Go

<a id="ref-go-cynai-agents-agentmodeltooling"></a>

```go
package modelutil

// ThinkingKind enumerates the recognised thinking-block conventions.
type ThinkingKind string

const (
    ThinkingNone         ThinkingKind = "none"
    ThinkingXMLThink     ThinkingKind = "xml_think"
    ThinkingAnthropicTag ThinkingKind = "anthropic_think"
    ThinkingOpenAI       ThinkingKind = "openai_think"
    ThinkingGoogle       ThinkingKind = "google_think"
    ThinkingXAI          ThinkingKind = "xai_think"
)

// InferencePath enumerates the agent execution strategies.
type InferencePath int

const (
    PathDirect         InferencePath = iota // callInference / single-pass HTTP
    PathFunctionsAgent                      // OpenAIFunctionsAgent
    PathReactAgent                          // OneShotAgent (ReAct)
)

// ToolProtocol enumerates the tool-calling wire formats.
type ToolProtocol string

const (
    ToolProtocolOpenAIFunctions ToolProtocol = "openai_functions"
    ToolProtocolReAct           ToolProtocol = "react"
    ToolProtocolDirect          ToolProtocol = "direct"
)

// ModelCapabilityRecord is the resolved capability for one model name.
type ModelCapabilityRecord struct {
    ModelName          string
    ToolProtocol       ToolProtocol
    Thinking           ThinkingKind
    MaxContextTokens   int
    SupportsVision     bool
    SupportsJSONMode   bool
    Provider           string
}

func LookupModelCapability(name string) ModelCapabilityRecord
func StripThinkingBlocks(s string, thinking ThinkingKind) string
func NormaliseOutput(s string, thinking ThinkingKind) (string, error)
func SelectInferencePath(record ModelCapabilityRecord, mcpGatewayURL string) InferencePath
```

---

## 10. References

- [`docs/tech_specs/project_manager_agent.md`](../tech_specs/project_manager_agent.md)
- [`docs/tech_specs/project_analyst_agent.md`](../tech_specs/project_analyst_agent.md)
- [`docs/tech_specs/model_management.md`](../tech_specs/model_management.md)
- [`docs/tech_specs/external_model_routing.md`](../tech_specs/external_model_routing.md)
- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) -
  `CYNAI.ORCHES.Operation.SelectProjectManagerModel`
- [`docs/draft_specs/model_capabilities_update_blob_spec_proposal.md`](model_capabilities_update_blob_spec_proposal.md)
- [`agents/internal/pma/langchain.go`](../../agents/internal/pma/langchain.go) - current `isCapableModel` implementation
