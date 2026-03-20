# Secure Web Search MCP Tool

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contract](#tool-contract)
  - [`web.search` Operation](#websearch-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

Secure web search is a policy-controlled MCP tool that allows agents to run search queries without direct, unfiltered access to the open internet.
Results are returned through a secure path (e.g. Secure Browser Service or a dedicated search proxy) so that only sanitized or allowlisted search provider responses are exposed to the agent.

Related documents

- [Secure Browser Service](../secure_browser_service.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation).

## Tool Contract

- Spec ID: `CYNAI.MCPTOO.SecureWebSearch` <a id="spec-cynai-mcptoo-securewebsearch"></a>

### `web.search` Operation

- **Inputs**: Required `task_id` (uuid), `query` (string); optional `limit` (int), `safe_filter` (boolean).
  Scope: `pm` or `both`.
- **Outputs**: Search results (titles, snippets, URLs) in size-limited, policy-compliant format; MUST NOT expose raw internet.
- **Behavior**: Gateway enforces allowlist and scope, forwards query to secure search path (Secure Browser Service or dedicated search proxy); only sanitized or allowlisted results returned.
  See [web.search Algorithm](#algo-cynai-mcptoo-websearch).

#### `web.search` Algorithm

<a id="algo-cynai-mcptoo-websearch"></a>

1. Resolve caller identity and agent type; reject if unauthenticated.
2. Check tool allowlist and scope; deny if not allowed.
3. Validate `task_id` and `query` (non-empty, size-limited); apply optional `limit` and `safe_filter`.
4. Forward search request to secure search endpoint (same policy-controlled path as web.fetch or dedicated search proxy); MUST NOT allow direct internet search from agent.
5. Receive sanitized or allowlisted results (titles, snippets, URLs); enforce response size limit.
6. Emit audit record (task_id, action web.search, decision, outcome); return result or error (no secrets).

#### `web.search` Error Conditions

- Invalid args; query over size limit; search endpoint unavailable; policy rejected query; result set over limit; access denied.

#### Traces To

- [REQ-MCPTOO-0119](../../requirements/mcptoo.md#req-mcptoo-0119)
- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)
- [REQ-MCPTOO-0110](../../requirements/mcptoo.md#req-mcptoo-0110)

## Allowlist and Scope

- **Allowlist**: PMA, PAA, and optionally Worker agent per [MCP Gateway Enforcement](../mcp_gateway_enforcement.md); subject to access control rules for `web.search`.
