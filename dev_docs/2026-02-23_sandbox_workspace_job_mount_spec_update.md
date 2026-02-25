# Sandbox `\workspace` and `\job` Persistent Mount Spec Update

- [Summary](#summary)
- [Changes](#changes)
  - [Requirements Changes](#requirements-changes)
  - [Tech Spec Updates](#tech-spec-updates)
- [Implementation Questions for Product or Implementation Owner](#implementation-questions-for-product-or-implementation-owner)

## Summary

**Date:** 2026-02-23.
Specs and requirements were updated so that the worker node MUST map `/workspace` and `/job` inside sandbox containers to real, persistent filesystem paths on the host, with the host parent directory configurable by the user.

## Changes

Edited files and new requirement/spec text are listed below.

### Requirements Changes

- **`docs/requirements/worker.md`**
  - **REQ-WORKER-0250 (new):** The worker node MUST map container paths `/workspace` and `/job` to real, persistent filesystem paths on the host (bind mounts).
    The host parent directory under which the node creates these paths MUST be configurable by the user via the node startup configuration.
    Traces to `CYNAI.WORKER.SandboxWorkspaceJobMounts`.

### Tech Spec Updates

- **`docs/tech_specs/worker_node.md`**
  - New section **Sandbox workspace and job mounts** (Spec ID: `CYNAI.WORKER.SandboxWorkspaceJobMounts`):
    - `/workspace` and `/job` MUST be bind-mounted from real, persistent host directories.
    - **Configuration (how):**
      - **Primary:** Node startup YAML key `sandbox.mount_root` (string, optional).
        Absolute host path under which the node creates per-job subdirectories.
        When unset, implementation-defined default (e.g. under `storage.artifacts_dir` or `storage.state_dir`); default MUST be documented.
      - **Optional override:** Implementations MAY support an environment variable (e.g. `CYNODE_SANDBOX_MOUNT_ROOT`) at Node Manager startup; when both YAML and env are present, precedence MUST be documented.
    - Node MUST create host directories before starting the container and MUST keep them persistent until result processing is complete.
  - **Sandbox Settings:** New key `sandbox.mount_root` documented in User-Configurable Properties and example YAML (commented).

- **`docs/tech_specs/sandbox_container.md`**
  - **Filesystem and Working Directories:** Clarified that `/workspace` and `/job` are bind mounts to persistent host paths; host parent directory configurable per node; link to `worker_node.md#spec-cynai-worker-sandboxworkspacejobmounts`.
  - **Recommended paths:** Added Job directory `/job` (bind mount to persistent host path per worker node config).
  - **SBA Runner Image:** Sentence updated so the node "bind-mounts" `/workspace` and `/job` from real, persistent host paths; added cross-links.

## Implementation Questions for Product or Implementation Owner

1. **Per-job directory layout under `sandbox.mount_root`:** Should the node use a single parent with one subdir per job, e.g. `<mount_root>/<job_id>/workspace` and `<mount_root>/<job_id>/job`, or a different layout (e.g. `<mount_root>/workspace/<job_id>` and `<mount_root>/job/<job_id>`)?
   Spec leaves structure under the parent implementation-defined; documenting a standard layout will help operators and debugging.

2. **Environment variable override:** Do you want the optional env override (e.g. `CYNODE_SANDBOX_MOUNT_ROOT`) in the first implementation, and if so, should env override YAML or the reverse?

3. **Default when `sandbox.mount_root` is unset:** Preferred default: a subdirectory of `storage.artifacts_dir`, of `storage.state_dir`, or a new dedicated default (e.g. `/var/lib/cynode/sandbox_mounts`)?
   This affects upgrade path and docs.

4. **Cleanup policy:** When may the node delete or prune per-job directories under `mount_root` (e.g. after result persisted to orchestrator, after a TTL, or never by default)?
   Spec currently requires persistence until result processing is complete; retention beyond that is not specified.

5. **Rootless / permission handling:** For rootless Podman, the chosen `mount_root` must be writable by the rootless user.
   Should the spec or implementation require the node to create `mount_root` if missing (with documented ownership), or require the operator to pre-create it with correct permissions?
