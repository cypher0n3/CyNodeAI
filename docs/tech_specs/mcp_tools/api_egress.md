# API Egress MCP Tool

- [Document Overview](#document-overview)
- [Related Documents](#related-documents)
- [Definition Compliance](#definition-compliance)
- [Tool Contract](#tool-contract)
  - [`api.call` Operation](#apicall-operation)
- [Allowlist and Scope](#allowlist-and-scope)

## Document Overview

`api.call` is routed through the API Egress Server.
Agents use it to perform outbound API calls to configured providers without direct sandbox or agent access to credentials.

## Related Documents

- [API Egress Server](../api_egress_server.md)

## Definition Compliance

Tool definitions MUST conform to the project's MCP tool definition format: `Server: default`, `Name`, `Help`, `Scope`, `Tools` (single direct invocation).

## Tool Contract

- Spec ID: `CYNAI.MCPTOO.ApiEgress` <a id="spec-cynai-mcptoo-apiegress"></a>

### `api.call` Operation

- **Inputs**: Required `task_id` (uuid), `provider` (string), `operation` (string), `params` (object).
  Scope: `pm` or `both`.
- **Outputs**: Provider response (size-limited, schema-validated); MUST NOT include credentials or raw secrets.
- **Behavior**: Gateway enforces allowlist and scope, forwards (task_id, provider, operation, params) to API Egress Server; server resolves credentials for provider in request context and performs outbound call; gateway returns normalized result.
  See [api.call Algorithm](#algo-cynai-mcptoo-apicall).

#### `api.call` Algorithm

<a id="algo-cynai-mcptoo-apicall"></a>

1. Resolve caller identity and agent type; reject if unauthenticated.
2. Check tool allowlist and scope; deny if not allowed.
3. Validate `task_id`, `provider`, `operation`, `params` (schema and size limits); reject if invalid.
4. Forward request to API Egress Server with task context (and subject for credential resolution); server MUST resolve credentials server-side and MUST NOT expose them to the agent.
5. API Egress Server performs outbound call to provider with resolved credentials; returns normalized response.
6. Gateway enforces response size limit and schema validation; strip any credential or sensitive headers from result.
7. Emit audit record (task_id, provider, operation, decision, outcome); return result or error (no secrets in error).

#### `api.call` Error Conditions

- Invalid args; provider or operation not configured/allowed; credential resolution failure; outbound call failure or timeout; response over size limit; access denied.

#### Traces To

- [REQ-MCPTOO-0109](../../requirements/mcptoo.md#req-mcptoo-0109)
- [REQ-MCPTOO-0110](../../requirements/mcptoo.md#req-mcptoo-0110)

## Allowlist and Scope

- **Allowlist**: PMA, PAA, and optionally Worker agent per [MCP Gateway Enforcement](../mcp/mcp_gateway_enforcement.md); subject to API egress policy and access control.
