# Task 4 Control Plane E2E Execution Report

- [Command](#command)
- [Result](#result)
- [Log](#log)
- [Note on Red Phase Wording](#note-on-red-phase-wording)

## Command

**Date:** 2026-03-27 (UTC).

- `just setup-dev restart --force` with `WORKER_INTERNAL_AGENT_TOKEN` and `MCP_SANDBOX_AGENT_BEARER_TOKEN` exported so the control-plane container receives MCP allowlist tokens.
- `just e2e --tags control_plane`

## Result

- **Exit code:** 0.
- **Tests:** 9 ran; **OK (skipped=1)**.
- **Workflow API (`e2e_0500`):** All three tests passed (`state.TASK_ID` prereq succeeded; no `ensure_e2e_task failed` warning).
- **`e2e_0810_mcp_control_plane_tools`:** Both tests passed (direct control-plane and worker UDS proxy paths).
- **`e2e_0812_mcp_agent_tokens_and_allowlist`:** `test_mcp_bearer_pm_and_sandbox_allowlist` passed.
- **Skip (documented):** `test_pre_agent_token_in_node_config_matches_orchestrator` skipped with reason `managed_services not present (PMA host selection / inference); not an MCP token failure`.
  This is expected when the dev stack does not expose `managed_services` on node config without the full PMA host selection path.

## Log

- Full console log: `tmp/e2e_control_plane_task4_closeout_2026-03-27.log` (repo root).

## Note on Red Phase Wording

The plan text referenced "11 failures" from an earlier Bug 5 snapshot.
The current run had **zero** failures; symptoms were addressed by E2E and helper fixes (`artifact.get` arguments, `ensure_e2e_task`, `gateway_post_task_no_inference`, and stack env for MCP tokens).
