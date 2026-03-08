# Setup-Dev Startup Order Evaluation (2026-03-05)

## Purpose

Evaluate whether `just setup-dev start` (and related commands) bypass the intended system startup order.

Specs: `docs/tech_specs/orchestrator_bootstrap.md`, the startup sequence remediation plan, and the worker-managed PMA lifecycle proposal.

## Intended Startup Order (Specs)

The following prescribed sequence is taken from the referenced specs.

Sources: `orchestrator_bootstrap.md`, `2026-02-28_startup_sequence_remediation_plan.md`, and `2026-03-04_pma_worker_managed_lifecycle_spec_proposal.md`.

### Prescribed Sequence

1. **Orchestrator** (without OLLAMA): Postgres, control-plane, user-gateway (and optional services) start.
   No OLLAMA in the orchestrator stack; orchestrator MUST start independently of any node-local inference (CYNAI.BOOTST.OrchestratorIndependentStartup).
2. **Node:** Node Manager starts, registers with orchestrator, sends capability report.
3. **Orchestrator:** Acks registration, returns node configuration (including `inference_backend` when the node is inference-capable and not already using existing host inference).
4. **Node:** Fetches config, starts Worker API; then, only when instructed via config and when no existing inference service is present, starts inference container (e.g. OLLAMA).
   Reports config ack and readiness.
5. **Orchestrator:** When first inference path exists (worker reported ready and inference-capable, or API Egress key for PMA), orchestrator **instructs** a worker to start PMA via node configuration (managed services desired state).
6. **Worker:** Starts PMA container per orchestrator instruction, reports PMA ready and endpoint.
7. **Orchestrator:** Reports ready (e.g. `/readyz` 200) only after PMA is online (worker-reported).

Production and multi-node deployments MUST use this sequence.
Dev/single-host MAY use OLLAMA in the same compose as a convenience but MUST NOT be the only supported pattern.

## What Setup-Dev Actually Does

The following steps are implemented in the Python setup scripts.
Relevant entry points: `cmd_start`, `start_orchestrator_stack_compose`, `start_node`, `wait_for_orchestrator_readyz`.

1. Build binaries (`just build-dev`).
2. Compose up with `--profile optional` (and `--profile ollama` only when bypass): starts postgres, control-plane, user-gateway, mcp-gateway, api-egress; optionally ollama.
3. Start node: run node-manager binary (registers, fetches config, starts Worker API, starts PMA when instructed).
4. Wait for control-plane `/readyz` 200 (inference path + worker-reported PMA ready).

PMA is started by the worker when the orchestrator directs (no script bypass to start PMA via compose).

## Bypasses Identified

Setup-dev diverges from the prescribed order in the following ways.

### 1. OLLAMA in Orchestrator Stack (Bypass)

- Spec: Orchestrator MUST start without OLLAMA; nodes start inference when instructed (orchestrator_bootstrap.md, Worker Node Requirement; prescribed startup sequence).
- Setup-dev: Brings up compose with `--profile ollama`, so OLLAMA starts with the orchestrator stack.
- Effect: Single-host dev never exercises "orchestrator independent of OLLAMA."
  Node may coexist with an already-running OLLAMA instead of being instructed to start it via config.
  E2E and dev do not validate the prescribed node-driven inference startup.

### 2. PMA Started by Script, Not by Orchestrator Instruction (Bypass)

- Spec: Orchestrator MUST start PMA by instructing a worker via node configuration (managed services / PMA start bundle); worker starts and reports PMA ready (orchestrator_bootstrap.md, PMA Startup; worker-managed PMA proposal).
- Setup-dev: After node is up, the script runs `compose --profile pma up -d cynode-pma`.
  PMA runs as an orchestrator-side compose service, not as a worker-managed container.
- Effect: The "orchestrator instructs worker to start PMA" path is never exercised in setup-dev.
  PMA lifecycle and readiness are not driven by worker-reported managed service status.

### 3. Order and Control Flow (Partial Alignment)

- Aligned: Node starts after control-plane is listening; PMA is started only after the script has brought the node up (and after a 5s delay).
  So PMA does not start before a node exists.
  That matches the high-level ordering "inference path before PMA."
- Not aligned: The mechanism is script-driven (compose up pma) rather than orchestrator-driven (config payload, worker starts PMA, worker reports ready).
  Readiness is script-polling PMA `/healthz`, not orchestrator observing worker-reported PMA status.

## Summary

- **OLLAMA in orchestrator stack:** Intended: no (orchestrator independent).
  Setup-dev: yes (profile ollama).
  Bypass: yes.
- **Node starts inference:** Intended: when instructed via config.
  Setup-dev: node-manager logic (may start regardless).
  Bypass: depends on node-manager.
- **PMA who starts it:** Intended: orchestrator instructs worker.
  Setup-dev: script runs compose up cynode-pma.
  Bypass: yes.
- **PMA where it runs:** Intended: worker-managed container.
  Setup-dev: orchestrator compose service.
  Bypass: yes.
- **Readiness:** Intended: worker reports PMA ready.
  Setup-dev: script polls PMA /healthz.
  Bypass: yes (different mechanism).

## Conclusion

Setup-dev does bypass the intended startup order in two main ways:

1. OLLAMA is started with the orchestrator stack instead of by the node after receiving config.
2. PMA is started by the setup script via compose, not by the orchestrator instructing the worker.
  PMA runs as an orchestrator-side container, not worker-managed.

The ordering (control-plane, then node, then PMA) is respected at a coarse level.
The control flow and ownership (orchestrator vs script, worker-managed vs compose) do not match the normative specs.
Remediation remains as in `2026-02-28_startup_sequence_remediation_plan.md` and `2026-03-04_pma_worker_managed_lifecycle_spec_proposal.md`.
Optional dev convenience (OLLAMA in compose) should coexist with a spec-compliant path.
PMA should be started by the orchestrator via worker instruction.
Setup-dev should either drive that path or document the gap.

## References

- `docs/tech_specs/orchestrator_bootstrap.md` (Orchestrator Independent Startup, Worker Node Requirement, Orchestrator Readiness and PMA Startup)
- `docs/dev_docs/2026-02-28_startup_sequence_remediation_plan.md` (Prescribed Startup Sequence, Remediation Tasks)
- `docs/dev_docs/2026-03-04_pma_worker_managed_lifecycle_spec_proposal.md` (Required Startup Sequence Single-Host Dev)
- `docs/dev_docs/2026-03-04_pma_auto_start_root_cause.md` (PMA auto-start root cause; note compose now uses profile "pma" for cynode-pma)
- `scripts/setup_dev.py`, `scripts/setup_dev_impl.py` (cmd_start, start_orchestrator_stack_compose, start_node, wait_for_orchestrator_readyz)
- `orchestrator/docker-compose.yml` (profiles: ollama, optional, pma)
