# BOOTST Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `BOOTST` domain.
It covers bootstrap configuration requirements for orchestrator and node startup and configuration delivery.

## 2 Requirements

- REQ-BOOTST-0001: Bootstrap YAML optional; import idempotent; config and preferences read from PostgreSQL via MCP or gateway.
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  <a id="req-bootst-0001"></a>
- REQ-BOOTST-0002: MVP Phase 1 startup must ensure at least one inference-capable path is available before reporting ready.
  If no local inference is available and no external provider key is configured, the system must fail fast (or refuse to enter a ready state).
  [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md)
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-bootst-0002"></a>

- REQ-BOOTST-0100: On startup, the orchestrator MAY load a bootstrap YAML file when configured to do so.
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  <a id="req-bootst-0100"></a>
- REQ-BOOTST-0101: Bootstrap import MUST be idempotent and SHOULD support updates using versioned keys.
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  <a id="req-bootst-0101"></a>
- REQ-BOOTST-0102: After import, agents and systems MUST read configuration and preferences from PostgreSQL via MCP tools or the User API Gateway.
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  <a id="req-bootst-0102"></a>
