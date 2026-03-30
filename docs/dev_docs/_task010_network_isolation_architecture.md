# Task 10: Pod Sandbox Network Isolation (Architecture)

## Decision (REQ-WORKER-0174)

**Chosen approach:** (a) Keep the inference **proxy sidecar** on the pod's default network so it can reach `OLLAMA_UPSTREAM_URL` (e.g. host/Ollama).
Run **sandbox and SBA workload containers** in the same pod with **`--network=none`**.

## Rationale

- Containers in a Podman pod share a network namespace by default, which allowed sandbox workloads to open arbitrary TCP connections to the internet alongside the proxy.
- **`--network=none`** removes routable interfaces from those containers only; **UDS** to the inference proxy remains available via the shared **`/run/cynode`** bind mount (filesystem socket), not IP.
- The proxy container is **not** started with `--network=none`, so upstream inference traffic is unchanged.

## Files

- `worker_node/internal/executor/executor.go`: `buildSandboxRunArgsForPod`, `buildSBARunArgsForPod`.

## Validation

- Unit tests assert `--network=none` appears in generated `podman run` argv for sandbox/SBA pod workloads.
- E2E `e2e_0325_sandbox_network_isolation.py` exercises a command-mode inference job that probes external connectivity.
