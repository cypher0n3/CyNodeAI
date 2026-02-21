# BOOTST Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `BOOTST` domain.
It covers bootstrap configuration requirements for orchestrator and node startup and configuration delivery.

## 2 Requirements

- **REQ-BOOTST-0001:** Bootstrap YAML optional; import idempotent; system configuration and policy, and user preferences read from PostgreSQL via MCP or gateway.
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  <a id="req-bootst-0001"></a>
- **REQ-BOOTST-0002:** MVP Phase 1 startup must ensure at least one inference-capable path is available before reporting ready.
  If no local inference is available and no external provider key is configured, the system MUST refuse to enter a ready state until an inference-capable path becomes available.
  [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md)
  [external_model_routing.md](../tech_specs/external_model_routing.md)
  <a id="req-bootst-0002"></a>
- **REQ-BOOTST-0100:** On startup, the orchestrator MAY load a bootstrap YAML file when configured to do so.
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  <a id="req-bootst-0100"></a>
- **REQ-BOOTST-0101:** Bootstrap import MUST be idempotent and SHOULD support updates using versioned keys.
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  <a id="req-bootst-0101"></a>
- **REQ-BOOTST-0102:** After import, agents and systems MUST read system configuration and policy, and user preferences from PostgreSQL via MCP tools or the User API Gateway.
  [CYNAI.BOOTST.BootstrapSource](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-bootstrapsource)
  <a id="req-bootst-0102"></a>
- **REQ-BOOTST-0103:** Bootstrap YAML MUST support seeding the Project Manager model default for local inference as a system setting.
  The system setting key MUST be `agents.project_manager.model.local_default_ollama_model`.
  [orchestrator_bootstrap.md](../tech_specs/orchestrator_bootstrap.md)
  <a id="req-bootst-0103"></a>
