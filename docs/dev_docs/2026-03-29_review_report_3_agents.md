# Review Report 3: Agents (PMA and SBA)

- [1 Summary](#1-summary)
- [2 Specification Compliance](#2-specification-compliance)
- [3 Architectural Issues](#3-architectural-issues)
- [4 Concurrency and Safety](#4-concurrency-and-safety)
- [5 Security Risks](#5-security-risks)
- [6 Performance Concerns](#6-performance-concerns)
- [7 Maintainability Issues](#7-maintainability-issues)
- [8 Recommended Actions](#8-recommended-actions)

## 1 Summary

This report covers the `agents/` module: 44 Go files across 2 binaries (`cynode-pma`, `cynode-sba`) and ~3 internal packages (`pma`, `sba`, `mcpclient`).

The PMA delivers working interactive chat with streaming NDJSON, thinking/visible separation, and langchaingo tool integration.
The SBA delivers working agent-loop execution with local tools, MCP tools via gateway, lifecycle callbacks, and structured result output.
However, the review surfaces **5 critical**, **6 high**, **8 medium**, and **7 low** severity findings.

The most impactful gaps are:

- **Three PMA streaming spec features are entirely unimplemented**: model keep-warm (REQ-PMAGNT-0129), opportunistic secret scan (REQ-PMAGNT-0125), and overwrite events (REQ-PMAGNT-0124).
- **HTTP `WriteTimeout` (120s) < inference timeout (300s)** -- streaming responses are silently killed mid-stream.
- **SBA prompt construction violates REQ-SBAGNT-0113** -- persona, skills, and preferences are omitted; context ordering is wrong.
- **Multiple unbounded reads** in both PMA and SBA can OOM the process.
- **HTTP response bodies leaked** in SBA lifecycle callbacks.

## 2 Specification Compliance

Gaps identified against requirements and technical specifications.

### 2.1 Critical Gaps

- ❌ **REQ-PMAGNT-0129 -- Model keep-warm entirely unimplemented.**
  The spec requires PMA to load the model on startup and periodically call the inference backend (default 300s interval) to keep it resident.
  No goroutine, ticker, or keep-warm call exists anywhere in the PMA codebase.

- ❌ **REQ-PMAGNT-0125 -- Opportunistic secret scan entirely unimplemented.**
  The spec requires that after each langchaingo iteration, PMA scans accumulated visible text, thinking, and tool-call content for secrets and emits an overwrite event if detected.
  No scanning code exists.
  `appendStreamBufferSecure` (`streambuf.go:7-13`) wraps buffer ops in `secretutil.RunWithSecret`, which addresses REQ-PMAGNT-0126 (buffer handling), not secret scanning.

- ❌ **REQ-PMAGNT-0124 -- Overwrite NDJSON events never emitted.**
  Helper functions `iterationOverwriteReplace` and `turnOverwriteReplace` exist (`streaming_fsm.go:203-216`) but are dead code -- never called from any production path.
  No `{"overwrite": ...}` JSON event is ever encoded on the NDJSON stream.

- ❌ **REQ-SBAGNT-0113 -- SBA prompt construction violates required context ordering.**
  `buildUserPrompt` (`agent.go:396-460`) constructs prompts in the order: time remaining, task, baseline, project, requirements, acceptance.
  The spec requires: persona, baseline, project, task, requirements, preferences, additional context, skills, runtime.
  Persona is missing entirely, skills are missing, preferences are missing, and baseline/task are swapped.

- ❌ **SBA `ContextSpec` lacks persona fields.**
  `go_shared_libs/contracts/sbajob/sbajob.go:57-69`: the `ContextSpec` struct has no `PersonaTitle`/`PersonaDescription` fields.
  REQ-SBAGNT-0113 requires inline `persona.title` and `persona.description` as the first context block.

### 2.2 High-Severity Gaps

- ⚠️ **SBA `ContextSpec.Preferences` field exists but is never rendered into the prompt (REQ-SBAGNT-0111).**
  `appendContextBlock` (`agent.go:420-442`) reads `TaskContext`, `BaselineContext`, `ProjectContext`, `Requirements`, `AcceptanceCriteria`, `AdditionalContext` but ignores `Preferences` and `SkillIDs`/`Skills`.

- ⚠️ **PMA `WriteTimeout` (120s) < inference timeout (300s) -- streaming responses silently killed.**
  `main.go:93` sets `WriteTimeout: 120 * time.Second`.
  `pmaLangchainCompletionTimeout` is `300 * time.Second` (`chat.go:30`).
  For streaming responses, Go's `net/http` closes the connection when `WriteTimeout` expires.
  Any inference taking longer than 120s has its streaming response truncated mid-stream with no error to the client.

- ⚠️ **SBA lifecycle HTTP response bodies never closed.**
  `NotifyInProgress` (`lifecycle.go:53`) and `NotifyCompletion` (`lifecycle.go:71`) discard the HTTP response without closing the body.
  Over the lifecycle of many jobs, this leaks TCP connections.

- ⚠️ **SBA `applyUnifiedDiffStep` uses `exec.Command` without context -- timeout not enforced.**
  `runner.go:165-176`: uses `exec.Command("patch", ...)` instead of `exec.CommandContext`.
  If the patch command hangs, there is no timeout.

### 2.3 Medium-Severity Gaps

- **REQ-PMAGNT-0106 partial -- MCP client re-instantiated per request, not per handler lifetime.**
  `canStreamCompletion` (`chat.go:95`), `streamTryLangchainNDJSON` (`chat.go:142`), and `getCompletionContent` (`chat.go:474`) each call `NewMCPClient()`, allocating a new `http.Client`.
  The gateway URL is stable per-process and should be resolved once at construction.

- **SBA `isSmallModel` defaults unknown models to direct generation.**
  `agent.go:52-70`: any model not matching known `capablePrefixes` defaults to `true` (small/direct).
  New capable models (e.g., `gemma3:27b`) would lose tool-use capability.
  Default should be `false` (capable) with a blocklist for known-small models.

## 3 Architectural Issues

Structural and design concerns in the agents codebase.

### 3.1 PMA Architecture

- ⚠️ **`testLLMForCompletion` is a package-level mutable var** (`langchain.go:110`).
  Used for test injection; not goroutine-safe for parallel test subtests.

- **`os.Getenv("INFERENCE_MODEL")` called 4+ times per request path.**
  Model name is read in `canStreamCompletion`, `streamTryLangchainNDJSON`, `getCompletionContent`, and `runCompletionWithLangchain`.
  Same for `OLLAMA_BASE_URL` via `resolveOllamaConfig`.
  Should be resolved once at handler creation.

- **Fat handler closure.** `ChatCompletionHandler` (`chat.go:57-88`) orchestrates streaming vs non-streaming, context detachment, and retries.
  Should be decomposed into a service-layer struct.

### 3.2 SBA Architecture

- **Double-marshal in tool wrappers.**
  `agent_tools.go:86-108`: tool unmarshal -> re-marshal -> step unmarshal adds latency, allocations, and silently ignores `json.Marshal` error at line 98.
  Step functions should accept parsed args directly.

- **`ErrExtNetRequired` is an exported mutable variable** (`mcp_tools.go:61`).
  Can be overwritten; should be unexported with an `IsExtNetRequired()` accessor.

- **Error detection by string prefix convention.**
  `processStepsToResult` (`agent.go:345`): `strings.HasPrefix(s.Observation, "error: ")` misclassifies steps whose output legitimately starts with "error: ".

### 3.3 Shared MCP Client

- **`mcpclient/client.go:132` and `client.go:198` read response body without limit.**
  `buf.ReadFrom(resp.Body)` has no `io.LimitReader`.
  Both direct and proxy paths are affected.

## 4 Concurrency and Safety

- ⚠️ **`context.WithoutCancel` lets inference outlive the HTTP response.**
  `chat.go:76`: `detached := context.WithoutCancel(r.Context())`.
  If client disconnects at T=10s, inference continues for up to 300s.
  Combined with retry, this could be 600s of orphaned GPU work.

- **`streamingLLM.enc`/`flusher` writes not mutex-protected** (`streaming.go:58-79`).
  The `streamFn` closure calls `enc.Encode` and `flusher.Flush` outside `s.mu` lock.
  Safe today (single-threaded langchaingo executor) but fragile.

- **`ToolEnv.ConstraintError` is a mutable pointer stored in context** (`agent_tools.go:19`).
  Benign today with single-threaded agent loop, but would race under concurrent tool execution.

## 5 Security Risks

Vulnerabilities organized by severity level.

### 5.1 High Severity

- ⚠️ **PMA: No request body limit.**
  `chat.go:64`: `json.NewDecoder(r.Body).Decode(&req)` reads full body with no size limit.

- ⚠️ **SBA: `readFileStep` reads entire file into memory before truncation.**
  `runner.go:124-146`: `os.ReadFile(full)` allocates the full file size before `capString` truncates.
  A 4 GB file in `/workspace` causes OOM.

- ⚠️ **SBA: Unbounded stdin read.**
  `main.go:47`: `io.ReadAll(os.Stdin)` with no `io.LimitReader`.

### 5.2 Medium Severity

- **PMA: `readInferenceOllamaChatBody` uses unbounded `io.ReadAll`** (`chat.go:776`).

- **SBA: `resolveWorkspacePath` does not follow symlinks** (`runner.go:375-390`).
  No `filepath.EvalSymlinks` call; symlink inside `/workspace` pointing to `/etc` passes the string-prefix check.
  Container is the security boundary, but the check is misleading if it exists.

- **SBA: `runDirectGeneration` decodes Ollama response without size limit** (`agent.go:241`).

### 5.3 Low Severity

- **PMA: No role validation on `messages[].role`** (`chat.go:64-72`).
  Values like `"system"` from a client could inject system-level prompts.

- **PMA: Hardcoded `openai.WithToken("ollama")`** (`langchain.go:507`).

## 6 Performance Concerns

- **PMA: `NewMCPClient()` and `http.Client` created per request.**
  Defeats connection pooling across `canStreamCompletion`, `streamTryLangchainNDJSON`, `getCompletionContent`.

- **PMA: String concatenation in FSM hot loop.**
  `streaming_fsm.go:46`: `c.pending += chunk` creates O(n^2) copies for long responses.
  Should use `strings.Builder` or `[]byte` buffer.

- **PMA: 64KB buffer allocated per streaming call** (`chat.go:249`).
  Could use `sync.Pool` for high throughput.

- **SBA: Double JSON marshal/unmarshal in tool wrappers** adds unnecessary allocation and latency.

## 7 Maintainability Issues

- **PMA: `iterationOverwriteReplace` and `turnOverwriteReplace` are dead code** (`streaming_fsm.go:203-216`).
  Only called from tests; exist for REQ-PMAGNT-0124 which is unimplemented.

- **PMA: `instructionsLoader` recursion edge case** (`instructions.go:50-51`).
  A directory named `bundle.md/` would pass extension check before `IsDir()` check.
  `IsDir()` should be checked first.

- **PMA: `json.Marshal` error ignored** in `doInferenceRequest` (`chat.go:756`).

- **SBA: `writeResultFailure` is dead code** (`main.go:167-172`).

- **SBA: `failureResult` hardcodes `ProtocolVersion: "1.0"`** (`main.go:140`).
  If protocol moves to "1.1", early failures report wrong version.

- **SBA: Streaming FSM `trailingIncompleteTagPrefix` only checks open tags.**
  `streaming_fsm.go:182-200`: checks for partial prefixes of `xmlThinkOpen` and `toolCallOpen` but not close tags.

## 8 Recommended Actions

Prioritized remediation steps for the findings in this report.

### 8.1 P0 -- Immediate (Spec Compliance and Correctness)

1. **Implement REQ-PMAGNT-0129 (keep-warm).**
   Add a background goroutine with configurable ticker (default 300s) that sends minimal Ollama chat/generate requests.
   Wire into `run()` lifecycle with graceful shutdown.
2. **Implement REQ-PMAGNT-0125 (secret scan).**
   After each langchaingo iteration, scan accumulated buffers for secret patterns.
   Emit overwrite NDJSON events.
3. **Wire overwrite events (REQ-PMAGNT-0124).**
   Connect existing helpers to the streaming pipeline.
   Emit `{"overwrite": {...}}` NDJSON events.
4. **Fix SBA prompt construction.**
   Add persona, skills, and preferences rendering.
   Add persona fields to `ContextSpec`.
   Reorder context blocks per REQ-SBAGNT-0113.
5. **Fix PMA `WriteTimeout`.**
   Set to 0 (disabled) or at minimum 310s to cover inference timeout plus margin.

### 8.2 P1 -- Short-Term (High-Severity Issues)

1. **Close lifecycle response bodies** in `lifecycle.go`.
2. **Add context to `applyUnifiedDiffStep`.**
   Use `exec.CommandContext` for timeout enforcement.
3. **Add body size limits.**
   Wrap PMA `r.Body` with `http.MaxBytesReader`.
   Add `io.LimitReader` to Ollama and MCP client response bodies.
4. **Fix `readFileStep`.**
   Stat file first or use `io.LimitReader` before reading.
5. **Add `io.LimitReader` to SBA stdin** (`main.go:47`).
6. **Invert `isSmallModel` default.**
   Maintain a blocklist of known-small models; default unknown to capable.

### 8.3 P2 -- Planned (Medium-Severity Improvements)

1. **Inject dependencies into PMA handler.**
   Create a `ChatService` struct holding MCP client, model name, inference URL, and logger.
   Eliminate per-request `os.Getenv` and `NewMCPClient()`.
2. **Scope detached context.**
   Use `context.WithoutCancel` only for retry, not primary call.
3. **Replace string concatenation in FSM** with `strings.Builder`.
4. **Add `io.LimitReader` to MCP client** (`client.go:132,198`).
5. **Refactor tool wrappers** to pass parsed args directly to step functions.
6. **Add `filepath.EvalSymlinks`** to `resolveWorkspacePath` or document its non-security nature.

### 8.4 P3 -- Longer-Term (Maintenance)

1. Remove dead code (`writeResultFailure`, unreachable overwrite helpers if wired).
2. Fix instructions loader `IsDir()` check ordering.
3. Add mutex protection for `streamingLLM.enc`/`flusher` writes.
4. Replace string-prefix error detection with typed error channel in SBA.
5. Make `ErrExtNetRequired` unexported.
