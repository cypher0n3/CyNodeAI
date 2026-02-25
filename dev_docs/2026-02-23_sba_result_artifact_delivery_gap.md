# SBA Result and Artifact Delivery: Gap Analysis and Spec/MVP Updates

- [Summary](#summary)
- [What the Specs Say Today](#what-the-specs-say-today)
- [Gap Summary](#gap-summary)
- [Intended Design (Clarification)](#intended-design-clarification)
- [References](#references)

## Summary

**Date:** 2026-02-23. **Scope:** How SBA completion, result contract, and artifacts get from the sandbox to the orchestrator. **Status:** Gap identified; spec and MVP plan updates applied.

The SBA spec (cynode_sba.md) defines two delivery paths: (1) SBA reports via outbound calls through worker proxies (callback URL or MCP artifact.put), and (2) SBA writes to `/job/` and the node reads and forwards after container exit.
Neither the Worker API spec nor the MVP plan spelled out how result and artifacts are transmitted when using the node-mediated path (sync: node reads `/job/result.json` and `/job/artifacts/` and returns them in the same HTTP response).
There is no "container internal proxy" in the current spec; the node-mediated path does not require SBA to call the Worker API directly.
This doc records the gap and the spec/MVP plan updates that clarify the sync node-mediated flow and response shape for SBA jobs.

## What the Specs Say Today

Relevant specs are summarized below.

### CyNode_sba_sba (Result and Artifact Delivery)

Two delivery paths:

- **Outbound via worker proxies:** SBA reports status and uploads artifacts by calling orchestrator-mediated endpoints (e.g. job callback URL) or MCP tools (e.g. `artifact.put`) through worker proxies; runtime injects URLs/config.
- **Node-mediated delivery:** SBA MAY write result and artifacts under `/job/`; the node then reads and forwards after the container exits.

The spec does not define a "container internal proxy" (sidecar that SBA POSTs to).

### Worker API Spec (Worker_api_api)

Run Job returns a result in the same response; response body is defined for the simple case (exit_code, stdout, stderr) with no field for SBA result contract or artifacts.
Job lifecycle says the node MUST report completion and retain result until persisted; mechanism (response body vs callback) is implementation-defined.

### MVP Plan (P2-09, P2-10)

P2-10 says the node derives the Worker API response from the SBA result (e.g. reads `/job/result.json` on container exit) but did not describe transmission (response body), artifact handling, or orchestrator persistence.

## Gap Summary

- **Area:** Worker API response
  - gap: No defined field(s) for SBA result contract or artifacts for SBA-runner jobs.
- **Area:** Node-mediated flow
  - gap: Spec does not state that in sync mode the node builds the response from `/job/result.json` and `/job/artifacts/` and returns it in the same HTTP response.
- **Area:** Artifacts
  - gap: No spec for how `/job/artifacts/` files are delivered (inline in response, refs, or MCP only).
- **Area:** MVP plan
  - gap: No description of result/artifact transmission or orchestrator persistence for SBA jobs.

## Intended Design (Clarification)

For **MVP synchronous** implementation (node-mediated path):

1. SBA writes `/job/result.json` and optionally files under `/job/artifacts/`.
2. **Worker service:** The node MUST monitor the container for exit (wait for process exit or job timeout; on timeout, stop the container).
   After the container has exited (or been stopped), the node MUST read `/job/result.json` and `/job/artifacts/` (if present) from the host bind-mount and MUST derive job status from exit code and/or SBA result contract.
3. The node includes the SBA result contract and artifact refs/content in the Worker API response body (e.g. `sba_result`, `artifacts`) and returns it in the **same HTTP response** to the orchestrator.
4. The orchestrator persists the result (e.g. `jobs.result`) and artifact blobs/refs.
5. The node MUST NOT clear `/job/` until the response has been sent and SHOULD retain until orchestrator persistence is confirmed when the protocol supports it.

The requirement for the worker to **monitor for container exit** and to **read job results from the host path after exit** is now spelled out in worker_api.md (Node-Mediated SBA Result (Sync)).
Outbound path (SBA calls callback or MCP artifact.put) remains valid for async or in-progress reporting; not required for MVP sync.
A "container internal proxy" is not in the current spec and is not required for MVP.

## References

- `docs/tech_specs/cynode_sba.md` - Job lifecycle, Result and Artifact Delivery
- `docs/tech_specs/worker_api.md` - Run Job, Job Lifecycle and Result Persistence
- `docs/tech_specs/worker_node.md` - /job bind-mount
- `docs/mvp_plan.md` - P2-09, P2-10
