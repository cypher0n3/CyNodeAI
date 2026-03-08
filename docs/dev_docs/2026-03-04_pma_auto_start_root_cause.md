# Root Cause: PMA Auto-Started During Setup-Dev Full-Demo

## Summary

When running `just setup-dev full-demo --stop-on-success`, the container `cynodeai-cynode-pma` is started automatically with the orchestrator stack.
Per the bootstrap spec, PMA must **not** start until the orchestrator has at least one inference path (worker reported ready and inference-capable, or API Egress key for PMA).
The orchestrator is supposed to **start** PMA after that condition is met.

## Why It Happens

PMA is part of the default compose services and is brought up with the stack.

### Compose Defines PMA as a Default Service

   In `orchestrator/docker-compose.yml`, `cynode-pma` is a top-level service with no profile.
    It is built and started whenever the stack is brought up.

### `just setup-dev start` / Full-Demo Runs Full Compose Up

   `scripts/setup_dev_impl.py` -> `start_orchestrator_stack_compose()` runs:

- `compose -f orchestrator/docker-compose.yml --profile ollama --profile optional up -d`
   That starts **all** default services: postgres, control-plane, user-gateway, **cynode-pma** (plus profile services).
    So PMA starts at stack bring-up, before any worker has registered or sent a capability bundle.

### User-Gateway Depends on CyNode-Pma

   user-gateway has `depends_on: cynode-pma: condition: service_healthy`, so compose starts and waits for PMA as part of bringing up the stack.

### Control-Plane is Explicitly Not Starting PMA in Compose

   In the same compose file, control-plane has `PMA_ENABLED: "false"` and the comment "PMA runs as separate container; do not start cynode-pma subprocess inside this container."
    So the intended dev model is "PMA as a separate container", but that container is currently started by compose up, not by the orchestrator when the first inference path exists.

## Spec (Correct Behavior)

From `docs/tech_specs/orchestrator_bootstrap.md`:

- **PMA Startup:** The orchestrator MUST start the Project Manager Agent (cynode-pma) when the **first** inference path becomes available (worker reported ready and inference-capable, or API Egress key for PMA).
  The orchestrator MUST NOT start PMA before at least one of these conditions is satisfied.

- **Inference path:** Either (1) a worker that has registered, been instructed to start inference, has started its inference container, and has **reported ready** (e.g. config ack with status applied), or (2) an LLM API key for PMA via API Egress.

So the intended sequence is: stack up (no PMA) -> node starts and registers -> capability bundle / config ack -> orchestrator sees inference path -> **orchestrator** starts PMA -> PMA informs orchestrator -> orchestrator reports ready.

## Control-Plane Implementation (When Not in Compose)

The control-plane already implements this when it runs with PMA as a subprocess:

- `startPMAWhenInferencePathReady()` waits for `inferencePathAvailable()` (dispatchable node or external key), then calls `pmasubprocess.Start()` to run the cynode-pma binary.
  So when PMA is not "separate container", the orchestrator starts PMA only after the first inference path exists.

In the compose-based dev setup at the time of writing, control-plane had `PMA_ENABLED=false`, so it never started the subprocess; the expectation was that "PMA runs as separate container", but that container was started by compose, not by the orchestrator in response to an inference path.

Note: PMA is a core system feature and is always required.
Any documentation suggesting PMA can be disabled should be treated as legacy drift and updated.

## Conclusion

PMA is auto-started because it is a **default** service in `orchestrator/docker-compose.yml` and is brought up by `compose up -d` during setup-dev.
That violates the bootstrap spec, which requires the orchestrator to start PMA only after the first inference path is available (e.g. after the worker has registered and reported ready with its capability bundle).

## Possible Fixes (For Implementation)

Options to align dev startup with the bootstrap spec:

### Profile for PMA

   Put `cynode-pma` behind a compose profile (e.g. `pma`).
    Do **not** use that profile in `just setup-dev start` / full-demo.
    Then either:

- Have setup-dev, after starting the node and waiting for it to register, start the PMA container (e.g. second compose up with profile, or `runtime start cynodeai-cynode-pma`), so PMA starts only after the stack and node are up; or
- Document that for spec-compliant dev flow, start PMA manually (or via a script) after the node is registered.

### Remove User-Gateway Dependency on CyNode-Pma

   If PMA is not started with the stack, user-gateway must not `depends_on: cynode-pma` (or the dependency must be optional / best-effort).
    The gateway already handles PMA unreachable (e.g. model=cynodeai.pm when PMA is down).

### Orchestrator-Triggered Start of PMA Container

   For full spec compliance in containerized dev: control-plane would need a way to start the PMA container when `inferencePathAvailable` becomes true (e.g. call a small helper that runs `podman start cynodeai-cynode-pma` or `compose up -d cynode-pma`).
    That would require either Docker-out-of-Docker, a host-mounted socket, or a separate "stack manager" service that control-plane can call.

## Fix Applied (2026-03-04)

- **Compose:** `cynode-pma` is behind profile `pma`; user-gateway no longer `depends_on` cynode-pma.
  Initial `compose up -d` does not start PMA.
- **Setup script:** (Historical.) A bypass previously started PMA via compose; that option was removed.
  PMA is now only started by the worker when the orchestrator directs.
- **Teardown:** Compose down and rm list include profile `pma` and container `cynodeai-cynode-pma` so PMA is stopped and removed with the stack.

**Date:** 2026-03-04
