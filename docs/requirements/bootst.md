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
  An inference path is either: (a) at least one worker node that has registered, been instructed to start inference, and has reported ready to the orchestrator, or (b) an LLM API key configured for the Project Manager Agent via the API Egress Server.
  If neither exists, the system MUST refuse to enter a ready state until an inference path becomes available.
  When the Project Manager Agent (cynode-pma) is enabled (default), the system MUST also refuse to enter a ready state until the PMA is started and has informed the orchestrator that it is online (and is reachable).
  [CYNAI.BOOTST.OrchestratorReadinessAndPmaStartup](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-orchestratorreadinessandpmastartup)
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
- **REQ-BOOTST-0104:** Deployments MUST support auto-start of the orchestrator on its host and of worker node services on worker hosts.
  On Linux, implementations MUST provide or document systemd unit files (user or system) for orchestrator and worker node services.
  On macOS, implementations MUST provide or document equivalent auto-start (e.g. launchd plist files) so that orchestrator and worker nodes can start on boot or on demand.
  [CYNAI.BOOTST.DeploymentAutoStart](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-deploymentautostart)
  [CYNAI.WORKER.DeploymentAutoStart](../tech_specs/worker_node.md#spec-cynai-worker-deploymentautostart)
  <a id="req-bootst-0104"></a>
- **REQ-BOOTST-0105:** The orchestrator control-plane and core orchestrator services (e.g. user-gateway, api-egress, cynode-pma when enabled) MUST be able to start and run independently of any OLLAMA or node-local inference container.
  OLLAMA (or equivalent local inference backend) is a node-side concern; the orchestrator MUST NOT require an OLLAMA container to be part of its own process or compose stack for correct operation.
  Dev or single-host convenience setups MAY run OLLAMA in the same compose as the orchestrator for local testing; production and multi-node deployments MUST use the prescribed startup sequence (orchestrator first, then nodes register and start inference when instructed).
  [CYNAI.BOOTST.OrchestratorIndependentStartup](../tech_specs/orchestrator_bootstrap.md#spec-cynai-bootst-orchestratorindependentstartup)
  [CYNAI.WORKER.NodeStartupProcedure](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure)
  <a id="req-bootst-0105"></a>
