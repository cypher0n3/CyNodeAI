# Task 8 Completion: MCP Gateway Authorization (Fail Closed)

- [Sub-Issues Addressed](#sub-issues-addressed)
- [Code](#code)
- [Tests and Gates](#tests-and-gates)
- [Deviations](#deviations)

## Sub-Issues Addressed

**Date:** 2026-03-29.

1. **No-token bypass:** When any agent bearer is configured, missing `Authorization` returns **401** (`tryAgentAllowlist`).
2. **PM allowlist:** PM role uses prefix allowlist (`pmToolPrefixes`); sandbox and PA use their maps/prefix rules.
3. **PA role:** `ToolCallAuth.PAToken` / `MCP_PA_AGENT_BEARER_TOKEN`; `AgentRolePA` with `paToolPrefixes` and `paBlockedPrefixes`.
4. **System skills:** `skills.update` / `skills.delete` return **403** when `skill.IsSystem` is true.

## Code

- `orchestrator/internal/mcpgateway/allowlist.go`, `handlers.go`; control-plane and mcp-gateway wire `PAToken`.
- `orchestrator/internal/config/config.go`: `MCPPAAgentBearerToken`.
- Unit tests: `allowlist_test.go`, `handlers_test.go` (including no-bearer matrix), `handlers_skills_test.go` (system skill update/delete).
- `scripts/test_scripts/config.py`: `MCP_PA_AGENT_BEARER_TOKEN`.
- `scripts/test_scripts/e2e_0812_mcp_agent_tokens_and_allowlist.py`: no-bearer **401**; optional PA checks when `MCP_PA_AGENT_BEARER_TOKEN` is set (`help.list` OK, `node.list` **403**).

## Tests and Gates

- `just lint-go`
- `just test-go-cover`
- `just test-bdd 45m`
- `just e2e --tags gateway,no_inference`
- `just e2e --tags control_plane,no_inference`

## Deviations

- **`e2e_0812` allowlist cases** skip when `WORKER_INTERNAL_AGENT_TOKEN` and `MCP_SANDBOX_AGENT_BEARER_TOKEN` are unset; full matrix requires those env vars on the dev stack.
- **System skill MCP rejection** is asserted in unit tests; live E2E does not seed a system skill for mutation.
- Plan text references `just test-bdd` with `@req_MCPGAT`; the `just test-bdd` recipe runs all `_bdd` modules (no tag filter in `justfile`).
