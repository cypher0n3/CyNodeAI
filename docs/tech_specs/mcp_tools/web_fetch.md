# Web Fetch MCP Tool

- [Document Overview](#document-overview)
- [Definition Compliance](#definition-compliance)
- [Tool Contract](#tool-contract)
  - [`web.fetch` Operation](#webfetch-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

`web.fetch` is a policy-controlled, sanitized fetch implemented by the Secure Browser Service.
Agents use it to retrieve web content without direct, unfiltered internet access.

Related documents

- [Secure Browser Service](../secure_browser_service.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation).

## Tool Contract

- Spec ID: `CYNAI.MCPTOO.WebFetch` <a id="spec-cynai-mcptoo-webfetch"></a>

### `web.fetch` Operation

- **Inputs**: Required `task_id` (uuid), `url` (string).
  Scope: `pm` or `both`.
- **Outputs**: Sanitized content (e.g. text or safe HTML); size-limited; MUST NOT include secrets.
- **Behavior**: Gateway enforces allowlist and scope, forwards URL to Secure Browser Service (or equivalent), which fetches under policy and sanitizes; gateway returns size-limited result.
  See [web.fetch Algorithm](#algo-cynai-mcptoo-webfetch).

#### `web.fetch` Algorithm

<a id="algo-cynai-mcptoo-webfetch"></a>

1. Resolve caller identity and agent type; reject if unauthenticated.
2. Check tool allowlist and scope; deny if not allowed.
3. Validate `task_id` and `url` (scheme, host allowlist, or policy); reject disallowed URLs.
4. Forward request (task_id, url) to Secure Browser Service; service fetches URL under policy (rules, allowlist), sanitizes content, and returns result.
5. Enforce response size limit on content returned to agent; truncate or error per policy.
6. Emit audit record (task_id, url, decision, outcome); return result or error (no secrets in error).

#### `web.fetch` Error Conditions

- Invalid args; URL not allowed by policy; fetch timeout or failure; content failed sanitization; size over limit; access denied.

#### Traces To

- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)
- [REQ-MCPTOO-0110](../../requirements/mcptoo.md#req-mcptoo-0110)

## Allowlist and Scope

- **Allowlist**: PMA, PAA, and optionally Worker agent per [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md); subject to access control rules for `web.fetch` or equivalent action.
