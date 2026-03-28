---
name: Strict Godog across all BDD modules
overview: |
  Roll out github.com/cucumber/godog Options.Strict under GODOG_STRICT=1 across every
  workspace module that contains a `_bdd` package (agents, cynork, e2e, orchestrator,
  worker_node), while go_shared_libs stays out of scope.
  Work is sequenced module-by-module with inventory first, then worker_node, orchestrator,
  e2e, cynork, agents @wip completion, and documentation or CI closeout.
  Each task ends with a dev_docs report and a hold point before the next task.
todos:
  - id: strict-godog-step-001
    content: "Read [`go.work`](../../go.work) and list workspace directories that contain a `_bdd` package (expect [`agents`](../../agents/_bdd), [`cynork`](../../cynork/_bdd), [`e2e`](../../e2e/_bdd), [`orchestrator`](../../orchestrator/_bdd), [`worker_node`](../../worker_node/_bdd); confirm [`go_shared_libs`](../../go_shared_libs) has none)."
    status: pending
  - id: strict-godog-step-002
    content: "Read root [`justfile`](../../justfile) targets `ci`, `bdd-ci`, and `test-bdd`, plus [`agents/_bdd/suite_test.go`](../../agents/_bdd/suite_test.go), and summarize how `GODOG_STRICT` maps to `godog.Options.Strict` today."
    status: pending
    dependencies:
      - strict-godog-step-001
  - id: strict-godog-step-003
    content: "From repo root, run `(cd agents && GODOG_STRICT=1 go test -count=1 ./_bdd)` and confirm exit code 0."
    status: pending
    dependencies:
      - strict-godog-step-002
  - id: strict-godog-step-004
    content: "From repo root, run `(cd orchestrator && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd) 2>&1 | tee tmp/bdd_strict_orchestrator.log`."
    status: pending
    dependencies:
      - strict-godog-step-003
  - id: strict-godog-step-005
    content: "From repo root, run `(cd worker_node && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd) 2>&1 | tee tmp/bdd_strict_worker_node.log`."
    status: pending
    dependencies:
      - strict-godog-step-004
  - id: strict-godog-step-006
    content: "From repo root, run `(cd e2e && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd) 2>&1 | tee tmp/bdd_strict_e2e.log`."
    status: pending
    dependencies:
      - strict-godog-step-005
  - id: strict-godog-step-007
    content: "From repo root, run `(cd cynork && GODOG_STRICT=1 go test -count=1 -timeout 45m ./_bdd) 2>&1 | tee tmp/bdd_strict_cynork.log`."
    status: pending
    dependencies:
      - strict-godog-step-006
  - id: strict-godog-step-008
    content: "Create or update `tmp/bdd_strict_gap_analysis.md` with deduplicated `step is undefined`, `step implementation is pending`, and `ambiguous step definition` lines from the four non-agent logs in `tmp/`, grouped by module."
    status: pending
    dependencies:
      - strict-godog-step-007
  - id: strict-godog-step-009
    content: "In the same gap analysis artifact, record the `godogStrict()` strategy: either copy the small helper into each `*/_bdd/suite_test.go` or introduce a shared internal package, including import-cycle rationale."
    status: pending
    dependencies:
      - strict-godog-step-008
  - id: strict-godog-step-010
    content: "Update the comment above `bdd-ci` in root [`justfile`](../../justfile) to describe the end state: every workspace `_bdd` package enables `godog.Options.Strict` when `GODOG_STRICT=1` (same semantics as today in [`agents/_bdd/suite_test.go`](../../agents/_bdd/suite_test.go))."
    status: pending
    dependencies:
      - strict-godog-step-009
  - id: strict-godog-step-011
    content: "Refactor (Task 1): if no production code moved, write `no code refactor` in [`docs/dev_docs/2026-03-27_task1_strict_godog_inventory_report.md`](../../docs/dev_docs/2026-03-27_task1_strict_godog_inventory_report.md); otherwise run `gofmt` and `go mod tidy` in each touched module."
    status: pending
    dependencies:
      - strict-godog-step-010
  - id: strict-godog-step-012
    content: "Testing (Task 1): if any `*.go` file changed during Task 1, run `just lint-go` from repo root; if none changed, record `skipped` in the Task 1 report."
    status: pending
    dependencies:
      - strict-godog-step-011
  - id: strict-godog-step-013
    content: "Testing (Task 1): from repo root, run `just test-bdd` with `GODOG_STRICT` unset and confirm exit code 0."
    status: pending
    dependencies:
      - strict-godog-step-012
  - id: strict-godog-step-014
    content: "Closeout (Task 1): write [`docs/dev_docs/2026-03-27_task1_strict_godog_inventory_report.md`](../../docs/dev_docs/2026-03-27_task1_strict_godog_inventory_report.md) linking `tmp/bdd_strict_*.log` and the gap analysis."
    status: pending
    dependencies:
      - strict-godog-step-013
  - id: strict-godog-step-015
    content: "Closeout (Task 1): hold point => do not start Task 2 until Task 1 Testing steps and the Task 1 report are complete."
    status: pending
    dependencies:
      - strict-godog-step-014
  - id: strict-godog-step-016
    content: "Closeout (Task 1): mark every Task 1 checkbox in this plan file as `- [x]` when Task 1 execution finishes."
    status: pending
    dependencies:
      - strict-godog-step-015
  - id: strict-godog-step-017
    content: "Discovery (Task 2): read [`features/worker_node/`](../../features/worker_node/) feature files that mention secure-store env vars and grep [`worker_node/_bdd/`](../../worker_node/_bdd/) for `CYNODE_SECURE_STORE_MASTER_KEY` step regexes; note which patterns overlap the literal `B64` suffix."
    status: pending
    dependencies:
      - strict-godog-step-016
  - id: strict-godog-step-018
    content: "Red (Task 2): from repo root, run `(cd worker_node && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd) 2>&1 | tee tmp/bdd_strict_worker_node_red.log` and confirm non-zero exit until fixes land."
    status: pending
    dependencies:
      - strict-godog-step-017
  - id: strict-godog-step-019
    content: "Green (Task 2): change [`worker_node/_bdd`](../../worker_node/_bdd) step registrations so `the environment variable CYNODE_SECURE_STORE_MASTER_KEY_B64 is set` matches exactly one regex (resolve ambiguity with `B(\d+)` vs literal `B64`)."
    status: pending
    dependencies:
      - strict-godog-step-018
  - id: strict-godog-step-020
    content: "Green (Task 2): implement the step text `the node manager launches the managed service with the same derived \"OLLAMA_NUM_CTX\" value` (or align feature wording) in [`worker_node/_bdd`](../../worker_node/_bdd) with a registered `ctx.Step` pattern."
    status: pending
    dependencies:
      - strict-godog-step-019
  - id: strict-godog-step-021
    content: "Green (Task 2): add `godogStrict()` and `Strict: godogStrict()` to [`worker_node/_bdd/suite_test.go`](../../worker_node/_bdd/suite_test.go) matching [`agents/_bdd/suite_test.go`](../../agents/_bdd/suite_test.go) unless Task 1 chose a shared helper."
    status: pending
    dependencies:
      - strict-godog-step-020
  - id: strict-godog-step-022
    content: "Refactor (Task 2): refactor `worker_node/_bdd` helpers only if needed for the new steps; if no refactor, record `none` in the Task 2 report."
    status: pending
    dependencies:
      - strict-godog-step-021
  - id: strict-godog-step-023
    content: "Testing (Task 2): `(cd worker_node && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits 0."
    status: pending
    dependencies:
      - strict-godog-step-022
  - id: strict-godog-step-024
    content: "Testing (Task 2): from repo root, run `just lint-go` after `worker_node` edits."
    status: pending
    dependencies:
      - strict-godog-step-023
  - id: strict-godog-step-025
    content: "Closeout (Task 2): write [`docs/dev_docs/2026-03-27_task2_worker_node_strict_godog_report.md`](../../docs/dev_docs/2026-03-27_task2_worker_node_strict_godog_report.md) listing files and scenarios touched."
    status: pending
    dependencies:
      - strict-godog-step-024
  - id: strict-godog-step-026
    content: "Closeout (Task 2): hold point => do not start Task 3 until Task 2 passes; mark all Task 2 checkboxes `- [x]` when done."
    status: pending
    dependencies:
      - strict-godog-step-025
  - id: strict-godog-step-027
    content: "Discovery (Task 3): map each [`features/orchestrator/*.feature`](../../features/orchestrator/) file to the primary [`orchestrator/_bdd/*.go`](../../orchestrator/_bdd/) step file that should own its bindings; append the map to `tmp/bdd_strict_gap_analysis.md`."
    status: pending
    dependencies:
      - strict-godog-step-026
  - id: strict-godog-step-028
    content: "Discovery (Task 3): refresh `tmp/bdd_strict_orchestrator.log` and build `tmp/bdd_strict_orchestrator_checklist.md` listing each failing `TestOrchestratorBDD/` scenario name with its undefined or ambiguous line."
    status: pending
    dependencies:
      - strict-godog-step-027
  - id: strict-godog-step-029
    content: "Red (Task 3): confirm `(cd orchestrator && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits non-zero before Green work; keep the log as the red signal."
    status: pending
    dependencies:
      - strict-godog-step-028
  - id: strict-godog-step-030
    content: "Green (Task 3): implement undefined steps for OpenAI-compatible chat, responses, SSE streaming, redaction, and amendment scenarios in [`orchestrator/_bdd`](../../orchestrator/_bdd) until `go test -run 'TestOrchestratorBDD//OpenAI'` passes under `GODOG_STRICT=1` (adjust `-run` to match Godog subtest names from the log)."
    status: pending
    dependencies:
      - strict-godog-step-029
  - id: strict-godog-step-031
    content: "Green (Task 3): implement remaining items from `tmp/bdd_strict_orchestrator_checklist.md` until `(cd orchestrator && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits 0."
    status: pending
    dependencies:
      - strict-godog-step-030
  - id: strict-godog-step-032
    content: "Green (Task 3): add `godogStrict()` and `Strict: godogStrict()` to [`orchestrator/_bdd/suite_test.go`](../../orchestrator/_bdd/suite_test.go)."
    status: pending
    dependencies:
      - strict-godog-step-031
  - id: strict-godog-step-033
    content: "Refactor (Task 3): consolidate duplicated HTTP or gateway helpers in [`orchestrator/_bdd`](../../orchestrator/_bdd) only after strict green; record changes in the Task 3 report."
    status: pending
    dependencies:
      - strict-godog-step-032
  - id: strict-godog-step-034
    content: "Testing (Task 3): `(cd orchestrator && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits 0."
    status: pending
    dependencies:
      - strict-godog-step-033
  - id: strict-godog-step-035
    content: "Testing (Task 3): from repo root, run `just lint-go` after orchestrator edits."
    status: pending
    dependencies:
      - strict-godog-step-034
  - id: strict-godog-step-036
    content: "Closeout (Task 3): write [`docs/dev_docs/2026-03-27_task3_orchestrator_strict_godog_report.md`](../../docs/dev_docs/2026-03-27_task3_orchestrator_strict_godog_report.md); hold Task 4; mark Task 3 checkboxes `- [x]` when done."
    status: pending
    dependencies:
      - strict-godog-step-035
  - id: strict-godog-step-037
    content: "Discovery (Task 4): map [`features/e2e/*.feature`](../../features/e2e/) files to [`e2e/_bdd/*.go`](../../e2e/_bdd/) owners; refresh `tmp/bdd_strict_e2e.log` and build `tmp/bdd_strict_e2e_checklist.md` from strict output."
    status: pending
    dependencies:
      - strict-godog-step-036
  - id: strict-godog-step-038
    content: "Red (Task 4): confirm `(cd e2e && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits non-zero before Green work."
    status: pending
    dependencies:
      - strict-godog-step-037
  - id: strict-godog-step-039
    content: "Green (Task 4): implement [`e2e/_bdd`](../../e2e/_bdd) steps until scenarios for OpenAI-compatible responses, streaming chat and responses, explicit thread creation, streaming amendment with and without secrets, and client cancel/disconnect pass under `GODOG_STRICT=1` (use checklist scenario names; adjust if Godog renames slightly)."
    status: pending
    dependencies:
      - strict-godog-step-038
  - id: strict-godog-step-040
    content: "Green (Task 4): add `godogStrict()` and `Strict: godogStrict()` to [`e2e/_bdd/suite_test.go`](../../e2e/_bdd/suite_test.go)."
    status: pending
    dependencies:
      - strict-godog-step-039
  - id: strict-godog-step-041
    content: "Refactor (Task 4): refactor streaming helpers in [`e2e/_bdd`](../../e2e/_bdd) only with strict tests green."
    status: pending
    dependencies:
      - strict-godog-step-040
  - id: strict-godog-step-042
    content: "Testing (Task 4): `(cd e2e && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits 0."
    status: pending
    dependencies:
      - strict-godog-step-041
  - id: strict-godog-step-043
    content: "Testing (Task 4): `just lint-go` from repo root after `e2e` edits."
    status: pending
    dependencies:
      - strict-godog-step-042
  - id: strict-godog-step-044
    content: "Closeout (Task 4): write [`docs/dev_docs/2026-03-27_task4_e2e_strict_godog_report.md`](../../docs/dev_docs/2026-03-27_task4_e2e_strict_godog_report.md); hold Task 5; mark Task 4 checkboxes `- [x]` when done."
    status: pending
    dependencies:
      - strict-godog-step-043
  - id: strict-godog-step-045
    content: "Discovery (Task 5): grep [`cynork/_bdd`](../../cynork/_bdd) for `godog.ErrPending` and `TODO: write pending` and save hits to `tmp/bdd_strict_cynork_pending.txt`; refresh `tmp/bdd_strict_cynork.log` and split undefined vs pending in `tmp/bdd_strict_cynork_checklist.md`."
    status: pending
    dependencies:
      - strict-godog-step-044
  - id: strict-godog-step-046
    content: "Red (Task 5): confirm `(cd cynork && GODOG_STRICT=1 go test -count=1 -timeout 45m ./_bdd)` exits non-zero before Green work."
    status: pending
    dependencies:
      - strict-godog-step-045
  - id: strict-godog-step-047
    content: "Green (Task 5): replace headless-testable pending stubs in [`cynork/_bdd/steps2.go`](../../cynork/_bdd/steps2.go) (and related step files) with real assertions; keep PTY-only scenarios tagged `@wip` per [`features/cynork/README.md`](../../features/cynork/README.md)."
    status: pending
    dependencies:
      - strict-godog-step-046
  - id: strict-godog-step-048
    content: "Green (Task 5): implement every remaining **undefined** step from `tmp/bdd_strict_cynork_checklist.md` in [`cynork/_bdd`](../../cynork/_bdd)."
    status: pending
    dependencies:
      - strict-godog-step-047
  - id: strict-godog-step-049
    content: "Green (Task 5): add `godogStrict()` and `Strict: godogStrict()` to [`cynork/_bdd/suite_test.go`](../../cynork/_bdd/suite_test.go)."
    status: pending
    dependencies:
      - strict-godog-step-048
  - id: strict-godog-step-050
    content: "Refactor (Task 5): refactor cynork BDD helpers only after strict green."
    status: pending
    dependencies:
      - strict-godog-step-049
  - id: strict-godog-step-051
    content: "Testing (Task 5): `(cd cynork && GODOG_STRICT=1 go test -count=1 -timeout 45m ./_bdd)` exits 0."
    status: pending
    dependencies:
      - strict-godog-step-050
  - id: strict-godog-step-052
    content: "Testing (Task 5): `just lint-go` from repo root after `cynork` edits."
    status: pending
    dependencies:
      - strict-godog-step-051
  - id: strict-godog-step-053
    content: "Closeout (Task 5): write [`docs/dev_docs/2026-03-27_task5_cynork_strict_godog_report.md`](../../docs/dev_docs/2026-03-27_task5_cynork_strict_godog_report.md); hold Task 6; mark Task 5 checkboxes `- [x]` when done."
    status: pending
    dependencies:
      - strict-godog-step-052
  - id: strict-godog-step-054
    content: "Discovery (Task 6): read [`features/agents/pma_chat_file_context.feature`](../../features/agents/pma_chat_file_context.feature) and [`features/agents/sba_inference.feature`](../../features/agents/sba_inference.feature) and confirm `@req_*` / `@spec_*` tags still match the intended behavior before removing `@wip`."
    status: pending
    dependencies:
      - strict-godog-step-053
  - id: strict-godog-step-055
    content: "Red (Task 6): with `@wip` still present on unimplemented scenarios, confirm `(cd agents && GODOG_STRICT=1 go test -count=1 ./_bdd)` exits 0; after removing `@wip` for a scenario, confirm strict fails until steps exist."
    status: pending
    dependencies:
      - strict-godog-step-054
  - id: strict-godog-step-056
    content: "Green (Task 6): implement PMA file-context steps in [`agents/_bdd`](../../agents/_bdd) for both scenarios in [`features/agents/pma_chat_file_context.feature`](../../features/agents/pma_chat_file_context.feature) until they pass under `GODOG_STRICT=1`."
    status: pending
    dependencies:
      - strict-godog-step-055
  - id: strict-godog-step-057
    content: "Green (Task 6): implement SBA inference steps in [`agents/_bdd`](../../agents/_bdd) for both scenarios in [`features/agents/sba_inference.feature`](../../features/agents/sba_inference.feature) until they pass under `GODOG_STRICT=1`."
    status: pending
    dependencies:
      - strict-godog-step-056
  - id: strict-godog-step-058
    content: "Green (Task 6): remove `@wip` tags from those four scenarios once they pass under strict mode."
    status: pending
    dependencies:
      - strict-godog-step-057
  - id: strict-godog-step-059
    content: "Refactor (Task 6): refactor PMA/SBA step helpers only with tests green."
    status: pending
    dependencies:
      - strict-godog-step-058
  - id: strict-godog-step-060
    content: "Testing (Task 6): `(cd agents && GODOG_STRICT=1 go test -count=1 ./_bdd)` exits 0."
    status: pending
    dependencies:
      - strict-godog-step-059
  - id: strict-godog-step-061
    content: "Testing (Task 6): `just lint-go` from repo root after `agents` edits."
    status: pending
    dependencies:
      - strict-godog-step-060
  - id: strict-godog-step-062
    content: "Closeout (Task 6): write [`docs/dev_docs/2026-03-27_task6_agents_strict_godog_report.md`](../../docs/dev_docs/2026-03-27_task6_agents_strict_godog_report.md); hold Task 7; mark Task 6 checkboxes `- [x]` when done."
    status: pending
    dependencies:
      - strict-godog-step-061
  - id: strict-godog-step-063
    content: "Documentation (Task 7): audit all five `*/_bdd/suite_test.go` files and confirm each sets `Strict: godogStrict()` with identical `GODOG_STRICT` semantics; fix drift if found."
    status: pending
    dependencies:
      - strict-godog-step-062
  - id: strict-godog-step-064
    content: "Documentation (Task 7): update root [`justfile`](../../justfile) `bdd-ci` comment to state all workspace `_bdd` modules run with `GODOG_STRICT=1` in CI."
    status: pending
    dependencies:
      - strict-godog-step-063
  - id: strict-godog-step-065
    content: "Documentation (Task 7): update [`features/README.md`](../../features/README.md) (or the canonical BDD pointer it references) documenting `@wip`, `GODOG_STRICT`, `just bdd-ci`, and `just ci`."
    status: pending
    dependencies:
      - strict-godog-step-064
  - id: strict-godog-step-066
    content: "Testing (Task 7): from repo root, run `just bdd-ci` and confirm exit code 0."
    status: pending
    dependencies:
      - strict-godog-step-065
  - id: strict-godog-step-067
    content: "Testing (Task 7): from repo root, run `just ci` when local prerequisites (Go, podman socket, linters, Python venv if required by lint) are satisfied; record any intentional skips with rationale in the completion report if a full run is not possible."
    status: pending
    dependencies:
      - strict-godog-step-066
  - id: strict-godog-step-068
    content: "Closeout (Task 7): write [`docs/dev_docs/2026-03-27_plan_strict_godog_completion_report.md`](../../docs/dev_docs/2026-03-27_plan_strict_godog_completion_report.md) summarizing tasks 1 through 7."
    status: pending
    dependencies:
      - strict-godog-step-067
  - id: strict-godog-step-069
    content: "Closeout (Task 7): mark every remaining checkbox in this plan file `- [x]` when the overall plan execution is finished."
    status: pending
    dependencies:
      - strict-godog-step-068
