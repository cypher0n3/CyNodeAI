# SBA Inference Remediation Plan

## Metadata

- Date: 2026-03-04
- Status: remediation plan
- Scope: SBA inference behavior for prompt-driven tasks and adjacent result-surface issues
- Audience: orchestrator, worker node, SBA runner, and CLI maintainers

## Summary

This plan remediates the current gap where SBA jobs can complete without performing inference for prompt-style work.
This plan also adds guardrails so similar drift is caught early by contract tests and BDD coverage.
The target outcome is that prompt tasks routed to SBA produce model-derived outputs in a deterministic and spec-aligned way.

## Specification Compliance Issues

- `docs/tech_specs/cynode_sba.md` defines `steps` as optional suggested to-dos, but current behavior can force strict direct-step execution.
- `docs/requirements/sbagnt.md` requires SBA to operate as a full AI agent with inference (`REQ-SBAGNT-0001`, `REQ-SBAGNT-0109`), but prompt+SBA flow may bypass inference.
- `docs/tech_specs/cynode_sba.md` requires runtime to provide at least one allowed model and inference endpoint injection, but task builder path does not enforce this precondition for SBA prompt tasks.
- `docs/tech_specs/cli_management_app_commands_tasks.md` requires task result output with stdout and stderr in terminal states, but there is no explicit contract for where the user-facing SBA final answer is mapped.

## Architectural Risks

- Task intent is under-specified at dispatch time, so execution mode can drift between inferred agent flow and literal step execution.
- Worker-level defaults can silently override orchestrator intent when environment flags are hardcoded.
- Result payload lacks a normalized user-answer field for SBA prompt tasks, so clients rely on incidental stdout behavior.
- Current tests validate that SBA result exists, but not that prompt tasks actually invoke inference and return a useful answer.

## Remediation Principles

- Make execution mode explicit and transport it end-to-end in job spec and runtime.
- Enforce preflight checks that fail closed when inference is required but unavailable.
- Define one canonical mapping of SBA final answer into persisted job result and task result output.
- Add cross-layer tests that validate behavior, not only payload presence.
- Preserve backward compatibility for non-inference direct-step jobs by using an explicit mode instead of implicit defaults.

## Step-By-Step Remediation Plan

The sequence below is ordered to reduce regression risk while restoring spec-compliant SBA inference behavior.

### Step 1: Define SBA Execution Mode Contract

- Add an explicit execution mode in the SBA job spec contract, with values `agent_inference` and `direct_steps`.
- State that prompt-driven SBA task creation MUST use `agent_inference` by default.
- State that `direct_steps` is only for explicit deterministic runbooks and migration compatibility.
- Define conflict rules where mode overrides implied behavior from `steps` presence.
- Document backward compatibility and rollout default in `docs/tech_specs/cynode_sba.md` and linked worker/orchestrator specs.

### Step 2: Define Inference Readiness Preconditions

- Specify a pre-dispatch validation gate in orchestrator for SBA jobs with `agent_inference` mode.
- Validation MUST check that `inference.allowed_models` is non-empty and at least one allowed model is reachable through injected runtime path.
- Validation MUST fail task/job creation with a structured error when preconditions are not met.
- Add requirement and spec traceability updates for this gate, including failure semantics.

### Step 3: Orchestrator Job Builder Changes

- Update SBA task payload builder to stop hardcoding placeholder steps for prompt tasks.
- Populate job context with task intent, acceptance criteria, and prompt content suitable for SBA planning loop.
- Set execution mode to `agent_inference` for prompt + `use_sba` flows.
- Keep explicit `direct_steps` support for known deterministic command pipelines.
- Add a feature flag for temporary fallback only if needed for staged rollout.

### Step 4: Worker API Runtime Mode Enforcement

- Change worker SBA launch logic to set direct-step environment only when job mode is `direct_steps`.
- Remove hardcoded direct-step env injection for all SBA jobs.
- Add runtime validation so unknown execution mode fails fast with structured job failure.
- Emit structured logs and telemetry fields for selected execution mode and inference endpoint presence.

### Step 5: SBA Runner Behavioral Alignment

