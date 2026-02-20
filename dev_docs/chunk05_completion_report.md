# Chunk 05 Completion Report: Worker API Sandbox Execution (Phase 1 Spec Constraints)

- [Deliverables](#deliverables)
- [Files Changed](#files-changed)
- [Validation](#validation)
- [References](#references)

## Deliverables

**Date:** 2026-02-19. **Plan:** [MVP Phase 1 Completion Plan](./mvp_phase1_completion_plan.md) Section 4.5.

Chunk 05 work is complete.
The Worker API sandbox execution now matches Phase 1 spec constraints: `network_policy` support, per-task workspace mount and working directory, task context environment variables without orchestrator secrets, and BDD coverage for these behaviors.

### 1. Network_policy_policy (`worker_api.md`)

- Implemented in `worker_node/cmd/worker-api/executor/executor.go`.
- `none` and `restricted` both map to `--network=none` (Phase 1: deny-all).
- Empty or unknown values default to `--network=none`.

### 2. Per-Task Workspace and Working Directory (`sandbox_container.md`)

- Executor accepts optional `workspaceDir`; when set, container run uses `-v <hostDir>:/workspace` and `-w /workspace`.
- Worker API creates a per-job workspace under `WORKSPACE_ROOT` (default: `$TMPDIR/cynodeai-workspaces/<job_id>`), passes it to the executor, and removes it after the job.
- Direct mode sets `cmd.Dir` to the host workspace path and `CYNODE_WORKSPACE_DIR` to that path; container mode uses `/workspace`.

### 3. Task Context Environment (`sandbox_container.md`)

- Injected env: `CYNODE_TASK_ID`, `CYNODE_JOB_ID`, `CYNODE_WORKSPACE_DIR`.
- Implemented in `buildTaskEnv`; request `sandbox.env` is merged but cannot override `CYNODE_*` (no orchestrator secrets in sandbox).

### 4. BDD Coverage

- Updated `features/worker_node/worker_node_sandbox_execution.feature` with scenarios:
  - Sandbox runs with `network_policy` "none" and "restricted".
  - Sandbox has working directory and task context environment (CYNODE_* present in stdout).
  - Request env cannot override CYNODE_* task context.
- New/updated step definitions in `worker_node/_bdd/steps.go`: submit with `network_policy`, submit with env, "completes successfully", "stdout contains" (substring).

### 5. CPU, Memory, PIDs Limits

- Not implemented (deferred per plan).

## Files Changed

| Path | Change |
|------|--------|
| `worker_node/cmd/worker-api/executor/executor.go` | `RunJob(_, _, workspaceDir)`, network_policy switch, workspace mount, `buildTaskEnv`, task env in direct mode |
| `worker_node/cmd/worker-api/main.go` | `WORKSPACE_ROOT`, per-job workspace create/cleanup, `validateRunJobRequest`, `prepareWorkspace` |
| `worker_node/cmd/worker-api/main_test.go` | `newMux`/handler signature (workspace root, logger), workspace success and failure tests |
| `worker_node/cmd/worker-api/executor/executor_test.go` | All `RunJob` calls pass `workspaceDir`; added `TestRunJobDirectTaskEnv`, `TestRunJobDirectWorkspaceDir`, `TestRunJobContainerPathWithWorkspace`, `TestRunJobDirectCynodeEnvNotOverridable` |
| `worker_node/_bdd/steps.go` | `RunJob(_, _, "")`, new steps for network_policy, env, "completes successfully", "stdout contains" |
| `features/worker_node/worker_node_sandbox_execution.feature` | Four new scenarios with `@req_worker_*` and `@phase1_sandbox` tags |
| `orchestrator/go.mod`, `worker_node/go.mod` | Added `github.com/cucumber/godog` so `just test-bdd` (and `just ci`) can run |

## Validation

- `just validate-feature-files`: OK  
- `just test-go-cover`: All packages >= 90%  
- `just test-bdd`: Pass (orchestrator and worker_node suites)  
- `just lint-go-ci`: 0 issues  
- `just ci`: Pass (lint, vulncheck, test-go-cover, test-bdd; test-go-race may still have an unrelated nodemanager timeout failure in some environments)

## References

- [worker_api.md](../docs/tech_specs/worker_api.md) (network_policy, Phase 1 constraints)
- [sandbox_container.md](../docs/tech_specs/sandbox_container.md) (workspace, env)
