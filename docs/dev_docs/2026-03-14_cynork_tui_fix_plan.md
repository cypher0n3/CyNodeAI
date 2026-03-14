# Cynork TUI/Chat Fix Plan

- [Goal](#goal)
- [References](#references)
- [Constraints](#constraints)
- [Execution Plan](#execution-plan)
- [Execution Order](#execution-order)
- [Verification Checklist](#verification-checklist)

## Goal

Fix the two failing BDD scenarios, bring all packages to at least 90% coverage, implement undefined BDD steps and missing slash commands, close spec-compliance gaps (thread commands, resume-thread, transcript rendering, generation state, connection recovery), replace project stubs with gateway calls, and remove lint suppressions (except the one owner-approved exception).
Outcome: `just ci` passes and cynork TUI/chat behavior matches the technical specifications.

## References

- `docs/requirements/client.md` (REQ-CLIENT-* for chat, TUI, slash, coverage)
- `docs/tech_specs/cynork_tui.md` (TUI layout, transcript, generation state, connection recovery)
- `docs/tech_specs/cynork_tui_slash_commands.md` (slash command algorithms)
- `docs/tech_specs/cli_management_app_commands_chat.md` (CliChat, thread controls, flags)
- `docs/tech_specs/cynork_cli.md` (CLI scope, config)
- Implementation: `cynork/cmd/`, `cynork/internal/tui/`, `cynork/internal/chat/`, `cynork/internal/gateway/`, `cynork/_bdd/steps.go`; `orchestrator/internal/pmaclient/`, `orchestrator/internal/handlers/`; `agents/internal/pma/`

## Constraints

- Requirements are source of truth; then tech specs; then implementation.
- Do not proceed to the next task until the current task's Testing validation gate passes.
- Use BDD/TDD: add or update specs and failing tests before implementation; implement smallest change to pass; refactor only after green.
- Coverage is enforced per package (90% minimum) via `just test-go-cover`.
- Only one `//nolint` is allowed: `worker_node/internal/telemetry/sloghandler.go` (gocritic hugeParam, owner-approved).

## Execution Plan

Execute tasks in the order given in [Execution Order](#execution-order).
Each task is self-contained: it has its own Requirements and Specifications, Discovery steps, and Testing gate.
Do not start a later task until the current task's Testing steps pass.

---

### Task 1: Fix the 2 Failing BDD Scenarios

Fix the two BDD scenarios that currently fail so the BDD suite has zero failures before expanding step definitions or coverage work.

#### Task 1 Requirements and Specifications

- `docs/requirements/client.md` (REQ-CLIENT-0171, REQ-CLIENT-0173)
- `docs/tech_specs/cli_management_app_commands_chat.md` (CliChat `--model`; chat command flags)
- `docs/tech_specs/cynork_tui_slash_commands.md` (ProjectSlashCommands algorithm: bare id must set session project)

#### Discovery (Task 1) Steps

- [ ] Read the requirements and specs relevant to this task (see Task 1 Requirements and Specifications above).
- [ ] Run `just test-bdd` and capture the two failing scenario names and locations (confirm 2 failed, 72 undefined).
- [ ] Inspect `cynork/cmd/chat.go` for missing `--model` flag and `cmd/chat_slash.go:runSlashProjectDelegated` for bare-id handling.

#### Red (Task 1)

- [ ] Confirm the following scenarios fail for the right reason (no guesswork):
  - [ ] `/model change does not mutate system settings or user preferences` (`features/cynork/cynork_tui_slash_model.feature`, line 37): fails due to `cynork chat` unknown flag `--model`.
  - [ ] `/project with bare id updates context as shorthand` (`features/cynork/cynork_tui_slash_project.feature`, line 45): fails because bare id is delegated to Cobra and usage is printed; session project is not updated.
- [ ] Validation gate: do not proceed until the failing tests prove the gap.

#### Green (Task 1)

- [ ] **1A - Model flag:** In `cynork/cmd/chat.go`, declare `var chatModel string` and add flag in `init()`: `chatCmd.Flags().StringVar(&chatModel, "model", "", "model ID for chat completions")`.
  In `runChat`, set `session.Model = chatModel` after creating the session and before the interactive loop.
- [ ] **1B - Project bare id:** In `cmd/chat_slash.go:runSlashProjectDelegated`, parse `rest` into parts; if empty show current project; if `parts[0]` is `set` call `setChatSessionProject(session, parts[1])`; if `list` or `get` delegate to `runCynorkSubcommandForSlash("project", rest)`; for any other value (bare id) call `setChatSessionProject(session, rest)`.
- [ ] Run `just test-bdd` and confirm both scenarios pass.
- [ ] Validation gate: do not proceed until the targeted tests are green.

#### Refactor (Task 1)

- [ ] Refine implementation without changing behavior (e.g. naming, extract helpers if useful).
  - [ ] Keep all tests green throughout.
- [ ] Re-run `just test-bdd`.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 1)

- [ ] Run `just test-bdd` and confirm 0 failed scenarios for cynork.
- [ ] Run `just test-go-cover` and confirm `cynork/cmd` is not in the failure list (if it was below 90%).
- [ ] Validation gate: do not start Task 2 until all checks for this task pass.

---

### Task 2: Coverage Gaps (All Packages Below 90%)

Bring all seven packages below the 90% threshold to at least 90% coverage using targeted unit tests.
Address in order of gap size (largest first); after each package run `just test-go-cover` and confirm that package is no longer listed before moving to the next.

#### Task 2 Requirements and Specifications

- `docs/requirements/client.md` (coverage and test expectations)
- `docs/tech_specs/cynork_tui.md` (TUI behavior under test)
- Justfile target `test-go-cover` (per-package 90% enforcement)

#### Discovery (Task 2) Steps

- [ ] Read the requirements and specs relevant to this task (see above).
- [ ] Run `just test-go-cover` and capture the full list of packages below 90% (confirm 7 packages).
- [ ] For each package, run `go test -coverprofile=cover.out ./<pkg/...>` in the owning module and `go tool cover -func=cover.out` to list uncovered lines.

#### Red (Task 2)

- [ ] Document the uncovered branches/lines for each of the seven packages (pmaclient, tui, chat, pma, gateway, handlers, cmd).
- [ ] Add or identify unit tests that would cover those branches; run tests and confirm new tests fail or coverage gaps remain before implementation.
- [ ] Validation gate: do not proceed until the coverage gaps are clearly identified and test plan is in place.

#### Green (Task 2)

- [ ] **2A -** `orchestrator/internal/pmaclient`: Add unit tests for client methods, context propagation, cancellation until package reaches 90%.
- [ ] **2B -** `cynork/internal/tui`: Add tests for View branches, login form flows, `applyStreamDelta`, `applyEnsureThreadResult`, `applyOpenLoginForm`/`applyLoginResult`, `scheduleNextDelta`/`readNextDelta` until 90%.
- [ ] **2C -** `cynork/internal/chat`: Add tests for EnsureThread, ResolveThreadSelector, StreamMessage (responses transport) until 90%.
- [ ] **2D -** `agents/internal/pma`: Add table-driven tests for uncovered branches until 90%.
- [ ] **2E -** `cynork/internal/gateway`: Add tests for ResponsesStream SSE parsing, PatchThreadTitle, GetChatThread, readChatSSEStream edge cases until 90%.
- [ ] **2F -** `orchestrator/internal/handlers`: Add tests for uncovered handler paths until 90%.
- [ ] **2G -** `cynork/cmd`: If still below 90% after Task 1, add tests for `runSlashProjectDelegated` bare-id logic or other uncovered paths until 90%.
- [ ] Run `just test-go-cover` after each package; do not proceed to the next package until the current one is green.
- [ ] Validation gate: do not proceed until all seven packages are at or above 90%.

#### Refactor (Task 2)

- [ ] Refine test code (structure, fixtures, naming) without reducing coverage or changing behavior.
  - [ ] Keep all tests green.
- [ ] Re-run `just test-go-cover`.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 2)

- [ ] Run `just test-go-cover` and confirm no package in any module is below 90% (except excluded e.g. testutil).
- [ ] Validation gate: do not start Task 3 until all checks for this task pass.

---

### Task 3: Undefined BDD Step Definitions

Implement the missing step definitions in `cynork/_bdd/steps.go` so that the 72 undefined scenarios can execute.
Group work by feature file; each group is independent and can be done in any order within this task.

#### Task 3 Requirements and Specifications

- `docs/requirements/client.md` (REQ-CLIENT-* for chat/TUI/slash)
- `docs/tech_specs/cynork_tui_slash_commands.md`, `cynork_tui.md` (behavior under test)
- Feature files under `features/cynork/` (scenarios and step text)

#### Discovery (Task 3) Steps

- [ ] Read the feature files and `cynork/_bdd/steps.go` to map undefined steps to required implementations.
- [ ] Confirm Task 1 and relevant Task 4 slash commands are in place so scenarios can run.
- [ ] Identify which steps require PTY harness (`tui_pty_harness.py`) vs `cynork chat` proxy; document approach for auth/streaming/thread steps.

#### Red (Task 3)

- [ ] For each feature group (3A-3G), list the exact step patterns and expected behavior; run `just test-bdd` and confirm scenarios are undefined or pending.
- [ ] Validation gate: do not proceed until step gaps are documented and failing/undefined state is confirmed.

#### Green (Task 3)

- [ ] **3A -** Implement steps for `cynork_tui_slash_model.feature` (gateway model list/error, scrollback checks, stored preferences unchanged).
- [ ] **3B -** Implement steps for `cynork_tui_slash_project.feature` (gateway project list/get, scrollback output checks).
- [ ] **3C -** Implement steps for `cynork_tui_slash_dispatch.feature` (scrollback contains references to model, project, thread, task; and status, auth, nodes, prefs, skills).
- [ ] **3D -** Implement steps for `cynork_tui_auth.feature` (TUI auth recovery, login prompt, exit outcome; use PTY harness or agreed approach).
- [ ] **3E -** Implement steps for `cynork_tui_streaming.feature` (streaming deltas, amendment event, in-flight turn, finalization).
- [ ] **3F -** Implement steps for `cynork_tui_threads.feature` (thread creation, selector, auto-reconnect, history, title).
- [ ] **3G -** Implement steps for `cynork_tui.feature` (thinking placeholder, in-flight indicator, file/drafts/scroll/cursor where implemented; mark known-pending with comment and chunk reference).
- [ ] Run `just test-bdd` after each group; fix step definitions until scenarios pass or are explicitly pending.
- [ ] Validation gate: do not proceed until all implementable steps are green and remaining undefined/pending are documented.

#### Refactor (Task 3)

- [ ] Refine step implementations (shared helpers, readability) without changing behavior.
  - [ ] Keep BDD scenarios green.
- [ ] Re-run `just test-bdd`.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 3)

- [ ] Run `just test-bdd` and confirm no new failures; undefined count reduced as intended.
- [ ] Validation gate: do not start Task 4 until all checks for this task pass.

---

### Task 4: Missing Slash Commands in TUI Dispatcher

Add the slash commands required by the spec to the TUI `slash.go` dispatcher (and chat_slash.go where applicable).

#### Task 4 Requirements and Specifications

- `docs/tech_specs/cynork_tui_slash_commands.md` (LocalSlashCommands, StatusSlashCommands, TaskSlashCommands, NodeSlashCommands, PreferenceSlashCommands, SkillSlashCommands)

#### Discovery (Task 4) Steps

- [ ] Read `docs/tech_specs/cynork_tui_slash_commands.md` for mandatory slash behaviors.
- [ ] Inspect `cynork/internal/tui/slash.go:handleSlashCmd` and `slashHelpCatalog`; list missing commands.
- [ ] Inspect `cynork/cmd/chat_slash.go` for parity with TUI.

#### Red (Task 4)

- [ ] Add or update BDD scenarios / unit tests for each new command (connect, show-thinking, hide-thinking, status, whoami, task, nodes, prefs, skills) so they fail before implementation.
  - [ ] Add unit tests in `tui/slash_test.go` and `cmd/cmd_test.go` as needed.
- [ ] Run targeted tests and confirm they fail.
- [ ] Validation gate: do not proceed until failing tests prove the gap.

#### Green (Task 4)

- [ ] **4A -** Add `/connect` to TUI and chat_slash: no arg show current gateway URL; with arg update session URL and optionally validate via GET /healthz; add to slashHelpCatalog; add unit tests.
- [ ] **4B -** Add `/show-thinking` and `/hide-thinking` to TUI and chat_slash; toggle session flag and persist `tui.show_thinking_by_default` (atomic write); add to slashHelpCatalog; add unit tests.
- [ ] **4C -** Add `/status` to TUI handleSlashCmd and slashHelpCatalog; add unit test.
- [ ] **4D -** Add `/whoami` case in TUI handleSlashCmd delegating to authWhoami(); add unit test.
- [ ] **4E -** Add task, nodes, prefs, skills slash commands in TUI (gateway client calls preferred over subprocess); add to slashHelpCatalog and handleSlashCmd; add unit tests for each.
- [ ] Run targeted tests until they pass.
- [ ] Validation gate: do not proceed until the targeted tests are green.

#### Refactor (Task 4)

- [ ] Refine implementation (shared dispatch helpers, naming) without changing behavior.
  - [ ] Keep all tests green.
- [ ] Re-run targeted tests and `just test-bdd` for affected scenarios.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 4)

- [ ] Run `just test-bdd` and lint for changed code.
- [ ] Validation gate: do not start Task 5 until all checks for this task pass.

---

### Task 5: Spec Compliance Gaps

Close implementation gaps where the code does not match the spec (thread commands in chat, resume-thread flag, ResponsesStream responseID, structured transcript, generation state, connection recovery).

#### Task 5 Requirements and Specifications

- `docs/tech_specs/cynork_tui_slash_commands.md` (ThreadSlashCommands)
- `docs/tech_specs/cli_management_app_commands_chat.md` (CliChat --resume-thread)
- `docs/tech_specs/cynork_tui.md` (TranscriptRendering, GenerationState, ConnectionRecovery)
- Gateway/client contract for SSE response id

#### Discovery (Task 5) Steps

- [ ] Read the spec sections for thread, transcript, thinking visibility, generation state, and connection recovery.
- [ ] Inspect `cmd/chat_slash.go:runSlashThread`, `cynork/cmd/chat.go` flags, `gateway/client.go:ResponsesStream`, `internal/tui/state.go` and model scrollback usage.

#### Red (Task 5)

- [ ] Add or update tests for 5A-5F (thread list/switch/rename in chat, resume-thread flag, responseID, structured transcript, spinner, connection recovery) so they fail before implementation.
- [ ] Validation gate: do not proceed until failing tests prove the gaps.

#### Green (Task 5)

- [ ] **5A -** Add `list`, `switch <selector>`, `rename <title>` to `cmd/chat_slash.go:runSlashThread`; call session ListThreads, ResolveThreadSelector, SetCurrentThreadID, PatchThreadTitle; add unit tests.
- [ ] **5B -** Add `--resume-thread` string flag to `cynork/cmd/chat.go`; in runChat resolve selector and set CurrentThreadID; add unit test.
- [ ] **5C -** Fix `ResponsesStream` to extract and return response id from SSE; update test to verify responseID.
- [ ] **5D -** Wire `state.go` types into TUI model; implement collapsed thinking placeholder, tool_call/tool_result rows; add unit tests (consider sub-tasks: wire types, text turns, thinking placeholders, tool rows).
- [ ] **5E -** Implement Braille spinner (with ASCII fallback), status chip on active assistant turn, canonical labels, tick-based advancement; add unit tests.
- [ ] **5F -** Add bounded-backoff reconnection, reconcile in-flight turn on reconnect, update status bar; add unit tests.
- [ ] Run targeted tests until they pass.
- [ ] Validation gate: do not proceed until the targeted tests are green.

#### Refactor (Task 5)

- [ ] Refine implementation without changing behavior; keep tests green.
- [ ] Re-run targeted tests.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 5)

- [ ] Run `just test-bdd` and `just test-go-cover` for affected packages.
- [ ] Validation gate: do not start Task 6 until all checks for this task pass.

---

### Task 6: Project List and Get Stubs in TUI

Replace stub strings for `/project list` and `/project get` with gateway API calls (or graceful handling when gateway does not yet implement the endpoint).

#### Task 6 Requirements and Specifications

- `docs/tech_specs/cynork_tui_slash_commands.md` (ProjectSlashCommands)
- `docs/requirements/client.md` and project/gateway spec for `/v1/projects`

#### Discovery (Task 6) Steps

- [ ] Read project/gateway spec for `/v1/projects` and project-get behavior.
- [ ] Inspect `tui/slash.go:dispatchProjectCmd` (lines 322, 327) for current stub behavior.

#### Red (Task 6)

- [ ] Add or update tests that expect `/project list` and `/project get <id>` to call gateway and render results (or handle 404); run tests and confirm they fail with stubs.
- [ ] Validation gate: do not proceed until failing tests prove the gap.

#### Green (Task 6)

- [ ] Implement `/project list` by calling gateway project-list API through session client.
- [ ] Implement `/project get <id>` by calling gateway project-get API.
- [ ] Handle 404 gracefully when gateway does not yet implement the endpoint (per User-Gateway Alignment).
- [ ] Add unit tests with mock gateway responses.
- [ ] Run targeted tests until they pass.
- [ ] Validation gate: do not proceed until the targeted tests are green.

#### Refactor (Task 6)

- [ ] Refine implementation without changing behavior; keep tests green.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 6)

- [ ] Run `just test-bdd` and lint for changed code.
- [ ] Validation gate: do not start Task 7 until all checks for this task pass.

---

### Task 7: Remove Lint Suppressions and Fix Underlying Issues

Remove all `//nolint` usages except the single owner-approved exception in `worker_node/internal/telemetry/sloghandler.go`; fix the underlying issues so lint passes without suppression.

#### Task 7 Requirements and Specifications

- `meta.md` (no new lint suppressions; do not add `//nolint`; remove existing except approved exception)
- Justfile lint targets

#### Discovery (Task 7) Steps

- [ ] Run `grep -Rn '//nolint' --include='*.go' .` and confirm inventory matches the plan.
- [ ] Read `meta.md` and the approved exception (sloghandler.go gocritic hugeParam).

#### Red (Task 7)

- [ ] Document each suppression (linter, file, reason); confirm that removing it causes lint to fail.
- [ ] Validation gate: do not proceed until the baseline lint failures are known.

#### Green (Task 7)

- [ ] Fix and remove suppressions in cynork (cmd, internal/tui, internal/chat, internal/gateway): dupl, gocyclo, mnd.
- [ ] Fix and remove suppressions in orchestrator: dupl, gocognit, gocyclo, exhaustruct.
- [ ] Fix and remove suppressions in agents and worker_node (do not remove sloghandler.go gocritic nolint).
- [ ] For each: dupl - extract shared helpers; gocognit/gocyclo - refactor to reduce complexity; mnd - named constant; exhaustruct - populate fields or constructor; gocritic - pointer/refactor (except approved).
- [ ] After each file or group run `just lint` and confirm no new violations; do not add nolint elsewhere.
- [ ] Validation gate: exactly one `//nolint` remains (sloghandler.go); all other .go files have zero nolint and `just lint` passes.

#### Refactor (Task 7)

- [ ] Refine refactors (naming, structure) without reintroducing lint issues.
- [ ] Re-run `just lint`.
- [ ] Validation gate: do not proceed until refactor is verified.

#### Testing (Task 7)

- [ ] Run `just lint` and confirm no violations; confirm only one nolint in repo (sloghandler.go).
- [ ] Validation gate: do not start Task 8 until all checks for this task pass.

---

### Task 8: Documentation and Closeout

Update documentation and verify completion criteria.

#### Task 8 Requirements and Specifications

- This plan document; `docs/tech_specs/` and `docs/requirements/` if any spec or requirement was updated during execution.

#### Discovery (Task 8) Steps

- [ ] Review which specs or requirements were changed during Tasks 1-7 (if any).
- [ ] Identify any user-facing or developer-facing docs that need updates.

#### Red (Task 8)

- [ ] N/A (closeout task).

#### Green (Task 8)

- [ ] Update any required user-facing or developer-facing documentation.
- [ ] Update this plan or dev_docs with completion status and any remaining risks or follow-up work.
- [ ] Remove or archive this plan from active work if all tasks are complete.

#### Refactor (Task 8)

- [ ] N/A.

#### Testing (Task 8)

- [ ] Run `just ci` and confirm full CI passes.
- [ ] Confirm no required follow-up work was left undocumented.
- [ ] Validation gate: plan is complete when `just ci` passes and follow-up is documented.

---

## Senior Go Reviewer Findings (Context)

Per the Senior Go Developer Reviewer skill: adversarial review against specs, best practices, and production readiness.
Repo tooling: `justfile`; validation via `just ci` (lint, test-go-cover, vulncheck-go, test-bdd).

### Review Summary

Cynork TUI/chat implementation is partially aligned with specs; two BDD scenarios fail due to missing CLI flags and incorrect slash dispatch.
Coverage is enforced per package; seven packages are below 90%.
Several spec-mandated behaviors (thinking visibility, structured transcript, connection recovery, response ID in ResponsesStream) are missing or stubbed.
No critical concurrency or security flaws identified in the reviewed paths; error handling and context propagation are generally correct.

### Specification Compliance Issues

- **Missing `--model` on `cynork chat`:** CliChat spec requires `--model` (string, optional).
  Not implemented; BDD step that passes `--model` causes "unknown flag" and scenario failure.
- **`/project <bare_id>` in chat:** ProjectSlashCommands algorithm requires bare id to set session project. `cmd/chat_slash.go` delegates all input to Cobra; bare id is not a subcommand so usage is printed and session is not updated.
- **`/thread` in chat only supports `new`:** ThreadSlashCommands requires list, switch, rename.
  Only `new` is implemented in `runSlashThread`.
- **Missing `--resume-thread` on chat:** CliChat requires `--resume-thread <thread_selector>`.
  Not implemented.
- **ResponsesStream never populates responseID:** Gateway client returns empty string; spec expects response id from SSE.
- **Structured transcript and thinking visibility:** TranscriptRendering and ThinkingVisibilityBehavior not implemented (state types exist but unused; no `/show-thinking`/`/hide-thinking` in dispatcher).
- **Generation state and connection recovery:** GenerationState and ConnectionRecovery spec sections not implemented (no canonical spinner, no reconnection with backoff).

### Architectural Issues

- **Unused state types:** `internal/tui/state.go` defines TranscriptTurn, PartKind, etc., but model uses flat `[]string` scrollback; no layering of structured transcript into view.
- **Dual dispatch paths:** Slash commands implemented in both `cmd/chat_slash.go` (Cobra subprocess delegation) and `tui/slash.go` (in-process).
  Divergence (e.g. project bare id) causes inconsistent behavior between `cynork chat` and `cynork tui`.
- **Fat model:** `tui/model.go` handles init, update, view, slash, thread, stream, login; could be split into smaller components for testability and coverage.

### Concurrency / Safety Issues

- No data races identified in reviewed code.
  Streaming uses channels and single goroutine for delta application; cancellation via context and channel close is present.
- Potential goroutine leak if stream is abandoned without closing the delta channel; existing tests and Ctrl+C path should be verified.

### Security Risks

- Token and config are not logged (spec-compliant).
  Password input in login form must use secure input (no echo); confirm in auth BDD steps.
- No hardcoded secrets observed.

### Performance Concerns

- `captureToLines` in TUI slash uses os.Pipe and redirects Stdout/Stderr; acceptable for slash command output capture.
- No unbounded allocations identified in hot paths.

### Maintainability Issues

- Stub strings ("project list: not yet supported (stub)") in production code; should be replaced with gateway calls or explicit "not implemented" errors with issue references.
- 72 undefined BDD steps indicate many scenarios are not yet executable; increases risk of spec drift.
- **Lint suppressions:** `meta.md` forbids adding linter suppression comments (e.g. `//nolint`).
  The repo currently contains multiple `//nolint` usages (dupl, gocognit, gocyclo, mnd, gocritic, exhaustruct).
  All must be removed and the underlying issue fixed except one owner-approved exception: `worker_node/internal/telemetry/sloghandler.go` (gocritic hugeParam - `slog.Handler.Handle` interface requires `slog.Record` by value; cannot change to pointer).
  See Task 7.

### Recommended Refactor Strategy

1. Fix the two failing BDD scenarios (Task 1) and validate with `just test-bdd`.
2. Bring all seven packages to >=90% coverage with targeted unit tests (Task 2).
3. Align chat and TUI slash dispatch (bare project id, thread subcommands) and add missing flags (Tasks 1B, 5A, 5B).
4. Implement missing slash commands and wire state.go into transcript rendering (Tasks 4, 5D, 5E); then add connection recovery (Task 5F).
5. Replace stubs with gateway calls or documented "not implemented" paths (Task 6).

---

## Execution Order

Execute in this order; do not start the next task until the current task's Testing validation gate passes.

1. **Task 1** (fix 2 failing BDD scenarios): gate `just test-bdd` 0 failed.
2. **Task 2** (coverage gaps): gate `just test-go-cover` no package below 90%.
3. **Task 5A, 5B** (thread commands in chat, resume-thread flag): small, high-value; then Task 4A-4D (connect, thinking, status, whoami).
4. **Task 3** (undefined BDD steps): requires Task 1 and Task 4 commands in place.
5. **Task 6** (project list/get stubs).
6. **Task 4E** (task, nodes, prefs, skills in TUI).
7. **Task 5C, 5D, 5E, 5F** (responseID, structured transcript, generation state, connection recovery).
8. **Task 7** (remove lint suppressions except approved exception): gate only sloghandler.go nolint remains and `just lint` passes.
9. **Task 8** (documentation and closeout): gate `just ci` passes.

---

## Verification Checklist

After each task:

- [ ] Run the task's Testing steps and confirm the validation gate passes before starting the next task.
- [ ] `just test-bdd`: no new failures for scenarios that have step definitions.
- [ ] `just test-go-cover`: all packages at or above 90% (when Task 2 is complete).
- [ ] `just lint`: no new violations; after Task 7, only one `//nolint` remains (sloghandler.go).
- [ ] **Final gate (Task 8):** `just ci` passes before considering the plan complete.
