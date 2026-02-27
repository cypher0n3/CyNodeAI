# MVP Plan vs Implementation Review and Next Steps

- [Summary](#summary)
- [Dev_docs Reviewed](#dev_docs-reviewed)
- [MVP Plan vs Implementation](#mvp-plan-vs-implementation)
- [Gaps and Corrections](#gaps-and-corrections)
- [Suggested Next Work](#suggested-next-work)
- [References](#references)

## Summary

**Date:** 2026-02-27

This document reviews `docs/dev_docs`, `docs/mvp_plan.md`, and the current codebase to align docs with implementation and suggest what to work on next.
No code changes; docs-only.

**Bottom line:** Phase 1, 1.5, and 1.7 are complete.
Phase 2 foundation (MCP audit, preference tools, SBA binary and worker integration) is in place.
The **orchestrator does not yet build or send SBA job specs** (no `job_spec_json` in dispatch).
Next priorities: (1) orchestrator SBA job construction and dispatch path (P2-10 completion); (2) MCP gateway allow path beyond preference tools or explicit MVP scope; (3) PMA langchaingo refactor; (4) LangGraph workflow (P2-04--P2-08).

## Dev_docs Reviewed

- **Document:** `README.md`
  - Purpose: Temp space, pre-merge cleanup.
  - Status vs implementation: Current; 10 files present, cleanup owed before merge.
- **Document:** `mvp_remediation_plan.md`
  - Purpose: Code review and remediation order.
  - Status vs implementation: Remediation 4.5 mostly done; pending: PMA langchaingo, step-executor, SBA timeout, Worker API cmd.
- **Document:** `sba_tools_review_and_spec_additions.md`
  - Purpose: SBA step types and spec additions.
  - Status vs implementation: Spec updates applied; implementation in `agents/internal/sba` has run_command, write_file, read_file, apply_unified_diff, list_tree.
- **Document:** `pma_llm_tool_instructions_plan.md`
  - Purpose: PMA/SBA instructions and tool buildout.
  - Status vs implementation: Plan; buildout report below.
- **Document:** `pma_llm_tool_instructions_buildout_report.md`
  - Purpose: Execution report.
  - Status vs implementation: Role bundles, default skill, PMA context, SBA baseline dir done; shared tool package and SBA code loading baseline not done.
- **Document:** `worker_api_self_report_address_2026-02-26.md`
  - Purpose: Node-reported Worker API URL.
  - Status vs implementation: Implemented (payload, orchestrator, E2E).
- **Document:** `model_warm_up_proposal.md`
  - Purpose: Reduce first-chat latency.
  - Status vs implementation: Proposal only; no implementation.
- **Document:** `worker_identity_and_node_versioning_spec_proposal.md`
  - Purpose: Identity/versioning.
  - Status vs implementation: Proposal/spec.
- **Document:** `zero_trust_tech_specs_assessment.md`
  - Purpose: Zero-trust assessment.
  - Status vs implementation: Assessment.
- **Document:** `shared_go_libraries_assessment.md`
  - Purpose: Shared libs assessment.
  - Status vs implementation: Assessment.

## MVP Plan vs Implementation

Alignment between `docs/mvp_plan.md` and the codebase.

### Current Status (From `mvp_plan.md`)

- Phase 1, 1.5, 1.7: Complete.
- Phase 2: "In progress."
  P2-02 foundation (audit table, store, `POST /v1/mcp/tools/call` writes audit, 501 for non-preference tools); P2-01 scoping and P2-03 preference tools done per Remaining section.
- P2-09 (cynode-sba binary and SBA runner image), P2-10 (Worker API and orchestrator integration for SBA jobs) listed as remaining.

### Implementation Verification

- **SBA binary (P2-09):** Present.
  `agents/` module has `cynode-sba` binary; `runner.go` / `agent_tools.go` implement job spec read, step types, result contract; Containerfile/SBA runner image per spec.
- **Worker API SBA path (P2-10):** Implemented.
  `go_shared_libs/contracts/workerapi`: `RunJobRequest.Sandbox.JobSpecJSON`, `RunJobResponse.SbaResult` (and artifacts).
  `worker_node` executor: `runJobSBA` when `job_spec_json` set and image is SBA runner; writes `/job/job.json`, runs container, reads `/job/result.json` into `SbaResult`.
- **Orchestrator SBA path (P2-10):** **Not implemented.**
  Dispatcher uses `workerapi.RunJobRequest` with `Sandbox.Command` (and optional `UseInference`).
  No code sets `Sandbox.JobSpecJSON` or selects an SBA runner image; job builder and task-to-SBA flow are missing.
  `applyJobResult` already persists full `RunJobResponse` (including `SbaResult`) to `jobs.result`.
- **MCP gateway:** Audit on every call.
  `db.preference.get`, `db.preference.list`, `db.preference.effective` implemented; all other tools return 501 ("tool routing not implemented").
  No allow path for `artifact.*` or other MCP tools yet.
- **LangGraph schema:** workflow_checkpoints and task_workflow_leases present in GORM and RunSchema (remediation plan 4.5).

## Gaps and Corrections

Recommended doc and scope corrections:

1. **`mvp_plan.md` "Remaining" order:** It states "P2-01, P2-03 done; full P2-02 allow path; **P2-09, P2-10**; LangGraph P2-04--P2-08."
   Implementation shows P2-09 and worker-side P2-10 done; **orchestrator-side P2-10 (build and dispatch SBA job specs) is not done.**
   Recommend updating "Current Status" or "Remaining" to: "P2-10 orchestrator: job builder and dispatch path for SBA jobs (`job_spec_json` + SBA runner image) not yet implemented."
2. **E2E/Compose:** No compose or E2E references to SBA runner image or SBA job type; E2E remains script-based with classic sandbox + inference.
   Adding an SBA job to E2E (or a dedicated scenario) would validate P2-10 end-to-end.
3. **Dev_docs cleanup:** Per `docs/dev_docs/README.md`, all files except README must be cleaned before merge (move, delete, or fold into permanent docs).

## Suggested Next Work

Prioritized next steps based on MVP plan and implementation gaps.

### 1. Complete P2-10 Orchestrator Side (High)

- **Goal:** Orchestrator can create and dispatch jobs that use the SBA runner (image + `job_spec_json`), and persist `sba_result` (already done via full response JSON).
- **Tasks (summary):**
  - Define when a task/job uses SBA (e.g. task type, or PMA/workflow decision; may be MVP-narrow: e.g. "SBA task" flag or single path).
  - Add job builder that produces `RunJobRequest` with `Sandbox.JobSpecJSON` (and SBA runner image) from task context and spec (e.g. from `go_shared_libs/contracts/sbajob`).
  - Ensure dispatcher passes through `JobSpecJSON` and uses an SBA runner image from config or node capability.
  - Optional: E2E or BDD scenario that creates a task that results in an SBA job and checks `job.result` contains `sba_result`.
- **Refs:** [cynode_sba.md](../tech_specs/cynode_sba.md), [worker_api.md](../tech_specs/worker_api.md), P2-09/P2-10 in [mvp_plan.md](../mvp_plan.md).

### 2. MCP Gateway: Allow Path or Scope (Medium)

- **Option A:** Document MVP scope: only `db.preference.*` are implemented; other tools (artifact.*, etc.) return 501 until later phase; update mvp_plan or mcp_gateway_enforcement so "full P2-02 allow path" is clearly deferred.
- **Option B:** Implement allow path for one sandbox/orchestrator tool (e.g. minimal `artifact.put` or stub) so gateway returns 200 and audit "allow" for that tool; unblocks PMA/SBA calling MCP for non-preference tools.
- **Refs:** [mcp_gateway_enforcement.md](../tech_specs/mcp_gateway_enforcement.md), [mcp_tool_call_auditing.md](../tech_specs/mcp_tool_call_auditing.md).

### 3. PMA Langchaingo Refactor (Medium)

- **Goal:** PMA uses langchaingo for LLM and tools (per mvp_plan Tech Spec Alignment); MCP tools as langchaingo tools; multiple tool calls where supported.
- **Refs:** [mvp_remediation_plan.md](mvp_remediation_plan.md) item 9, [cynode_pma.md](../tech_specs/cynode_pma.md), [pma_llm_tool_instructions_buildout_report.md](pma_llm_tool_instructions_buildout_report.md).

### 4. LangGraph Workflow (P2-04--P2-08) (Larger)

- Checkpoint and schema done.
  Next: workflow start/resume API (Go to Python LangGraph), graph nodes wired to MCP DB and Worker API, lease acquisition, Verify Step Result (PMA tasking Project Analyst / SBA).
- **Refs:** [langgraph_mvp.md](../tech_specs/langgraph_mvp.md), [mvp_plan.md](../mvp_plan.md) Phase 2 LangGraph tasks.

### 5. Doc and Dev_docs Housekeeping

- Update **mvp_plan.md** "Current Status" and "Remaining" to state orchestrator SBA job builder/dispatch not yet implemented; P2-09 and worker P2-10 done.
- **Pre-merge:** Resolve each file in `docs/dev_docs/` per README (move useful content to permanent docs, delete or archive temp docs).

### 6. Lower Priority (When Needed)

Items to pick up when product or Phase 2 needs them.

- **User API Gateway:** Task create attachments and task name normalization (remediation deferred).
- **Model warm-up:** Implement from [model_warm_up_proposal.md](model_warm_up_proposal.md) if first-chat latency is a product goal.
- **SBA baseline loading:** Wire SBA to load baseline from `agents/instructions/sandbox_agent/` (or job-supplied path) per [pma_llm_tool_instructions_buildout_report.md](pma_llm_tool_instructions_buildout_report.md).
- **Shared tool descriptions:** Single source for MCP tool names/descriptions for PMA and SBA instructions (plan in pma_llm_tool_instructions_plan.md).
- **Step executor (cynode-sse), SBA timeout extension, Worker API dedicated cmd:** Per remediation plan, Phase 2 or later.

## References

- [docs/mvp_plan.md](../mvp_plan.md) - MVP plan and task breakdown
- [docs/tech_specs/_main.md](../tech_specs/_main.md) - Tech spec index
- [docs/dev_docs/mvp_remediation_plan.md](mvp_remediation_plan.md) - Remediation status
- [docs/dev_docs/README.md](README.md) - Dev_docs cleanup rules
- [meta.md](../../meta.md) - Repo layout and conventions
