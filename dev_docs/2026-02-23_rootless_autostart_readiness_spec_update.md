# Rootless, Auto-Start, and Worker Readiness Spec Update

- [Summary](#summary)
- [Changes](#changes)

## Summary

**Date:** 2026-02-23.
Specs and requirements were updated for: (1) worker node rootless-by-default when the runtime supports it, (2) auto-start of orchestrator and worker services on Linux (systemd) and macOS (launchd or equivalent), and (3) worker node startup checks before reporting ready.

## Changes

Edits are listed by topic below.

### Rootless When Supported

- **REQ-WORKER-0251** (new): When the container runtime supports rootless execution (e.g. Podman), the worker node MUST use rootless operations for sandbox containers (MUST NOT run as root).
  Operator MAY override via `sandbox.rootless: false` as a documented exception.
- **worker_node.md**: New section Sandbox rootless execution (Spec ID `CYNAI.WORKER.SandboxRootless`).
  Node Manager and Sandbox Settings updated: rootless MUST when runtime supports it; `sandbox.rootless` default true when supported, `false` only as exception.

### Auto-Start (Systemd and Macos)

- **REQ-BOOTST-0104** (new): Deployments MUST support auto-start of orchestrator on its host and worker node services on worker hosts.
  Linux: systemd unit files (user or system).
  macOS: equivalent (e.g. launchd plist).
- **orchestrator_bootstrap.md**: New section Deployment and auto-start (`CYNAI.BOOTST.DeploymentAutoStart`); references `orchestrator/systemd/README.md`, requires launchd/docs for macOS.
- **worker_node.md**: New section Deployment and auto-start (`CYNAI.WORKER.DeploymentAutoStart`); references `worker_node/systemd/README.md`, requires launchd/docs for macOS.

### Worker Startup Checks Before Ready

- **REQ-WORKER-0252** (new): Worker node MUST perform startup checks (e.g. can deploy containers) before reporting ready; `GET /readyz` returns 200 only after checks pass.
- **worker_node.md**: New section Node Startup Checks and Readiness (`CYNAI.WORKER.NodeStartupChecks`).
  Required checks: container runtime (create/run a container), sandbox mount root writable when sandbox enabled, Worker API listening, orchestrator registration/config when required.
  Startup procedure updated to require checks before reporting ready.
- **worker_api.md**: Health checks trace to REQ-WORKER-0252; `GET /readyz` defined as returning 200 only after node startup checks pass.