- Ensure SBA treats `steps` as suggested to-dos in `agent_inference` mode.
- Ensure SBA deterministic direct execution path is only used in `direct_steps` mode.
- Ensure SBA reports whether inference was used in result metadata for observability and tests.
- Ensure failures due to missing model path map to canonical failure code and actionable failure message.

### Step 6: Result Surface Contract Normalization

- Define a canonical SBA final answer field in SBA result contract for prompt-style tasks.
- Define deterministic mapping from SBA final answer to persisted orchestrator job result.
- Define deterministic mapping from job result to task result stdout for CLI parity until richer client fields are adopted.
- Preserve existing stdout and stderr capture rules from worker API spec.
- Document precedence rules when both stdout and structured SBA answer are present.

### Step 7: API and CLI Compatibility Updates

- Update task result response spec notes to clarify how SBA structured answers appear in task result output.
- Keep CLI output contract stable while using normalized server mapping to avoid CLI-specific heuristics.
- Add compatibility notes so existing consumers of `jobs.result` keep working.

### Step 8: Testing and Verification Expansion

- Add orchestrator unit tests for SBA job builder mode selection and preflight inference validation.
- Add worker unit tests for mode-based env injection and failure behavior for unknown mode.
- Add SBA unit tests for mode semantics, including `steps` as suggested in inference mode.
- Add integration tests that verify prompt+SBA returns model-derived answer, not placeholder step output.
- Add BDD scenarios for both modes and for inference-unavailable error path.
- Add E2E checks in `scripts/test_scripts` to assert non-empty prompt answer and contract field presence.

### Step 9: Rollout and Safety Controls

- Introduce a temporary orchestrator feature flag for old SBA path fallback with default off after validation period.
- Add deployment-time smoke checks that run one prompt SBA task and assert model-derived answer.
- Track mode distribution metrics and error rates before removing fallback code.
- Remove fallback path and dead code after two stable release cycles.

### Step 10: Spec Drift Prevention

- Add contract tests that assert `steps` semantics and execution mode handling against spec fixtures.
- Add CI doc-to-test trace checklist for SBA inference behavior.
- Add regression tests for every previously observed failure signature, including empty stdout with successful status.
- Require any future SBA execution mode changes to include spec updates and BDD evidence in same PR.

## Acceptance Criteria

- Prompt + `use_sba` tasks run in `agent_inference` mode by default.
- SBA prompt tasks produce model-derived final answer in persisted result and task result output.
- Worker no longer forces direct-step mode unless explicitly requested.
- Inference-unavailable conditions fail early with deterministic structured error.
- Test suite includes unit, integration, BDD, and E2E coverage for both execution modes.
- `just docs-check` passes with all updated docs and links.

## Work Breakdown and Sequencing

- Phase A: Spec updates for execution mode, preflight checks, and result mapping.
- Phase B: Orchestrator and worker implementation for mode propagation and enforcement.
- Phase C: SBA runner behavior updates and result contract wiring.
- Phase D: Test coverage expansion across unit, integration, BDD, and E2E.
- Phase E: Staged rollout with fallback flag and telemetry.
- Phase F: Fallback removal and hardening.

## Validation Commands

- `just docs-check`
- `just test-go`
- `just test-bdd`
- `just e2e`
- `just ci`

## Out of Scope

- Broad redesign of PMA architecture or worker-managed services lifecycle.
- New client UX beyond task result contract and compatibility mapping.
- Non-SBA prompt routing changes unrelated to SBA inference remediation.

## Risks and Mitigations

- Risk: Partial rollout causes mixed behavior across nodes.
  Mitigation: Gate by explicit execution mode and release with compatibility flag.
- Risk: Contract changes break existing consumers of `jobs.result`.
  Mitigation: Add additive fields first and preserve legacy fields until migration completes.
- Risk: Inference endpoint availability differs by environment.
  Mitigation: Enforce preflight validation and add environment smoke tests.
- Risk: Test flakiness in inference-dependent scenarios.
  Mitigation: Use deterministic fixtures where possible and keep one dedicated inference smoke test path.
