# E2E Full-Demo Workspace and Failures

- [Summary](#summary)
- [Workspace fix](#workspace-fix)
- [Other failures](#other-failures)

## Summary

- **Date:** 2026-03-06.
- **Context:** `just setup-dev full-demo --stop-on-success` reported 8 E2E failures (e2e_090, 110, 115, 140, 145, 192, 193, 194).

## Workspace Fix

SBA inference tests (e2e_140, e2e_145) failed with:

`Error: statfs /tmp/cynodeai-workspaces/<job_id>: no such file or directory`

The worker API already creates the per-job workspace in `prepareWorkspace` before calling the executor; the executor now also ensures the directory exists immediately before running the container in both:

- `runJobWithPodInference` (inference task in pod)
- `runJobSBAWithPodInference` (SBA inference in pod)

So the bind-mount source is present when podman runs.
Change: defensive `os.MkdirAll(workspaceDir, 0o700)` in `worker_node/cmd/worker-api/executor/executor.go` in both pod paths.

If the node runs inside a container and uses host podman (e.g. mounted socket), set `WORKSPACE_ROOT` to a path that is bind-mounted from the host so the created directory is visible to the host runtime.

## Other Failures

- **e2e_090 (inference task queued):** Task stayed `queued`; may be dispatcher/node readiness or timing.
- **e2e_110, 115, 192, 193, 194 (chat 502 / orchestrator_inference_failed):** User-gateway chat returns 502 or completion failed.
  Orchestrator resolves PMA **only** via worker-reported capability (`managed_services_status`); no env URL (REQ-ORCHES-0162). Chat works when the node has reported PMA ready with endpoint in its capability snapshot.