---
# Plan: Strict Godog Across All BDD Modules

## Goal

Enable `godog.Options.Strict` whenever `GODOG_STRICT=1` for every workspace `_bdd` package
so CI (`just bdd-ci` / `just ci`) fails on undefined, pending, or ambiguous steps across
agents, cynork, e2e, orchestrator, and worker_node.
[`go_shared_libs`](../../go_shared_libs) has no `_bdd` suite and is out of scope.

## References

- [`meta.md`](../../meta.md) (workspace layout and BDD expectations)
- [`ai_files/ai_coding_instructions.md`](../../ai_files/ai_coding_instructions.md) (Red / Green / Refactor workflow)
- [`features/README.md`](../../features/README.md) (suite tags, `@wip`, feature layout)
- Root [`justfile`](../../justfile) targets `ci`, `bdd-ci`, and `test-bdd`
- [`github.com/cucumber/godog`](https://github.com/cucumber/godog) `Options.Strict` semantics (official Cucumber Go runner)

## Constraints

- Do not weaken existing scenario intent: fix step bindings or split PTY-only work to Python E2E per [`features/cynork/README.md`](../../features/cynork/README.md) instead of deleting coverage.
- Keep `Tags: "~@wip"` in each `suite_test.go` unless the plan explicitly removes `@wip` after steps exist.
- Use `just` targets for repo checks (`just lint-go`, `just test-bdd`, `just bdd-ci`, `just ci`); do not bypass them with ad-hoc scripts unless a step documents why.
- Store scratch logs under `tmp/` only; do not commit large log files.

## Execution Plan

Execute tasks in order.
Do not start the next task until the previous task Testing and Closeout steps pass.

### Task 1: Inventory, Logs, and Shared Strict Contract

Establish a strict-mode baseline for every `_bdd` module, capture gap logs under `tmp/`, and record how `godogStrict()` will be shared or copied.

#### Task 1 Requirements and Specifications

- [`meta.md`](../../meta.md)
- [`features/README.md`](../../features/README.md)
- Root [`justfile`](../../justfile)

#### Discovery (Task 1)

- [ ] Read [`go.work`](../../go.work) and list workspace directories that contain a `_bdd` package (expect [`agents`](../../agents/_bdd), [`cynork`](../../cynork/_bdd), [`e2e`](../../e2e/_bdd), [`orchestrator`](../../orchestrator/_bdd), [`worker_node`](../../worker_node/_bdd); confirm [`go_shared_libs`](../../go_shared_libs) has none).
- [ ] Read root [`justfile`](../../justfile) targets `ci`, `bdd-ci`, and `test-bdd`, plus [`agents/_bdd/suite_test.go`](../../agents/_bdd/suite_test.go), and summarize how `GODOG_STRICT` maps to `godog.Options.Strict` today.
- [ ] From repo root, run `(cd agents && GODOG_STRICT=1 go test -count=1 ./_bdd)` and confirm exit code 0.
- [ ] From repo root, run `(cd orchestrator && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd) 2>&1 | tee tmp/bdd_strict_orchestrator.log`.
- [ ] From repo root, run `(cd worker_node && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd) 2>&1 | tee tmp/bdd_strict_worker_node.log`.
- [ ] From repo root, run `(cd e2e && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd) 2>&1 | tee tmp/bdd_strict_e2e.log`.
- [ ] From repo root, run `(cd cynork && GODOG_STRICT=1 go test -count=1 -timeout 45m ./_bdd) 2>&1 | tee tmp/bdd_strict_cynork.log`.

#### Red (Task 1)

- [ ] Create or update `tmp/bdd_strict_gap_analysis.md` with deduplicated `step is undefined`, `step implementation is pending`, and `ambiguous step definition` lines from the four non-agent logs in `tmp/`, grouped by module.

#### Green (Task 1)

- [ ] In the same gap analysis artifact, record the `godogStrict()` strategy: either copy the small helper into each `*/_bdd/suite_test.go` or introduce a shared internal package, including import-cycle rationale.
- [ ] Update the comment above `bdd-ci` in root [`justfile`](../../justfile) to describe the end state: every workspace `_bdd` package enables `godog.Options.Strict` when `GODOG_STRICT=1` (same semantics as today in [`agents/_bdd/suite_test.go`](../../agents/_bdd/suite_test.go)).

#### Refactor (Task 1)

- [ ] Refactor (Task 1): if no production code moved, write `no code refactor` in [`docs/dev_docs/2026-03-27_task1_strict_godog_inventory_report.md`](../../docs/dev_docs/2026-03-27_task1_strict_godog_inventory_report.md); otherwise run `gofmt` and `go mod tidy` in each touched module.

#### Testing (Task 1)

- [ ] Testing (Task 1): if any `*.go` file changed during Task 1, run `just lint-go` from repo root; if none changed, record `skipped` in the Task 1 report.
- [ ] Testing (Task 1): from repo root, run `just test-bdd` with `GODOG_STRICT` unset and confirm exit code 0.

#### Closeout (Task 1)

- [ ] Closeout (Task 1): write [`docs/dev_docs/2026-03-27_task1_strict_godog_inventory_report.md`](../../docs/dev_docs/2026-03-27_task1_strict_godog_inventory_report.md) linking `tmp/bdd_strict_*.log` and the gap analysis.
- [ ] Closeout (Task 1): hold point => do not start Task 2 until Task 1 Testing steps and the Task 1 report are complete.
- [ ] Closeout (Task 1): mark every Task 1 checkbox in this plan file as `- [x]` when Task 1 execution finishes.

### Task 2: `worker_node` `_bdd` Strict Mode

Fix ambiguous secure-store env steps, implement the missing managed-service step, and turn on `Strict` in [`worker_node/_bdd/suite_test.go`](../../worker_node/_bdd/suite_test.go).

#### Task 2 Requirements and Specifications

- [`features/worker_node/`](../../features/worker_node/)
- [`worker_node/_bdd/`](../../worker_node/_bdd/)

#### Discovery (Task 2)

- [ ] Discovery (Task 2): read [`features/worker_node/`](../../features/worker_node/) feature files that mention secure-store env vars and grep [`worker_node/_bdd/`](../../worker_node/_bdd/) for `CYNODE_SECURE_STORE_MASTER_KEY` step regexes; note which patterns overlap the literal `B64` suffix.

#### Red (Task 2)

- [ ] Red (Task 2): from repo root, run `(cd worker_node && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd) 2>&1 | tee tmp/bdd_strict_worker_node_red.log` and confirm non-zero exit until fixes land.

#### Green (Task 2)

- [ ] Green (Task 2): change [`worker_node/_bdd`](../../worker_node/_bdd) step registrations so `the environment variable CYNODE_SECURE_STORE_MASTER_KEY_B64 is set` matches exactly one regex (resolve ambiguity with `B(\\d+)` vs literal `B64`).
- [ ] Green (Task 2): implement the step text `the node manager launches the managed service with the same derived \"OLLAMA_NUM_CTX\" value` (or align feature wording) in [`worker_node/_bdd`](../../worker_node/_bdd) with a registered `ctx.Step` pattern.
- [ ] Green (Task 2): add `godogStrict()` and `Strict: godogStrict()` to [`worker_node/_bdd/suite_test.go`](../../worker_node/_bdd/suite_test.go) matching [`agents/_bdd/suite_test.go`](../../agents/_bdd/suite_test.go) unless Task 1 chose a shared helper.

#### Refactor (Task 2)

- [ ] Refactor (Task 2): refactor `worker_node/_bdd` helpers only if needed for the new steps; if no refactor, record `none` in the Task 2 report.

#### Testing (Task 2)

- [ ] Testing (Task 2): `(cd worker_node && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits 0.
- [ ] Testing (Task 2): from repo root, run `just lint-go` after `worker_node` edits.

#### Closeout (Task 2)

- [ ] Closeout (Task 2): write [`docs/dev_docs/2026-03-27_task2_worker_node_strict_godog_report.md`](../../docs/dev_docs/2026-03-27_task2_worker_node_strict_godog_report.md) listing files and scenarios touched.
- [ ] Closeout (Task 2): hold point => do not start Task 3 until Task 2 passes; mark all Task 2 checkboxes `- [x]` when done.

### Task 3: Orchestrator `_bdd` Strict Mode

Close all undefined and ambiguous bindings for gateway, chat, responses, streaming, and redaction scenarios, then enable `Strict` in [`orchestrator/_bdd/suite_test.go`](../../orchestrator/_bdd/suite_test.go).

#### Task 3 Requirements and Specifications

- [`features/orchestrator/`](../../features/orchestrator/)
- [`orchestrator/_bdd/`](../../orchestrator/_bdd/)

#### Discovery (Task 3)

- [ ] Discovery (Task 3): map each [`features/orchestrator/*.feature`](../../features/orchestrator/) file to the primary [`orchestrator/_bdd/*.go`](../../orchestrator/_bdd/) step file that should own its bindings; append the map to `tmp/bdd_strict_gap_analysis.md`.
- [ ] Discovery (Task 3): refresh `tmp/bdd_strict_orchestrator.log` and build `tmp/bdd_strict_orchestrator_checklist.md` listing each failing `TestOrchestratorBDD/` scenario name with its undefined or ambiguous line.

#### Red (Task 3)

- [ ] Red (Task 3): confirm `(cd orchestrator && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits non-zero before Green work; keep the log as the red signal.

#### Green (Task 3)

- [ ] Green (Task 3): implement undefined steps for OpenAI-compatible chat, responses, SSE streaming, redaction, and amendment scenarios in [`orchestrator/_bdd`](../../orchestrator/_bdd) until `go test -run 'TestOrchestratorBDD//OpenAI'` passes under `GODOG_STRICT=1` (adjust `-run` to match Godog subtest names from the log).
- [ ] Green (Task 3): implement remaining items from `tmp/bdd_strict_orchestrator_checklist.md` until `(cd orchestrator && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits 0.
- [ ] Green (Task 3): add `godogStrict()` and `Strict: godogStrict()` to [`orchestrator/_bdd/suite_test.go`](../../orchestrator/_bdd/suite_test.go).

#### Refactor (Task 3)

- [ ] Refactor (Task 3): consolidate duplicated HTTP or gateway helpers in [`orchestrator/_bdd`](../../orchestrator/_bdd) only after strict green; record changes in the Task 3 report.

#### Testing (Task 3)

- [ ] Testing (Task 3): `(cd orchestrator && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits 0.
- [ ] Testing (Task 3): from repo root, run `just lint-go` after orchestrator edits.

#### Closeout (Task 3)

- [ ] Closeout (Task 3): write [`docs/dev_docs/2026-03-27_task3_orchestrator_strict_godog_report.md`](../../docs/dev_docs/2026-03-27_task3_orchestrator_strict_godog_report.md); hold Task 4; mark Task 3 checkboxes `- [x]` when done.

### Task 4: e2e `_bdd` Strict Mode

Implement end-to-end OpenAI-compatible and streaming steps until [`e2e/_bdd`](../../e2e/_bdd) passes with `GODOG_STRICT=1`, then wire `Strict` in [`e2e/_bdd/suite_test.go`](../../e2e/_bdd/suite_test.go).

#### Task 4 Requirements and Specifications

- [`features/e2e/`](../../features/e2e/)
- [`e2e/_bdd/`](../../e2e/_bdd/)

#### Discovery (Task 4)

- [ ] Discovery (Task 4): map [`features/e2e/*.feature`](../../features/e2e/) files to [`e2e/_bdd/*.go`](../../e2e/_bdd/) owners; refresh `tmp/bdd_strict_e2e.log` and build `tmp/bdd_strict_e2e_checklist.md` from strict output.

#### Red (Task 4)

- [ ] Red (Task 4): confirm `(cd e2e && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits non-zero before Green work.

#### Green (Task 4)

- [ ] Green (Task 4): implement [`e2e/_bdd`](../../e2e/_bdd) steps until scenarios for OpenAI-compatible responses, streaming chat and responses, explicit thread creation, streaming amendment with and without secrets, and client cancel/disconnect pass under `GODOG_STRICT=1` (use checklist scenario names; adjust if Godog renames slightly).
- [ ] Green (Task 4): add `godogStrict()` and `Strict: godogStrict()` to [`e2e/_bdd/suite_test.go`](../../e2e/_bdd/suite_test.go).

#### Refactor (Task 4)

- [ ] Refactor (Task 4): refactor streaming helpers in [`e2e/_bdd`](../../e2e/_bdd) only with strict tests green.

#### Testing (Task 4)

- [ ] Testing (Task 4): `(cd e2e && GODOG_STRICT=1 go test -count=1 -timeout 35m ./_bdd)` exits 0.
- [ ] Testing (Task 4): `just lint-go` from repo root after `e2e` edits.

#### Closeout (Task 4)

- [ ] Closeout (Task 4): write [`docs/dev_docs/2026-03-27_task4_e2e_strict_godog_report.md`](../../docs/dev_docs/2026-03-27_task4_e2e_strict_godog_report.md); hold Task 5; mark Task 4 checkboxes `- [x]` when done.

### Task 5: Cynork `_bdd` Strict Mode

Replace headless-testable `ErrPending` stubs, implement undefined steps, keep PTY-only scenarios behind `@wip`, and enable `Strict` in [`cynork/_bdd/suite_test.go`](../../cynork/_bdd/suite_test.go).

#### Task 5 Requirements and Specifications

- [`features/cynork/`](../../features/cynork/)
- [`cynork/_bdd/`](../../cynork/_bdd/)
- [`features/cynork/README.md`](../../features/cynork/README.md)

#### Discovery (Task 5)

- [ ] Discovery (Task 5): grep [`cynork/_bdd`](../../cynork/_bdd) for `godog.ErrPending` and `TODO: write pending` and save hits to `tmp/bdd_strict_cynork_pending.txt`; refresh `tmp/bdd_strict_cynork.log` and split undefined vs pending in `tmp/bdd_strict_cynork_checklist.md`.

#### Red (Task 5)

- [ ] Red (Task 5): confirm `(cd cynork && GODOG_STRICT=1 go test -count=1 -timeout 45m ./_bdd)` exits non-zero before Green work.

#### Green (Task 5)

- [ ] Green (Task 5): replace headless-testable pending stubs in [`cynork/_bdd/steps2.go`](../../cynork/_bdd/steps2.go) (and related step files) with real assertions; keep PTY-only scenarios tagged `@wip` per [`features/cynork/README.md`](../../features/cynork/README.md).
- [ ] Green (Task 5): implement every remaining **undefined** step from `tmp/bdd_strict_cynork_checklist.md` in [`cynork/_bdd`](../../cynork/_bdd).
- [ ] Green (Task 5): add `godogStrict()` and `Strict: godogStrict()` to [`cynork/_bdd/suite_test.go`](../../cynork/_bdd/suite_test.go).

#### Refactor (Task 5)

- [ ] Refactor (Task 5): refactor cynork BDD helpers only after strict green.

#### Testing (Task 5)

- [ ] Testing (Task 5): `(cd cynork && GODOG_STRICT=1 go test -count=1 -timeout 45m ./_bdd)` exits 0.
- [ ] Testing (Task 5): `just lint-go` from repo root after `cynork` edits.

#### Closeout (Task 5)

- [ ] Closeout (Task 5): write [`docs/dev_docs/2026-03-27_task5_cynork_strict_godog_report.md`](../../docs/dev_docs/2026-03-27_task5_cynork_strict_godog_report.md); hold Task 6; mark Task 5 checkboxes `- [x]` when done.

### Task 6: Agents `_bdd` Strict Completion and `@wip` Removal

Implement PMA file-context and SBA inference steps, remove `@wip` from those four scenarios, and keep [`agents/_bdd`](../../agents/_bdd) green under `GODOG_STRICT=1`.

#### Task 6 Requirements and Specifications

- [`features/agents/pma_chat_file_context.feature`](../../features/agents/pma_chat_file_context.feature)
- [`features/agents/sba_inference.feature`](../../features/agents/sba_inference.feature)
- [`agents/_bdd/`](../../agents/_bdd/)

#### Discovery (Task 6)

- [ ] Discovery (Task 6): read [`features/agents/pma_chat_file_context.feature`](../../features/agents/pma_chat_file_context.feature) and [`features/agents/sba_inference.feature`](../../features/agents/sba_inference.feature) and confirm `@req_*` / `@spec_*` tags still match the intended behavior before removing `@wip`.

#### Red (Task 6)

- [ ] Red (Task 6): with `@wip` still present on unimplemented scenarios, confirm `(cd agents && GODOG_STRICT=1 go test -count=1 ./_bdd)` exits 0; after removing `@wip` for a scenario, confirm strict fails until steps exist.

#### Green (Task 6)

- [ ] Green (Task 6): implement PMA file-context steps in [`agents/_bdd`](../../agents/_bdd) for both scenarios in [`features/agents/pma_chat_file_context.feature`](../../features/agents/pma_chat_file_context.feature) until they pass under `GODOG_STRICT=1`.
- [ ] Green (Task 6): implement SBA inference steps in [`agents/_bdd`](../../agents/_bdd) for both scenarios in [`features/agents/sba_inference.feature`](../../features/agents/sba_inference.feature) until they pass under `GODOG_STRICT=1`.
- [ ] Green (Task 6): remove `@wip` tags from those four scenarios once they pass under strict mode.

#### Refactor (Task 6)

- [ ] Refactor (Task 6): refactor PMA/SBA step helpers only with tests green.

#### Testing (Task 6)

- [ ] Testing (Task 6): `(cd agents && GODOG_STRICT=1 go test -count=1 ./_bdd)` exits 0.
- [ ] Testing (Task 6): `just lint-go` from repo root after `agents` edits.

#### Closeout (Task 6)

- [ ] Closeout (Task 6): write [`docs/dev_docs/2026-03-27_task6_agents_strict_godog_report.md`](../../docs/dev_docs/2026-03-27_task6_agents_strict_godog_report.md); hold Task 7; mark Task 6 checkboxes `- [x]` when done.

### Task 7: Documentation, CI Verification, and Plan Completion

Align docs and [`justfile`](../../justfile) comments with full strict coverage, run `just bdd-ci` and `just ci`, and archive a completion report.

#### Task 7 Requirements and Specifications

- Root [`justfile`](../../justfile)
- [`features/README.md`](../../features/README.md)

#### Documentation (Task 7)

- [ ] Documentation (Task 7): audit all five `*/_bdd/suite_test.go` files and confirm each sets `Strict: godogStrict()` with identical `GODOG_STRICT` semantics; fix drift if found.
- [ ] Documentation (Task 7): update root [`justfile`](../../justfile) `bdd-ci` comment to state all workspace `_bdd` modules run with `GODOG_STRICT=1` in CI.
- [ ] Documentation (Task 7): update [`features/README.md`](../../features/README.md) (or the canonical BDD pointer it references) documenting `@wip`, `GODOG_STRICT`, `just bdd-ci`, and `just ci`.

#### Testing (Task 7)

- [ ] Testing (Task 7): from repo root, run `just bdd-ci` and confirm exit code 0.
- [ ] Testing (Task 7): from repo root, run `just ci` when local prerequisites (Go, podman socket, linters, Python venv if required by lint) are satisfied; record any intentional skips with rationale in the completion report if a full run is not possible.

#### Closeout (Task 7)

- [ ] Closeout (Task 7): write [`docs/dev_docs/2026-03-27_plan_strict_godog_completion_report.md`](../../docs/dev_docs/2026-03-27_plan_strict_godog_completion_report.md) summarizing tasks 1 through 7.
- [ ] Closeout (Task 7): mark every remaining checkbox in this plan file `- [x]` when the overall plan execution is finished.
