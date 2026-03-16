# Worker LLM Proxy: SBA Inference Capture and Reporting (Draft)

- [Document Overview](#document-overview)
  - [See Also](#see-also)
- [Scope and Goals](#scope-and-goals)
- [Definitions](#definitions)
- [Worker Inference Proxy Capture](#worker-inference-proxy-capture)
- [Proxy-To-Job Association](#proxy-to-job-association)
- [Opportunistic Redaction (Shared Library)](#opportunistic-redaction-shared-library)
- [SBA Non-Streaming Path: Cache, Redact, Forward](#sba-non-streaming-path-cache-redact-forward)
- [Worker-To-Orchestrator Inference Report](#worker-to-orchestrator-inference-report)
- [Orchestrator Redaction Before Storage](#orchestrator-redaction-before-storage)
  - [Redaction Behavior](#redaction-behavior)
- [Traceability](#traceability)

## Document Overview

- Spec ID: `CYNAI.WORKER.Doc.SbaInferenceCapture` <a id="spec-cynai-worker-doc-sbainferencecapture"></a>

This draft specifies how the worker-node **LLM (inference) proxy** used by the Sandbox Agent (SBA) captures inference request/response data, associates it with the running job, applies opportunistic secret redaction, and reports that data to the orchestrator.
The orchestrator redacts secrets (if any) before persisting SBA inference or chat logs.

**Status:** Draft; not yet promoted to `docs/tech_specs/` or `docs/requirements/`.

### See Also

- [worker_node.md](../tech_specs/worker_node.md): Node-local inference, unified UDS path, inference proxy
- [cynode_sba.md](../tech_specs/cynode_sba.md): SBA inference access, worker proxies
- [openai_compatible_chat_api.md](../tech_specs/openai_compatible_chat_api.md): Streaming redaction pipeline (PMA path)
- [worker_api.md](../tech_specs/worker_api.md): Job lifecycle, result persistence

## Scope and Goals

All local agents run by the worker Node Manager (managed services such as PMA and PAA, and sandbox containers including SBA) MUST use only UDS proxy endpoints for proxy and inference access; this is normative in [worker_node.md](../tech_specs/worker_node.md) (Unified UDS Path) and is assumed throughout this draft.

- **In scope:** Worker inference proxy behavior for SBA traffic: capture of request/response, binding of proxy endpoint to `job_id`/`task_id`, opportunistic redaction on the worker using a shared library, non-streaming path (cache then redact then forward), and reporting inference data to the orchestrator; orchestrator responsibility to redact before storing SBA inference or chat logs.
- **Out of scope:** PMA streaming redaction (already specified in openai_compatible_chat_api.md); API Egress-mediated SBA inference (capture/report may be defined later); schema for a new orchestrator table or artifact store for SBA inference logs (shape only suggested here).
- **Goals:** Single design for SBA inference visibility (audit, debugging, compliance), defense-in-depth redaction (worker and orchestrator), and shared redaction logic between gateway/PMA and worker.

## Definitions

- **Worker LLM proxy:** The node-local inference proxy that exposes a **UDS-only** endpoint to the sandbox (and to managed agents when they use node-local inference).
  It forwards OpenAI-compatible chat requests to the node's Ollama (or equivalent).
  Per [Unified UDS Path](../tech_specs/worker_node.md#spec-cynai-worker-unifiedudspath) and [Node-Local Inference](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalinference), all local agents run by the Node Manager (managed services and sandbox containers) MUST use only UDS proxy endpoints; no TCP is exposed to any agent or sandbox.
  Each SBA sandbox job that uses inference gets an inference proxy instance; the sandbox receives `INFERENCE_PROXY_URL=http+unix://...`.
- **SBA inference data:** The request body (e.g. chat completion request) and response body (e.g. chat completion response) for a single LLM call made by the SBA through the worker inference proxy.
- **Opportunistic redaction:** Best-effort detection and replacement of likely secrets (e.g. API keys, tokens, passwords) with a fixed placeholder (e.g. `SECRET_REDACTED`) before persistence or forwarding; same semantic as in [Streaming Redaction Pipeline](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streamingredactionpipeline) but applied to non-streaming payloads.

## Worker Inference Proxy Capture

- Spec ID: `CYNAI.WORKER.InferenceProxyCapture` <a id="spec-cynai-worker-inferenceproxycapture"></a>

The worker inference proxy that serves SBA traffic MUST capture, for each LLM request that passes through it:

- The full request body (e.g. OpenAI-compatible chat completion request).
- The full response body (e.g. chat completion response).

Because SBA does not use streaming for LLM responses (see [SBA Non-Streaming Path](#sba-non-streaming-path-cache-redact-forward)), the proxy MAY buffer the entire response before forwarding it to the SBA, so that capture and redaction can be done in one pass.

The proxy MUST associate each captured request/response with the job (and task) that is using that proxy instance; see [Proxy-to-Job Association](#proxy-to-job-association).

Captured data MUST be included in the worker's inference report to the orchestrator (see [Worker-to-Orchestrator Inference Report](#worker-to-orchestrator-inference-report)).
The worker MUST apply opportunistic redaction to the captured request and response before including them in the report; see [Opportunistic Redaction](#opportunistic-redaction-shared-library).

## Proxy-To-Job Association

- Spec ID: `CYNAI.WORKER.InferenceProxyJobBinding` <a id="spec-cynai-worker-inferenceproxyjobbinding"></a>

The Node Manager creates one inference proxy instance per sandbox job that uses inference, and injects the proxy's UDS URL into that sandbox (e.g. `INFERENCE_PROXY_URL`).
The worker MUST bind each such proxy instance to the corresponding `job_id` and `task_id` for the life of that job.

When the proxy captures a request/response (see [Worker Inference Proxy Capture](#worker-inference-proxy-capture)), it MUST tag the captured data with the same `job_id` and `task_id` so that the report to the orchestrator can associate each inference call with the correct job.

Implementation note: the proxy process or sidecar can receive `job_id` and `task_id` from the Node Manager at creation time (e.g. environment or config passed into the proxy); the proxy then attaches these identifiers to every captured inference record.

## Opportunistic Redaction (Shared Library)

- Spec ID: `CYNAI.WORKER.SharedRedactionLib` <a id="spec-cynai-worker-sharedredactionlib"></a>

Opportunistic secret redaction (detect-and-replace with a placeholder such as `SECRET_REDACTED`, and optionally record redaction kinds) MUST be implemented in a **shared library** consumed by:

- The orchestrator (User API Gateway) for the PMA chat path: streaming redaction per [Streaming Redaction Pipeline](../tech_specs/openai_compatible_chat_api.md#spec-cynai-usrgwy-openaichatapi-streamingredactionpipeline).
- The worker node for the SBA inference proxy path: non-streaming redaction per this draft.

The shared library MUST live in `go_shared_libs/` (e.g. a dedicated package such as `redact` or an extension of existing `secretutil`).
It MUST expose a single semantic: scan a string (or structured message content) for likely secrets, return redacted text and optional metadata (e.g. `redaction_applied`, `redaction_kinds`).
The same detection rules and placeholder MUST be used on both orchestrator and worker so that behavior is consistent and one implementation can be audited and updated.

Orchestrator and worker MUST NOT duplicate redaction logic; they MUST call the shared library.

## SBA Non-Streaming Path: Cache, Redact, Forward

- Spec ID: `CYNAI.WORKER.SbaNonStreamingRedactForward` <a id="spec-cynai-worker-sbanonstreamingredactforward"></a>

SBA does not require streaming LLM responses; it uses non-streaming chat completion.
The worker inference proxy for SBA MAY therefore:

1. Read the full request body from the SBA.
2. Optionally run opportunistic redaction on the request body (using the shared library); forward the (possibly redacted) request to the backend (Ollama).
3. Read the full response body from the backend.
4. Run opportunistic redaction on the response body using the shared library.
5. Forward the redacted response to the SBA.
6. Attach the redacted request and redacted response to the inference record for this job for reporting to the orchestrator.

This allows a single redaction pass per request and per response, and ensures that only redacted content is both forwarded to the SBA (for response) and sent to the orchestrator in the report.
Request redaction before forwarding to Ollama is optional (Ollama is node-local and does not persist); response redaction before forwarding to SBA and before reporting is required so that secrets in model output are not stored or sent onward.

## Worker-To-Orchestrator Inference Report

- Spec ID: `CYNAI.WORKER.InferenceReportToOrchestrator` <a id="spec-cynai-worker-inferencereporttoorchestrator"></a>

The worker MUST report SBA inference data to the orchestrator so that the orchestrator can persist it (after applying its own redaction; see [Orchestrator Redaction Before Storage](#orchestrator-redaction-before-storage)).

Report contract (conceptual):

- **When:** The worker MAY report inference data as part of job completion (e.g. attached to the same response or callback that delivers the job result), or via a dedicated inference-log endpoint that the orchestrator exposes.
  The exact endpoint and batching strategy are to be defined in the Worker API or orchestrator API spec when this draft is promoted.
- **Per-record shape:** Each record MUST include: `task_id`, `job_id`, model identifier, request payload (redacted), response payload (redacted), timestamps (e.g. request time, response time), and redaction metadata (`redaction_applied`, `redaction_kinds`) as produced by the worker's opportunistic redaction.
- **Association:** Every record MUST be tied to a single job via `job_id` and `task_id` so the orchestrator can store it in a job- or task-scoped store (e.g. SBA inference log table or task artifact).

The worker MUST NOT include unredacted request or response content in the report; only the output of the shared redaction library MUST be sent.

## Orchestrator Redaction Before Storage

- Spec ID: `CYNAI.ORCHES.SbaInferenceLogRedaction` <a id="spec-cynai-orches-sbainferencelogredaction"></a>

The orchestrator MUST redact secrets from SBA inference data (and any other chat or inference log content it stores) **before** persisting it.
This applies to data received from the worker's inference report and to any other path that writes inference or chat logs to persistent storage.

### Redaction Behavior

- Before writing to the database (or artifact store), the orchestrator MUST run the same shared opportunistic redaction library on the content to be stored.
- Only the redacted content MUST be persisted.
- The orchestrator SHOULD record redaction metadata (e.g. `redaction_applied`, `redaction_kinds`) in the audit or log row when supported by the schema (consistent with `chat_audit_log` for gateway chat).

This provides defense-in-depth: even if the worker's redaction were incomplete, the orchestrator does not store plaintext secrets.

## Traceability

- **Worker capture and report:** When promoted, this draft will trace to new or existing requirements in `docs/requirements/worker.md` (WORKER) for proxy capture, job binding, and reporting.
- **Orchestrator storage and redaction:** When promoted, will trace to `docs/requirements/orches.md` (ORCHES) for persistence of SBA inference logs and mandatory redaction before storage.
- **Shared redaction:** Will trace to `docs/requirements/stands.md` (STANDS) or `docs/requirements/usrgwy.md` (USRGWY) for the shared library contract and consistency with [REQ-USRGWY-0132](../requirements/usrgwy.md#req-usrgwy-0132) (redact before persist).
- **SBA inference path:** Will trace to [CYNAI.SBAGNT.WorkerProxies](../tech_specs/cynode_sba.md#spec-cynai-sbagnt-workerproxies) and [CYNAI.WORKER.NodeLocalInference](../tech_specs/worker_node.md#spec-cynai-worker-nodelocalinference).
