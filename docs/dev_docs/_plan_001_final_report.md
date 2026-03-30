# `_plan_001_immediate.md` - Final Status (2026-03-30)

- [Completed](#completed)
- [Validation run this session](#validation-run-this-session)
- [Residual risks / follow-ups](#residual-risks--follow-ups)
- [Follow-ups 1-3 executed (2026-03-30)](#follow-ups-1-3-executed-2026-03-30)
- [Task 13 CI note](#task-13-ci-note)

## Completed

- **Tasks 1-14** per plan: PMA WriteTimeout, container name match, TUI ensure-thread race, securestore `Close`, constant-time tokens, `ExitCode *int`, insecure-default validation, MCP gateway, `planning_state`, pod `--network=none` + `e2e_0325`, SBA prompt order + persona fields, **PMA keep-warm + secret scan + overwrite NDJSON**, CI branch triggers (`mvp/**`), local **`just ci`**, **`just e2e --tags no_inference`**, **`just docs-check docs/dev_docs`**, completion reports, plan checkbox sync.

## Validation Run This Session

- `just test-go-cover`, `just lint-go-ci`, `just ci` (includes `bdd-ci` / `just test-bdd`).
- `just e2e --tags no_inference` (~117 tests, ~16 min).
- `just e2e --tags pma_inference,streaming` (26 tests, ~158 s; after `just setup-dev restart`).
- `just docs-check docs/dev_docs`.

## Follow-Ups 1-3 Executed (2026-03-30)

1. **`just e2e --tags pma_inference,streaming`:** Run after `just setup-dev restart` (no `--ollama-in-stack`).
   **26 tests OK** (~158 s); plan `imm-214` marked **completed**.
2. **REQ-PMAGNT-0126:** `redactKnownSecrets` runs `secretutil.RunWithSecret` (same pattern as `appendStreamBufferSecure`); core logic in `redactKnownSecretsImpl`.
3. **BDD traceability:** `features/agents/pma_chat_and_context.feature` - `@req_pmagnt_0125` + `cynai_pmagnt_pmaopportunisticsecretscan` scenario (Godog); `@req_pmagnt_0129` + `cynai_pmagnt_nodelocalmodelloadandkeepalive` on a `@wip` traceability scenario.
   Worker `0174` tags remain in `features/worker_node/` (unchanged).

## Residual Risks / Follow-Ups

1. **GitHub Actions vs `just ci`:** Workflow does not mirror every local target one-for-one; `mvp/**` triggers are aligned for feature branches.
2. **REQ-PMAGNT-0129:** Executable BDD for keep-warm is still deferred (`@wip` scenario documents traceability; unit tests cover behavior).

## Task 13 CI Note

- `.github/workflows/ci.yml` - `push` / `pull_request` include `mvp/**` (in addition to `main` / `master`).
