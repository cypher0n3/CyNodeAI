# Review Report 4: Cynork CLI/TUI

- [1 Summary](#1-summary)
- [2 Specification Compliance](#2-specification-compliance)
- [3 Architectural Issues](#3-architectural-issues)
- [4 Concurrency and Safety](#4-concurrency-and-safety)
- [5 Security Risks](#5-security-risks)
- [6 Performance Concerns](#6-performance-concerns)
- [7 Maintainability Issues](#7-maintainability-issues)
- [8 Recommended Actions](#8-recommended-actions)

## 1 Summary

This report covers the `cynork/` module: 89 Go files across CLI commands, TUI (Bubble Tea), chat session/transport, gateway client, and config.

The cynork client delivers a working interactive TUI with SSE streaming, slash commands, auth recovery, thread management, and clipboard support.
The CLI provides task, auth, skills, preferences, and node management commands via Cobra.
However, the review surfaces **1 critical**, **6 high**, **7 medium**, and **8 low** severity findings.

The most impactful gaps are:

- **Critical data race** in `runEnsureThread` -- writes `Session.CurrentThreadID` from a `tea.Cmd` goroutine while `View()` reads it concurrently.
- **Synchronous network I/O in `Update()`** blocks the entire Bubble Tea event loop, freezing the UI during `/thread new`, `/thread switch`, and stream recovery health checks.
- **No HTTP client timeout** -- `http.DefaultClient` with zero timeout; all non-streaming requests can hang indefinitely.
- **No `context.Context` on non-streaming gateway methods** -- requests are uncancelable.
- **Exported mutable `Client.Token` and `Client.BaseURL`** with no synchronization; TUI goroutines read while auth refresh writes.

## 2 Specification Compliance

Gaps identified against requirements and technical specifications.

### 2.1 High-Severity Gaps

- ⚠️ **REQ-CLIENT-0171 -- `/model` slash command rejects all models except `cynodeai.pm`.**
  `runSlashModel` (`chat_slash.go:298-317`) hard-rejects any model that is not `gateway.ModelProjectManager`.
  The spec states: "The CLI MUST support selecting an OpenAI model identifier for chat completions."
  The `--model` CLI flag on `cynork chat` accepts any string (non-interactive path works), but the interactive slash command is non-compliant.

### 2.2 Medium-Severity Gaps

- **REQ-CLIENT-0216 -- Thinking content stored but never rendered.**
  Thinking content is stored in `Transcript[].Parts` but `renderScrollbackContent` only iterates `m.Scrollback` strings, not the structured transcript.
  `scrollbackRenderSignature()` (`view_render.go:76-94`) does not include `ShowThinking` in its hash, so toggling the flag does not invalidate the cached render.

- **REQ-CLIENT-0217 -- Tool-call content stored but never surfaced.**
  `appendTranscriptToolCall` (`transcript_sync.go:55-73`) stores tool calls in transcript parts, but the scrollback view never renders them.
  `ShowToolOutput` toggle exists without visible effect.

- **REQ-CLIENT-0218 -- Per-iteration scoped overwrite not implemented.**
  `iteration_start` SSE events update `StreamingState.Phase` but per-iteration overwrite is absent.
  Amendment handling replaces the entire `streamBuf` (per-turn), not per-iteration.

- **REQ-CLIENT-0107 -- No mTLS or pinned CA bundle support.**
  `NewClient` uses `http.DefaultClient` with no TLS customization.
  No flag or config exists for custom CA bundles or client certificates.

### 2.3 Low-Severity Gaps

- **REQ-CLIENT-0190 -- Compliant.**
  TUI correctly defers thread initialization when a token is present, and sets `OpenLoginFormOnInit` when no token.

- **Bug 4 partially addressed.** `handleEnterKey` (`model.go:598`) allows slash and shell commands during streaming.
  The full queue/Ctrl+Enter spec is still unimplemented.

- **REQ-CLIENT-0221 partial.**
  `secretutil.RunWithSecret` protects thinking and tool-call parts, but `m.streamBuf` (visible-text buffer) is not wrapped.

## 3 Architectural Issues

Structural and design concerns in the cynork codebase.

### 3.1 TUI Architecture

- ⚠️ **Dual scrollback model.**
  The TUI maintains two parallel representations: `m.Scrollback []string` (flat text for rendering) and `m.Transcript []TranscriptTurn` (structured turns with parts).
  Updated independently and can drift.
  Scrollback is rendered but transcript holds richer data (thinking, tool calls) that scrollback never consumes.
  This blocks REQ-CLIENT-0216/0217/0218 compliance.

- **Dead code.** `shellInteractiveCmd` (`slash.go:235-249`), `captureToLines` (`slash.go:650-669`), and `SessionState` struct (`state.go:118-132`) are unused.

### 3.2 CLI Architecture

- ⚠️ **~20 package-level mutable `var` declarations in `cmd/`.**
  Flag values, config, and control flags defined as package-level globals (`root.go:21-37`, `task.go:21-43`, `chat.go:23-28`).
  Prevents safe concurrent testing, leaks state across `rootCmd.Execute()` calls in shell REPL.

- **`runCynorkSubcommand` delegates to child process, losing in-memory token state.**
  Slash commands like `/task`, `/status`, `/nodes` execute a child `cynork` process that re-reads config.
  If the parent's in-memory token was obtained by refresh but not yet persisted, the child uses a stale token.

### 3.3 Gateway Client

- **`Config` struct retains `yaml:"token"` and `json:"token"` tags.**
  While `Save()` uses `persistedConfig` (excluding tokens), any future `json.Marshal(cfg)` call would serialize tokens.
  JSON tags serve no safe purpose; latent REQ-CLIENT-0103 / REQ-CLIENT-0149 violation risk.

## 4 Concurrency and Safety

Data races, synchronization, and thread safety issues.

### 4.1 Critical Severity

- ❌ **Data race in `runEnsureThread`.**
  `model.go:774-791`: writes `m.Session.CurrentThreadID` from a `tea.Cmd` goroutine via `tryResumeThreadFromCache` and `Session.EnsureThread`.
  `View()` concurrently reads `m.Session.CurrentThreadID` for the status bar.
  Fix: move all state mutations into `applyEnsureThreadResult` (runs in Update); have `runEnsureThread` return data without writing model fields.

### 4.2 High Severity Concurrency

- ⚠️ **Synchronous network I/O in `Update()` -- UI freezes.**
  `applyStreamRecoveryTick` (`model_stream_recovery.go:99`) calls `m.Session.Client.Health()` synchronously.
  `/thread new` (`model_thread_commands.go:41-49`) calls `m.Session.NewThread()` synchronously.
  `/thread switch` (`model_thread_commands.go:52-67`) calls `m.Session.ResolveThreadSelector()` synchronously.
  All block the Bubble Tea event loop.

- ⚠️ **`Client` struct has exported mutable fields with no synchronization.**
  `client.go:32-36`: `Token`, `BaseURL`, `HTTPClient` are exported and read/written from multiple goroutines (health polling, streaming, auth refresh).

### 4.3 Medium Severity Concurrency

- **Multiple `tea.Cmd` closures capture `m` and access `m.Session` from goroutines.**
  `slashModelsCmd` (`slash.go:311`), `slashStatusCmd` (`slash.go:518-521`), `slashSubprocCmd` (`slash.go:623-634`), `gatewayHealthCheckCmd` (`status_indicator.go:65-68`).
  Concurrent access with `SetClient` writes from `applyLoginResult`.
  Should capture `session := m.Session` before the closure.

- **`readChatSSEStream` discards context parameter** (`client.go:531`).
  Explicitly ignored with `_`; checking `ctx.Done()` in scan loop would provide faster cancellation.

## 5 Security Risks

Vulnerabilities organized by severity level.

### 5.1 High Severity Security

- ⚠️ **No HTTP client timeout.** `NewClient` (`client.go:39-44`) assigns `http.DefaultClient` with zero timeout.
  All non-streaming methods can hang indefinitely.

- ⚠️ **No `context.Context` on non-streaming gateway methods.**
  `doRequest` (`client_http.go:15-36`) uses `http.NewRequest` (no context) instead of `http.NewRequestWithContext`.
  All non-streaming methods are uncancelable.

### 5.2 Medium Severity Security

- **`readPasswordFromStdin` uses unbounded `io.ReadAll`** (`auth.go:174-178`).
- **`GetBytes`, `PostBytes`, `PutBytes`, `DeleteBytes` use unbounded `io.ReadAll`** on response body (`client.go:861-926`).
- **`Health()` uses unbounded `io.ReadAll`** (`client.go:101-104`).
- **`runSkillsLoad` and `runSkillsUpdate` read files without size limit** (`skills.go:75,103`).
- **Path-injection risk in stub commands** (`stub_helpers.go:37`, `skills.go:68`, `project.go:42`) -- user-provided IDs not URL-escaped.
- **`slashSubprocCmd` injects `CYNORK_TOKEN` into subprocess environment** (`slash.go:631-634`), readable via `/proc/<pid>/environ`.

### 5.3 Low Severity Security

- `/connect` persists any string as gateway URL without `url.Parse` validation.
- Session cache stores tokens as plaintext JSON when OS keyring unavailable; file permissions after rename not verified to be 0600.
- `/auth logout` clears tokens but does not clear `Session.CurrentThreadID` -- subsequent login as different user may reuse stale thread.

## 6 Performance Concerns

- **`task result --wait` poll loop has no context/signal handling** (`task.go:418-434`).
  Uses bare `time.Sleep` instead of `signal.NotifyContext`; Ctrl-C kills ungracefully.

- **`scrollbackRenderSignature` hashes entire `Scrollback` slice on every `View()`** (`view_render.go:76-94`).
  O(n) per frame; long sessions become sluggish.

- **`filteredSlashCommands()` called multiple times per key event** without caching.

- **`strings.Split(m.Input, "\n")` called independently by 4 cursor functions** per cursor move.

- **`formatChatResponse` creates new `glamour.TermRenderer` per message** (`chat.go:62-66`).

- **SSE scanner uses default 64KB buffer** (`client.go:535`); large events could exceed `bufio.MaxScanTokenSize`.

## 7 Maintainability Issues

- **Inconsistent async patterns.**
  `/thread list` and `/thread rename` use proper async `tea.Cmd`; `/thread new` and `/thread switch` block synchronously.

- **Duplicated URL construction** across 8+ client methods that bypass `doRequest`.

- **`shell.go` REPL reuses `rootCmd` with `SetArgs`** (`shell.go:48-51`); per-command flag variables retain values from previous iterations.

- **`parseArgs` has simplistic quoting model** (`shell.go:55-79`): no backslash escaping, no single-quote support.

- **`exit.CodeOf` uses type assertion instead of `errors.As`** (`exit.go:62-69`); wrapped `*exit.Error` returns code 1 instead of intended code.

## 8 Recommended Actions

Remediation items organized by priority tier.

### 8.1 P0 -- Immediate (Correctness)

1. **Fix `runEnsureThread` data race.**
   Move all `Session.CurrentThreadID` mutations to `applyEnsureThreadResult` in Update.
   Have the goroutine return resolved thread ID as part of the result message.
2. **Add `context.Context` to `doRequest` and all non-streaming client methods.**
   Use `http.NewRequestWithContext`.
3. **Set a default HTTP client timeout** (e.g., 30s for non-streaming).
4. **Make `Client.Token` and `Client.BaseURL` unexported** with synchronized read/write methods.

### 8.2 P1 -- Short-Term (High-Severity Issues)

1. **Move synchronous network I/O to async `tea.Cmd`.**
   Fix `/thread new`, `/thread switch`, and `applyStreamRecoveryTick`.
2. **Fix `/model` slash command** to accept any valid model identifier per REQ-CLIENT-0171.
3. **Add `io.LimitReader`** to all `io.ReadAll` calls: `Health`, `GetBytes`, `PostBytes`, `PutBytes`, `DeleteBytes`, `readPasswordFromStdin`.
4. **URL-escape user-provided IDs** in stub commands, skills, and project commands.

### 8.3 P2 -- Planned (Medium-Severity Improvements)

1. **Unify the dual scrollback model.**
   Make `Transcript` the single source of truth and derive scrollback strings from it, enabling thinking/tool rendering.
2. **Apply `session := m.Session` capture pattern** consistently across all `tea.Cmd` closures.
3. **Remove JSON struct tags** from `Config.Token` and `Config.RefreshToken`.
4. **Add `signal.NotifyContext`** to `task result --wait` poll loop.
5. **Reduce `cmd/` global mutable state** -- pass command context struct through `RunE` closures.

### 8.4 P3 -- Longer-Term (Maintenance)

1. Remove dead code (`shellInteractiveCmd`, `captureToLines`, `SessionState`).
2. Cache `filteredSlashCommands()` per Update cycle.
3. Use `errors.As` in `exit.CodeOf`.
4. Add mTLS / custom CA support for enterprise deployments.
5. Consolidate URL construction into `doRequest` for all client methods.
