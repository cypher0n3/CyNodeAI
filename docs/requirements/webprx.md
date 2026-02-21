# WEBPRX Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `WEBPRX` domain (Web Egress Proxy).
It covers the allowlist-based HTTP(S) forward proxy for sandbox dependency downloads and related policy and auditing.

## 2 Requirements

- **REQ-WEBPRX-0100:** The system MUST provide a Web Egress Proxy to support sandbox dependency downloads via HTTP and HTTPS without granting sandboxes direct, unrestricted internet egress.
  [CYNAI.WEBPRX.Doc.WebEgressProxy](../tech_specs/web_egress_proxy.md#spec-cynai-webprx-doc-webegressproxy)
  [CYNAI.WEBPRX.Purpose.WebEgressProxy](../tech_specs/web_egress_proxy.md#spec-cynai-webprx-purpose-webegressproxy)
  [CYNAI.WEBPRX.Protocol.WebEgressProxy](../tech_specs/web_egress_proxy.md#spec-cynai-webprx-protocol-webegressproxy)
  <a id="req-webprx-0100"></a>
- **REQ-WEBPRX-0101:** The Web Egress Proxy MUST enforce a default-deny destination policy for all sandbox web egress, allowing only allowlisted destinations and only for HTTP and HTTPS.
  [CYNAI.WEBPRX.Policy.WebEgressProxy](../tech_specs/web_egress_proxy.md#spec-cynai-webprx-policy-webegressproxy)
  <a id="req-webprx-0101"></a>
- **REQ-WEBPRX-0102:** The Web Egress Proxy MUST prevent access to private, local, or otherwise reserved network ranges, including via redirects and DNS rebinding.
  [CYNAI.WEBPRX.Policy.WebEgressProxy](../tech_specs/web_egress_proxy.md#spec-cynai-webprx-policy-webegressproxy)
  <a id="req-webprx-0102"></a>
- **REQ-WEBPRX-0103:** The Web Egress Proxy MUST emit audit records for allow and deny decisions with task context and destination metadata sufficient to attribute outbound traffic to `task_id` and `job_id`.
  [CYNAI.WEBPRX.Auditing.WebEgressProxy](../tech_specs/web_egress_proxy.md#spec-cynai-webprx-auditing-webegressproxy)
  <a id="req-webprx-0103"></a>
- **REQ-WEBPRX-0104:** The system MUST support task-scoped, temporary Web Egress Proxy allowlist entries with a bounded TTL.
  [CYNAI.WEBPRX.DynamicAllowlist.WebEgressProxy](../tech_specs/web_egress_proxy.md#spec-cynai-webprx-dynamicallowlist-webegressproxy)
  <a id="req-webprx-0104"></a>
- **REQ-WEBPRX-0105:** Agent-initiated requests to add temporary Web Egress Proxy allowlist entries MUST be policy-gated and MUST default to deny unless explicitly enabled.
  When enabled, strict validation MUST be enforced (hostname-only, no IP literals, no wildcards by default, bounded TTL, and constrained ports).
  [CYNAI.WEBPRX.DynamicAllowlist.WebEgressProxy](../tech_specs/web_egress_proxy.md#spec-cynai-webprx-dynamicallowlist-webegressproxy)
  <a id="req-webprx-0105"></a>
- **REQ-WEBPRX-0106:** The Web Egress Proxy MUST re-evaluate redirects against the same destination allowlist and private-network protections used for direct requests.
  [CYNAI.WEBPRX.Policy.WebEgressProxy](../tech_specs/web_egress_proxy.md#spec-cynai-webprx-policy-webegressproxy)
  <a id="req-webprx-0106"></a>
