# E2E Inference Task Failing (Connection Refused :8081)

- [Symptom](#symptom)
- [Root Cause](#root-cause)
- [Fix Applied](#fix-applied)
- [What You Can Do](#what-you-can-do)

## Symptom

`just e2e` fails at **Test 5b** (inference task) with:

```text
Inference task did not complete: status=failed result={
  "status": "failed",
  "stdout": "{\"error\":\"Post \\"http://host.containers.internal:8081/v1/worker/jobs:run\\": dial tcp ...:8081: connect: connection refused\"...}"
}
```

## Root Cause

The control-plane dispatcher calls the worker API at the URL stored per-node in the DB (`worker_api_target_url`).
That URL is set when the node fetches config (GET /v1/nodes/config).
Previously, the handler **preferred the stored DB value** over the orchestrator env `WORKER_API_TARGET_URL`.
If the DB had a stale or wrong URL (e.g. port **8081** instead of **12090**), it was never corrected on later config fetches.

- Worker API default port is **12090** (see `docs/tech_specs/ports_and_endpoints.md`).
- Port 8081 is not used by CyNodeAI; a stale DB or old env can leave 8081 in the node record.

## Fix Applied

In `orchestrator/internal/handlers/nodes.go`, GetConfig now **prefers the orchestrator env** `WORKER_API_TARGET_URL` when non-empty, and only falls back to the node's stored URL when env is unset.
So:

- On each config fetch, if the control-plane has `WORKER_API_TARGET_URL` set (e.g. by compose to `http://host.containers.internal:12090`), that value is used and written to the DB.
- A restart of the stack with correct env fixes the node's URL on the next node config fetch.

## What You Can Do

If the failure persists, try the following.

**Re-run E2E.**
With the handler fix, when the node starts and fetches config, the control-plane's env (12090 from compose) is used and the DB is updated.
So `just e2e` should pass on the next run.

**Fresh DB (if needed).**
If you still see the wrong port, reset the DB and run again:

- `just clean-db` (or `./scripts/setup-dev.sh clean-db`)
- Then `just e2e`

**Check env.**
Ensure the control-plane container gets the right URL.
For the repo's E2E, `scripts/setup-dev.sh full-demo` exports `WORKER_API_TARGET_URL="http://${CONTAINER_HOST_ALIAS}:${WORKER_PORT}"` (WORKER_PORT=12090) before starting compose.

<!-- Date: 2026-02-26 -->
