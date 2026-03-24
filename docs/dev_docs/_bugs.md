# Identified Bugs

## Overview

This doc outlines any discovered bugs that need to be addressed.

## Bug 1: ROCM Ollama on Nvidia GPU

On a system with an NVIDIA GPU, the ROCM Ollama container was launched, meaning either the logic for determining what container to run is wrong, or it's not actually being set dynamically properly.
Could also be that it's detecting both GPUs on the system (laptop with discrete GPU), and it isn't set up to handle that.

### Bug 1 Evidence

```text
~  podman ps
CONTAINER ID  IMAGE                                      COMMAND               CREATED        STATUS                  PORTS                               NAMES
ce28a546db63  docker.io/pgvector/pgvector:pg16           postgres              6 minutes ago  Up 6 minutes (healthy)  0.0.0.0:5432->5432/tcp              cynodeai-postgres
d07722d1bd4d  localhost/orchestrator_mcp-gateway:latest                        6 minutes ago  Up 6 minutes (healthy)  0.0.0.0:12083->12083/tcp, 8083/tcp  cynodeai-mcp-gateway
a52ecd7f9334  localhost/orchestrator_api-egress:latest                         6 minutes ago  Up 6 minutes (healthy)  0.0.0.0:12084->12084/tcp, 8084/tcp  cynodeai-api-egress
dafe1e39ed0d  localhost/cynodeai-control-plane:dev                             6 minutes ago  Up 6 minutes (healthy)  0.0.0.0:12082->12082/tcp, 8082/tcp  cynodeai-control-plane
c86178471826  localhost/cynodeai-user-gateway:dev                              6 minutes ago  Up 6 minutes (healthy)  0.0.0.0:12080->12080/tcp, 8080/tcp  cynodeai-user-gateway
6ffbd30f245b  docker.io/ollama/ollama:rocm               serve                 6 minutes ago  Up 6 minutes            0.0.0.0:11434->11434/tcp            cynodeai-ollama
ed62630c55f3  localhost/cynodeai-cynode-pma:dev          --role=project_ma...  4 minutes ago  Up 4 minutes (healthy)  8090/tcp                            cynodeai-managed-pma-main
b70e21293710  docker.io/library/alpine:latest            sh -c sleep 300       2 minutes ago  Up 2 minutes                                                stupefied_hofstadter
~  lspci | grep VGA
01:00.0 VGA compatible controller: NVIDIA Corporation GA104M [GeForce RTX 3080 Mobile / Max-Q 8GB/16GB] (rev a1)
07:00.0 VGA compatible controller: Advanced Micro Devices, Inc. [AMD/ATI] Cezanne [Radeon Vega Series / Radeon Vega Mobile Series] (rev c4)
```

### Bug 1 Likely Causes (From Implementation Inspection)

Likely causes of the bug.

#### Worker GPU Detection and Reporting

- **Fixed:** In `worker_node/internal/nodeagent/gpu.go`, `detectGPU` merges devices from both `detectNVIDIAGPU` and `detectROCmGPU` into a single `GPUInfo`.
  Reports all GPUs from all supported vendors (REQ-WORKER-0265).

#### Multiple GPUs and Total VRAM per Vendor

GPU preference MUST be driven by **model and/or VRAM**, not vendor alone.
When multiple GPUs of different types exist, the system MUST prefer the **vendor whose total VRAM (sum of all devices of that vendor) is greatest**.

##### Edge Cases

- **1 AMD GPU 20 GB vs 3 NVIDIA GPUs 12 GB each:** Total NVIDIA = 36 GB > 20 GB AMD -> select **cuda** (NVIDIA).
- **2 NVIDIA GPUs 12 GB + 6 GB vs 1 AMD GPU 16 GB:** Total NVIDIA = 18 GB > 16 GB AMD -> select **cuda** (NVIDIA).
- **1 NVIDIA GPU 12 GB vs 3 AMD GPUs 8 GB each:** Total AMD = 24 GB > 12 GB NVIDIA -> select **rocm** (AMD).
- **2 AMD GPUs 16 GB + 12 GB vs 1 NVIDIA GPU 20 GB:** Total AMD = 28 GB > 20 GB NVIDIA -> select **rocm** (AMD).
- **1 NVIDIA GPU 8 GB vs 1 AMD GPU 16 GB:** Total AMD = 16 GB > 8 GB NVIDIA -> select **rocm** (AMD).

So variant selection MUST use **total VRAM per vendor** (and optionally discrete vs integrated by model when reported), not "first device" or "vendor order."

#### Orchestrator Stack Default Image

The dev stack is started via **`just start`** (from `scripts/justfile`; invoked as `just setup-dev start` or `just scripts/start` from repo root).
There is no `node-up` recipe.

##### How `just start` Works

1. Optionally loads `$root/.env.dev`.
2. Sets `ollama_in_stack` if any arg is `--ollama-in-stack` or env `SETUP_DEV_OLLAMA_IN_STACK=1`.
3. Builds dev binaries and cynode-pma image.
4. Detects runtime (podman or docker) and host alias.
5. Exports Postgres, JWT, ports, NODE_*, etc., then runs **compose down** and **compose up** with optional `--profile ollama` when `ollama_in_stack` is true.
6. **After** compose up, exports `OLLAMA_IMAGE="${OLLAMA_IMAGE:-ollama/ollama:rocm}"` and other node-manager env vars, then starts the node-manager binary in the background.
7. Waits for worker-api healthz and user-gateway readyz.

Because `OLLAMA_IMAGE` is set only after compose runs, the **compose** Ollama service (when using `--ollama-in-stack`) uses whatever `OLLAMA_IMAGE` was in the environment at compose time (often unset, so the compose file default `ollama/ollama`).
The **rocm** default is used by the **node-manager** when it starts the Ollama container (e.g. when Ollama is not in the stack, or when the node-manager brings up inference and no existing `cynodeai-ollama` container is found).
On an NVIDIA-only or NVIDIA-primary machine, that default is wrong.

#### Spec / Requirement Violations

The intended behavior is: the **orchestrator** directs whether and how to start the local inference backend; the **node-manager** MUST NOT start Ollama until it has received that direction and MUST use the orchestrator-supplied variant/image.

- **REQ-WORKER-0253** ([worker.md](../requirements/worker.md#req-worker-0253)): The node MUST NOT start the OLLAMA container until the orchestrator has acknowledged registration and returned node configuration that **instructs** the node to start the local inference backend (**including backend variant**, e.g. ROCm for AMD or CUDA for Nvidia).
  The node-manager does wait for config and only starts when `inference_backend.enabled` is true.
  When `inference_backend.image` is absent, it falls back to `OLLAMA_IMAGE` env (e.g. `ollama/ollama:rocm` from the justfile), which can **override** the orchestrator-derived **variant**, violating the requirement that the orchestrator's instruction (variant) be honored.
- **REQ-ORCHES-0149** ([orches.md](../requirements/orches.md#req-orches-0149)): The orchestrator MUST return a node configuration payload that instructs the node **whether and how** to start the local inference backend.
  The orchestrator does set `inference_backend.variant` (and optionally `image`) from capability; the failure is either (1) wrong variant due to worker GPU detection order, or (2) the node ignoring variant when image is absent by using a single env default.
- **worker_node_payloads** ([worker_node_payloads.md](../tech_specs/worker_node_payloads.md) `inference_backend`): "The node MUST use this [variant] to select or configure the correct image or runtime (ROCM for AMD GPUs, CUDA for Nvidia GPUs when reported in capabilities)."
  When `inference_backend.image` is absent, the node MUST derive the image from **variant** (for Ollama: rocm -> `ollama/ollama:rocm`; cuda or cpu -> `ollama/ollama` or `ollama/ollama:latest`, since Ollama has no cuda tag), not from a single `OLLAMA_IMAGE` env that ignores variant.
- **CYNAI.ORCHES.InferenceContainerDecision** ([orchestrator_inference_container_decision.md](../tech_specs/orchestrator_inference_container_decision.md)): The orchestrator owns the deterministic decision; the node must not substitute an env default that contradicts the orchestrator-derived variant.

#### Re-Evaluation of Related Code Paths

- **Node-manager startup** (`worker_node/internal/nodeagent/nodemanager.go`): Registration and `FetchConfig` happen first; `maybeStartOllama` is called only after config is fetched and only when `nodeConfig.InferenceBackend != nil` and `nodeConfig.InferenceBackend.Enabled`.
  **Fixed:** When `image` is empty and `variant` is set, the node now derives image from variant (`rocm` -> `ollama/ollama:rocm`; `cuda`/`cpu` -> `ollama/ollama`).
    Falls back to `OLLAMA_IMAGE` only when both are empty.
- **Justfile** (`scripts/justfile`): Setting `OLLAMA_IMAGE="${OLLAMA_IMAGE:-ollama/ollama:rocm}"` before starting the node-manager makes the node-manager's fallback a single-variant default that overrides the orchestrator's variant when the orchestrator omits `image`.
  The justfile should not set a default that contradicts orchestrator-directed behavior; ideally the node would never need this env when the orchestrator supplies variant (and the node derives image from variant when image is absent).
- **Orchestrator** (`orchestrator/internal/handlers/nodes.go`): `variantAndVRAM` sums VRAM per vendor and selects variant for vendor with greatest total.
  Worker reports all devices; orchestrator computes correctly.

- **Variant selection by total VRAM:** Orchestrator `variantAndVRAM` sums `vram_mb` per vendor and selects variant for vendor with greatest total.
  Tie-break: prefer cuda over rocm.
  Uses total VRAM of chosen vendor for inference env (orchestrator_inference_container_decision.md).

### Bug 1 Desired Behavior (Design - Docs Only)

- **GPU preference by model and/or VRAM.**
  Variant selection (rocm vs cuda) MUST be driven by **model and/or VRAM detection**, not vendor alone.
  When multiple GPUs exist (same or different vendors), the system MUST prefer the **vendor whose total VRAM (sum of all devices of that vendor) is greatest**; optionally, within that, prefer discrete over integrated by model when reported.
  If only one vendor is present, use that variant; if no GPU, use cpu.
- **Multiple GPUs: total VRAM per vendor.**
  The worker MUST report **all** GPUs (all vendors) with per-device `vram_mb` so the orchestrator can sum total VRAM per vendor and select the variant for the vendor with the greatest total.
  Edge cases (NVIDIA dominant): 1 AMD 20 GB vs 3*NVIDIA 12 GB (36) -> cuda; 2*NVIDIA 12+6 GB vs 1 AMD 16 GB -> cuda.
  Edge cases (AMD dominant): 1 NVIDIA 12 GB vs 3*AMD 8 GB (24) -> rocm; 2*AMD 16+12 GB vs 1 NVIDIA 20 GB -> rocm; 1 NVIDIA 8 GB vs 1 AMD 16 GB -> rocm.
- **Ollama gets all GPUs of the chosen variant.**
  The Ollama container MUST be configured with access to **all** GPUs of the selected variant (e.g. all NVIDIA devices when variant is cuda), not only the first device.
  This allows multi-GPU inference and avoids underusing hardware.

Implementation details (worker reporting all vendors, orchestrator summing VRAM per vendor, discrete vs integrated by model) belong in a tech spec or implementation plan when code changes are made.

### Bug 1 Updated References (Requirements and Specs)

Authoritative req/spec sources that were updated to clarify correct behavior (orchestrator directs inference backend; node derives image from variant when image absent; node must not use env default that overrides variant):

- **Type:** Requirement
  - id / doc: REQ-WORKER-0253
  - location: [worker.md](../requirements/worker.md#req-worker-0253)
- **Type:** Requirement
  - id / doc: REQ-ORCHES-0149
  - location: [orches.md](../requirements/orches.md#req-orches-0149)
- **Type:** Requirement
  - id / doc: REQ-WORKER-0265 (report all GPUs with vram_mb when multiple vendors)
  - location: [worker.md](../requirements/worker.md#req-worker-0265)
- **Type:** Requirement
  - id / doc: REQ-ORCHES-0175 (Intel support deferred until post-MVP)
  - location: [orches.md](../requirements/orches.md#req-orches-0175)
- **Type:** Requirement
  - id / doc: REQ-WORKER-0266 (Intel support deferred until post-MVP)
  - location: [worker.md](../requirements/worker.md#req-worker-0266)
- **Type:** Spec (payload)
  - id / doc: node_configuration_payload_v1 `inference_backend`
  - location: [worker_node_payloads.md](../tech_specs/worker_node_payloads.md)
- **Type:** Spec (orchestrator decision)
  - id / doc: CYNAI.ORCHES.InferenceContainerDecision
  - location: [orchestrator_inference_container_decision.md](../tech_specs/orchestrator_inference_container_decision.md)
- **Type:** Spec (node startup)
  - id / doc: CYNAI.WORKER.NodeStartupProcedure
  - location: [worker_node.md](../tech_specs/worker_node.md#spec-cynai-worker-nodestartupprocedure)
- **Type:** Spec (payload config)
  - id / doc: CYNAI.WORKER.Payload.ConfigurationV1
  - location: [worker_node_payloads.md](../tech_specs/worker_node_payloads.md#spec-cynai-worker-payload-configuration-v1)

Feature files with scenarios added for variant/image behavior:

- [features/worker_node/node_manager_config_startup.feature](../../features/worker_node/node_manager_config_startup.feature): "Node manager starts inference backend with orchestrator-supplied variant when image is absent"
- [features/orchestrator/node_registration_and_config.feature](../../features/orchestrator/node_registration_and_config.feature): "GET config returns inference_backend with variant from vendor with most total VRAM (single vendor)" and "GET config returns inference_backend with variant from vendor with most total VRAM (mixed GPUs)"

#### Intel Discrete GPUs (Deferred Until Post-MVP)

Intel discrete GPUs (e.g. Arc) are accounted for in capability and variant selection in the specs (e.g. `gpu.devices.vendor` MAY include `Intel`; total VRAM per vendor may include Intel post-MVP).
**Implementation of Intel support is deferred until post-MVP** (no standard Ollama Intel image yet; ecosystem still evolving).
See [REQ-ORCHES-0175](../requirements/orches.md#req-orches-0175) and [orchestrator_inference_container_decision.md - Vendor Support](../tech_specs/orchestrator_inference_container_decision.md#spec-cynai-orches-inferencevendorsupportmvp).

### Bug 1 Suggested Tests (To Catch This and Prevent Regressions)

- **Unit tests (worker_node)**
  - **`worker_node/internal/nodeagent/gpu_test.go`**: Add tests that when **both** AMD and NVIDIA are present, `detectGPU` (or the merged capability) returns **all** devices from both vendors with per-device `vram_mb`.
  - Add a test that documents required behavior: capability report includes all GPUs so orchestrator can compute total VRAM per vendor.

- **Unit tests (orchestrator)**
  - **`orchestrator/internal/handlers/nodes_test.go`**: Add `variantAndVRAM` (or equivalent) cases for **total VRAM per vendor**:
    - Multiple devices, one vendor: e.g. 3*NVIDIA 12 GB each -> variant cuda, total VRAM 36 GB.
    - Mixed vendors (NVIDIA dominant): 1 AMD 20 GB vs 3*NVIDIA 12 GB (36) -> variant cuda; 2*NVIDIA 12+6 GB vs 1 AMD 16 GB -> cuda.
    - Mixed vendors (AMD dominant): 1 NVIDIA 12 GB vs 3*AMD 8 GB (24) -> variant rocm; 2*AMD 16+12 GB vs 1 NVIDIA 20 GB -> rocm.
    - Tie-break: when total VRAM is equal, use deterministic tie-break (e.g. policy or vendor order).

- **Feature file / BDD**
  - **Worker node**: Scenario that when the node reports GPU capability with **all** devices (multiple vendors, each with `vram_mb`), the node receives config with `inference_backend.variant` derived from the vendor with most total VRAM; node starts Ollama with that variant.
  - **Orchestrator**: Scenarios for mixed GPUs: (1) 1 AMD 20 GB + 3 NVIDIA 12 GB each -> variant `cuda`; (2) 1 NVIDIA 12 GB + 3 AMD 8 GB each -> variant `rocm`; (3) 2 AMD 16+12 GB + 1 NVIDIA 20 GB -> variant `rocm`.

- **E2E** (implemented)
  - **`e2e_0800_gpu_variant_ollama.py`**: Asserts the running Ollama container image tag matches the expected variant for the host GPU.
  - The Python script **independently** detects GPU via `nvidia-smi` and `rocm-smi`, sums VRAM per vendor, and selects the expected variant (cuda vs rocm) by the same logic as the worker (vendor with greater total VRAM).
  - Runs by default; skips when no GPU detected or Ollama container not running (e.g. `just e2e --tags gpu_variant` or full suite).
  - **Stack must NOT use** `SETUP_DEV_OLLAMA_IN_STACK` or `--ollama-in-stack` (node-manager must start Ollama, not compose).
  - Ollama image mapping: variant `rocm` -> `ollama/ollama:rocm`; variant `cuda` -> `ollama/ollama` (no separate cuda tag).
  - Traces: REQ-WORKER-0253, REQ-ORCHES-0149.

- **Justfile / compose**
  - Document or add a check: when `OLLAMA_IMAGE` is unset and the stack is brought up, either detect GPU and set `OLLAMA_IMAGE` (e.g. `ollama/ollama` or `ollama/ollama:latest` for NVIDIA; Ollama has no cuda tag) or document that users on NVIDIA use the default image.
  - An E2E or script that validates "Ollama container image matches expected variant for this host" would catch the wrong default.

### Bug 1 Implementation Status (Fully Spec-Compliant)

- **Worker GPU detection** (`gpu.go`): Reports **all** GPUs from all supported vendors (AMD and NVIDIA) in a single capability report.
  Each device includes `vendor`, `vram_mb`, and `features`.
  Compliant with REQ-WORKER-0265 and orchestrator_inference_container_decision.md line 195.
- **Orchestrator variantAndVRAM** (`nodes.go`): Sums `vram_mb` per vendor and selects variant for the vendor with greatest total VRAM.
  Tie-break: prefer cuda over rocm.
  Uses total VRAM of chosen vendor for inference env (OLLAMA_NUM_CTX).
  Handles multiple GPUs of same vendor.
  Compliant with orchestrator_inference_container_decision.md lines 164-165.
- **Node-manager image derivation** (`nodemanager.go`): When `inference_backend.image` is absent and `variant` is set, derives image: `rocm` -> `ollama/ollama:rocm`; `cuda`/`cpu` -> `ollama/ollama` (Ollama has no cuda tag).
  Compliant with worker_node_payloads and REQ-WORKER-0253.
- **Ollama container GPU access** (`main.go`): Uses `--gpus all` for CUDA.
  Compliant with "Ollama gets all GPUs of the chosen variant."
- **E2E validation** (`e2e_0800_gpu_variant_ollama.py`): Independently detects GPU, sums VRAM per vendor, asserts Ollama image tag.
  Stack must NOT use `SETUP_DEV_OLLAMA_IN_STACK`.
- **Unit tests:** Worker: `TestDetectGPU_ReportsAllDevicesWhenBothVendorsPresent`, `TestDetectGPU_SingleVendorReturnsAllDevices`.
  Orchestrator: `TestVariantAndVRAM_SumVRAMPerVendorMixedGPUs`, `TestVariantAndVRAM_MultiGPUSameVendorSumsTotal`, `TestVariantAndVRAM_MixedVendorsNVIDIADominant`, `TestVariantAndVRAM_TieBreakPrefersCuda`.
- **BDD:** Worker node scenario "Node manager starts inference backend with orchestrator-supplied variant when image is absent" passes.

## Bug 2: Cannot Launch Cynork TUI Without Connectivity

Logged-in users hit gateway APIs before the TUI renders; offline startup should show the UI first, per [cynork_tui.md](../tech_specs/cynork_tui.md) entrypoint rules.

### Bug 2 Summary

When the user is already logged in (access token present in config), `cynork tui` performs gateway I/O **before** the Bubble Tea UI starts.
If the machine is offline or the gateway is unreachable, the process exits with an error and the TUI never appears.
The product spec expects the fullscreen TUI to **render first** and defer thread creation until a gateway interaction (for example before the first completion), matching the in-session login path.

### Bug 2 Observed Behavior

- **Scenario:** Config has a valid-looking `token` (and optional `gateway_url`), but there is no route to the gateway (airplane mode, VPN down, wrong host, gateway stopped).
- **Symptom:** The command fails immediately with an error such as `thread: ...` wrapping a dial failure, timeout, or HTTP error from `POST /v1/chat/threads` or `GET /v1/chat/threads`.
- **Contrast:** With **no** token, `runTUI` skips the pre-flight thread step.
  `runTUIWithSession` sets `OpenLoginFormOnInit` and the TUI opens (login overlay), which aligns with the "show UI first" intent.

### Bug 2 Expected Behavior (Spec)

[cynork_tui.md](../tech_specs/cynork_tui.md) (`CYNAI.CLIENT.CynorkTui.EntryPoint`):

- `cynork tui` MUST start and render the TUI surface **unconditionally**, without requiring a valid login token at launch.
- Token resolution and validation MUST be deferred to the initial gateway connection **after** the TUI is visible.
- When `--resume-thread` is absent, the TUI MUST create a new thread **before the first completion request** (not necessarily before the process shows the UI).

Related requirements: [REQ-CLIENT-0197](../requirements/client.md#req-client-0197), [REQ-CLIENT-0202](../requirements/client.md#req-client-0202).

### Bug 2 Root Cause (Implementation)

In `cynork/cmd/tui.go`, `runTUI` calls `session.EnsureThread(tuiResumeThread)` when `cfg.Token != ""` **before** `tea.NewProgram` / `Run()`:

- `EnsureThread("")` -> `NewThread()` -> `POST /v1/chat/threads` (gateway required).
- `EnsureThread("<selector>")` -> `ListThreads` + resolution -> `GET /v1/chat/threads` (gateway required).

The TUI model already has an async path that runs the same work **after** the UI is up.
`ensureThreadCmd()` is invoked from `applyLoginResult` after a successful in-session login (`cynork/internal/tui/model.go`).
Errors are shown in scrollback via `applyEnsureThreadResult` instead of aborting the binary.
The startup path for an existing token does not use that; it blocks in the CLI layer instead.

### Bug 2 Suggested Fix

- **Defer thread ensure for logged-in users:** Remove the synchronous `EnsureThread` from `runTUI` when a token is present (or replace it with a no-op at the CLI level).
- **On TUI `Init`:** When `OpenLoginFormOnInit` is false and the session has a token, dispatch `ensureThreadCmd()` (same as post-login) so thread creation/listing happens in the Bubble Tea loop and failures surface in-scrollback.
- **First message:** Ensure behavior matches the spec: thread must exist before the first completion.
  If `EnsureThread` fails in the async path, the first send may need to either block on retry or show a clear error.
  Align with [Connection Recovery](../tech_specs/cynork_tui.md#connection-recovery) and thread semantics already documented.

### Bug 2 Files Involved

- **Pre-TUI network call:** `cynork/cmd/tui.go` (`runTUI`).
- **Thread ensure implementation:** `cynork/internal/chat/session.go` (`EnsureThread`, `NewThread`, `ResolveThreadSelector`).
- **Async ensure after login:** `cynork/internal/tui/model.go` (`ensureThreadCmd`, `applyEnsureThreadResult`, `applyLoginResult`).
- **Spec:** `docs/tech_specs/cynork_tui.md` (Entrypoint and Compatibility).

### Bug 2 Suggested Tests

- **Unit / cmd:** With a token in config and a gateway client that fails on `NewChatThread` (or transport that errors on list), `cynork tui` should still invoke `runTUIWithSession` / `tea` (tests may continue to stub `tuiRunProgram`).
  It should **not** return from `runTUI` before the TUI starts.
- **TUI model:** After fix, when token is present at init, `Init()` should schedule `ensureThreadCmd` (or equivalent) and offline errors appear as `ensureThreadResult` scrollback, consistent with `TestModel_Update_EnsureThreadResult_Error`.
- **E2E / manual:** Disconnect network, run `cynork tui` with saved credentials; expect fullscreen TUI with an error banner/scrollback line instead of immediate process exit.

### Bug 2 Implementation Status

**Fixed:** `runTUI` no longer calls `EnsureThread` before `tea.NewProgram`.
When a token is present, `Model.Init` schedules `ensureThreadCmd()` so thread creation/listing runs after the TUI starts.
Failures surface in scrollback via `applyEnsureThreadResult` (same as post-login).

## Bug 3: `cynork tui` `/auth login` Always Makes New Thread

In-session `/auth login` is tied to thread ensure; users often see a new thread or messaging that looks like a thread switch.

### Bug 3 Summary

Users report that after `/auth login` in the fullscreen TUI, a new chat thread appears (or it feels that way every time).

Investigation shows two layers.

First, when the gateway actually creates a thread (`POST /v1/chat/threads`) vs reusing the current one.

Second, UX: successful thread ensure after login always appends a `[CYNRK_THREAD_SWITCHED]` line, which can read like a new or switched thread even when no POST ran.

### Bug 3 Evidence (Code Paths)

The following subsections trace the login and thread-ensure chain.

#### Post-Login Always Schedules Thread Ensure

After a successful in-TUI login, `applyLoginResult` always combines `ensureThreadCmd()` with optional gateway health polling (`cynork/internal/tui/model.go`).
There is no branch that skips thread ensure on login success.

#### What `ensureThreadCmd` Does

`ensureThreadCmd` calls `Session.EnsureThread(m.ResumeThreadSelector)` with the selector captured at process start from `--resume-thread` (`cynork/cmd/tui.go` passes `tuiResumeThread` into `runTUIWithSession` -> `SetResumeThreadSelector`).

#### When `EnsureThread` Creates a New Thread or Not

`chat.Session.EnsureThread` (`cynork/internal/chat/session.go`):

- If `resumeSelector` is non-empty: resolve via `ListThreads` + selector and set `CurrentThreadID` (no `NewThread` unless resolution implies switching to an existing id).
- If `resumeSelector` is empty and `CurrentThreadID` is already set: returns without calling `NewThread()` (documented as keeping the thread after in-session re-login).
- If `resumeSelector` is empty and `CurrentThreadID` is empty: calls `NewThread()` -> `POST /v1/chat/threads`.

Unit coverage: `TestSession_EnsureThread_SkipsNewWhenThreadAlreadySet` asserts zero POSTs when `CurrentThreadID` is preset.

A literal "always creates a new thread on every `/auth login`" would only hold if `CurrentThreadID` is always empty at login success.

For example, a typical no-token-at-launch flow where `Init` never ran `ensureThreadCmd` (no token), so the first successful login is the first thread ensure.

### Bug 3 Spec Expectations

- [cynork_tui.md](../tech_specs/cynork_tui.md) Auth Recovery: after successful in-session re-authentication, the TUI SHOULD resume the same session state and return focus to the interrupted flow rather than forcing a full restart.
- Same doc Entry / thread semantics: default is a new thread unless `--resume-thread` is supplied; thread must exist before the first completion.

There is tension only if "same session state" is interpreted as always keeping the same thread after login.

The implementation can keep the current thread when `CurrentThreadID` is already set, but will create one when it is not (first login in a session with no prior ensure).

### Bug 3 Likely Causes (User-Visible)

Several distinct mechanisms can explain the report.

#### Cause 1: First Login in Session With No Prior Thread ID

Launch without a saved token (`OpenLoginFormOnInit` or user invokes `/auth login` before any thread exists).

`CurrentThreadID` is empty -> `EnsureThread("")` -> `NewThread()`.

This matches "every time I log in I get a new thread" for users who routinely start unauthenticated or re-open the login flow before `Init` has finished ensuring a thread (edge timing).

#### Cause 2: Misleading Scrollback After Every Successful Ensure

`applyEnsureThreadResult` appends `[CYNRK_THREAD_SWITCHED] Thread: <id>` for any successful result with non-empty `threadID`, including when `EnsureThread` did not create a thread and only confirmed the existing `CurrentThreadID` (`cynork/internal/tui/model.go`).

That reuses the same landmark family as `/thread switch` and "New thread", so users may believe a new thread was created when the id is unchanged.

#### Cause 3: `--resume-thread` Only at CLI Startup

`ResumeThreadSelector` is fixed for the process lifetime.

In-session `/auth login` does not accept a thread selector; users who need to land in an existing thread after login must have passed `--resume-thread` at launch or switch with `/thread switch` after ensure completes.

#### Cause 4: `/auth logout` Does Not Clear `CurrentThreadID`

Logout clears tokens on the client/provider but leaves `Session.CurrentThreadID` as-is (`cynork/internal/tui/slash.go` `authLogout`).

A subsequent login may keep the old id client-side (`EnsureThread` skip path).

That is the opposite problem (stale thread id) but relevant when validating "new thread" reports.

### Bug 3 Suggested Fix (Design - Docs Only)

- Differentiate scrollback messages: after `ensureThreadResult` from post-login ensure, use a line that does not imply a switch when `EnsureThread` returned without creating a thread (e.g. only emit `[CYNRK_THREAD_SWITCHED]` when `NewThread` ran or selector resolution changed `CurrentThreadID`).

  Alternatively, split landmarks: "Thread ready:" vs "Switched to thread:".

- Optional: skip `ensureThreadCmd` after login when `CurrentThreadID` is already set and gateway already had a thread ensure from `Init` (redundant network), if product wants zero extra churn; today the skip is already logical inside `EnsureThread` without a POST.

- Document that first in-session login with no thread id will create a thread; re-login with an existing `CurrentThreadID` should not POST.

### Bug 3 Files Involved

- `cynork/internal/tui/model.go` - `applyLoginResult`, `ensureThreadCmd`, `applyEnsureThreadResult`, `Init` (token path schedules `ensureThreadCmd`).
- `cynork/internal/chat/session.go` - `EnsureThread`, `NewThread`.
- `cynork/cmd/tui.go` - `runTUI`, `runTUIWithSession`, `SetResumeThreadSelector`.
- `cynork/internal/tui/slash.go` - `/auth login` -> `openLoginFormMsg`; `authLogout`.
- Spec: `docs/tech_specs/cynork_tui.md` (Auth Recovery, Entry / threads).

### Bug 3 Suggested Tests

- Session: already covered - `TestSession_EnsureThread_SkipsNewWhenThreadAlreadySet`.

- Model: add/update test: successful `loginResultMsg` with `Session.CurrentThreadID` preset and mock server counting `POST /v1/chat/threads` -> expect 0 POSTs and scrollback that does not claim a "new" thread (once messaging is split).

- Manual: logged-in TUI with an active thread -> `/auth login` re-auth -> confirm no second thread on server / single POST count if instrumented.

### Bug 3 Investigation Status

Investigated (2026-03-24): behavior traced in `cynork`; root cause is a mix of real `NewThread` when `CurrentThreadID` is empty at login success and UX conflation via a single "thread switched" style scrollback line after ensure.

Not fixed (code): awaiting product decision on scrollback wording and whether to pass thread context into the in-TUI login path.

## Bug 4: `cynork tui` Cannot Submit Slash or Shell Commands While Chat is Streaming

Composer Enter is ignored for non-empty input while `Loading` is true, including slash and shell prefixes.

### Bug 4 Summary

While the assistant turn is **streaming** (`Loading` true, streaming deltas in flight), the user **cannot submit** composer input that would run a **slash command** (`/…`) or **shell escape** (`!…`).
Pressing Enter with non-empty input is ignored.
Typing in the composer may still work; the failure is on **Enter** to execute.

This is stricter than users expect (run `/help`, `/thread list`, `!date`, etc. without waiting for the stream to finish) and may conflict with **streaming-aware composer** behavior described in [cynork_tui.md](../tech_specs/cynork_tui.md) (queued drafts, Ctrl+Enter send-now), which is not fully implemented in the current key path.

### Bug 4 Observed Behavior

- **Scenario:** A chat message is in progress and the model response is streaming (status busy / in-flight assistant line updating).
- **Symptom:** User types e.g. `/help` or `!pwd` and presses Enter; nothing happens (no command runs, no scrollback from slash/shell).
- **Contrast:** After `streamDoneMsg` clears `Loading`, the same input on Enter runs as usual.

### Bug 4 Root Cause (Implementation)

In `cynork/internal/tui/model.go`, `handleEnterKey` returns immediately when `m.Loading` is true and the trimmed composer line is non-empty:

```go
if m.Loading && line != "" {
    return m, nil
}
```

That gate runs **before** branches that detect `!` (shell) or `/` (slash) or normal chat send.
So **all** non-empty submits are suppressed during loading, not only "send another chat message."

`Ctrl+C` can cancel an active stream when `streamCancel` is set (`handleCtrlC`), but there is no path to "submit local-only commands" while streaming without waiting or canceling first.

### Bug 4 Expected Behavior (Spec Cross-Check)

[cynork_tui.md](../tech_specs/cynork_tui.md) (**Layout and Interaction** / streaming) describes:

- While the agent is streaming, **Enter** should add composer content to a **queue** (and clear the composer), not send immediately.
- **Ctrl+Enter** should **send now** and may interrupt streaming.

The codebase today does not implement that full queue/interrupt model in `handleEnterKey`; the early return blocks even local slash/shell execution.
A fix should either:

- **Narrow the guard:** e.g. only block **plain chat** sends while streaming, and still dispatch lines starting with `/` or `!` (and optionally treat other "local" slash handlers consistently), or
- **Align with the full spec:** implement queue + Ctrl+Enter interrupt, and define whether slash/shell bypass the queue or use dedicated keys.

Product should confirm whether slash/shell during streaming must work **without** canceling the stream (parallel UX) or only after **Ctrl+C** / stream end.

### Bug 4 Suggested Fix (Design - Docs Only)

- Remove or refine the blanket `m.Loading && line != ""` early return so **slash** and **shell** lines are still routed through `handleSlashLine` / shell handler while streaming, **or** document that users must cancel the stream first (weaker UX).
- If adopting the spec's queue: non-slash/non-`!` Enter queues; slash/shell may still execute immediately or be queued per product rules.
- Ensure `Loading` / `streamCancel` state stays consistent if a slash command triggers async work that also uses `Loading`.

### Bug 4 Files Involved

- `cynork/internal/tui/model.go` - `handleEnterKey`, `handleSlashLine`, `streamCmd`, `applyStreamDone` / `Loading` lifecycle.
- Spec: [cynork_tui.md](../tech_specs/cynork_tui.md) (streaming, composer keybindings); [cynork_tui_slash_commands.md](../tech_specs/cynork_tui_slash_commands.md) for slash semantics.

### Bug 4 Suggested Tests

- **Model:** With `m.Loading == true` and `m.streamCancel` non-nil (simulated streaming), Enter on `/help` (or a stub slash that does not need network) should either run the handler or match an explicit product decision; Enter on plain text may queue or noop per spec.
- **Manual:** Start a long streaming reply; type `/thread list` or `!echo hi` + Enter; confirm expected behavior (command runs vs blocked).

### Bug 4 Investigation Status

Not fixed (code).
Documented 2026-03-24: root cause is the `handleEnterKey` loading guard applied to all non-empty lines before slash/shell dispatch.

## Bug 5: MCP Gateway `skills.*` Tool Calls Return `task_id required`

All `skills.*` MCP tool calls (`skills.create`, `skills.list`, `skills.get`, `skills.update`, `skills.delete`) return `400: {"error":"task_id required"}` on both the direct control-plane path (`POST /v1/mcp/tools/call`) and the worker UDS internal proxy path.

### Bug 5 Summary

After the MCP consolidation (2026-03-22/23), the `skills.*` tool calls fail with `task_id required` even though:

1. The MCP gateway routing table in `handlers.go` defines `skills.*` entries with `{UserID: true}` (not `TaskID: true`).
2. The spec ([mcp_tooling.md](../tech_specs/mcp/mcp_tooling.md)) explicitly states skills tools take `user_id`, not `task_id`.
3. The spec ([mcp_gateway_enforcement.md](../tech_specs/mcp/mcp_gateway_enforcement.md)) states the gateway MUST ignore extraneous arguments like `task_id` on tools that do not use it.

The E2E tests (`e2e_0810_mcp_control_plane_tools.py`) pass `user_id` in the arguments as required, but the response is `task_id required` - suggesting a different code path (possibly `api-egress` `resolveSubjectFromTask` or a request-level `task_id` field) is intercepting the call before the per-tool handler runs.

### Bug 5 Evidence

```text
FAIL: test_mcp_tool_routes_round_trip (tool='skills.create')
AssertionError: 400 != 200 : {"error":"task_id required"}

FAIL: test_mcp_tool_routes_round_trip (tool='skills.list')
AssertionError: 400 != 200 : {"error":"task_id required"}

FAIL: test_mcp_tool_routes_round_trip (tool='skills.get')
AssertionError: 400 != 200 : {"error":"task_id required"}

FAIL: test_mcp_tool_routes_round_trip (tool='skills.update')
AssertionError: 400 != 200 : {"error":"task_id required"}

FAIL: test_mcp_tool_routes_round_trip (tool='skills.delete')
AssertionError: 400 != 200 : {"error":"task_id required"}
```

Same failures on the worker UDS path (6 failures including one gateway-unreachable assertion).
`e2e_0812` tests skip due to missing env vars (not directly related).

### Bug 5 Affected Files

- **MCP gateway handler:** `orchestrator/internal/mcpgateway/handlers.go` (routing table, `validateScopedIDs`, per-tool handlers).
- **API egress:** `orchestrator/cmd/api-egress/main.go` (`resolveSubjectFromTask` requires `task_id`).
- **E2E tests:** `scripts/test_scripts/e2e_0810_mcp_control_plane_tools.py` (11 failures), `scripts/test_scripts/e2e_0812_mcp_agent_tokens_and_allowlist.py` (2 skips).
- **E2E helpers:** `scripts/test_scripts/helpers.py` (`mcp_tool_call`, `mcp_tool_call_worker_uds`).
- **Specs:**
  - [mcp_tooling.md](../tech_specs/mcp/mcp_tooling.md) (skills tools use `user_id`, not `task_id`).
  - [mcp_gateway_enforcement.md](../tech_specs/mcp/mcp_gateway_enforcement.md) (extraneous argument handling).
  - [mcp_tools/skills_tools.md](../tech_specs/mcp_tools/skills_tools.md) (skills tool contracts).

### Bug 5 Investigation Required

1. Trace the request path for `helpers.mcp_tool_call("skills.create", ...)`: confirm whether the request hits the MCP gateway handler directly or goes through the api-egress.
2. Determine whether the MCP consolidation introduced a request-level `task_id` requirement (outside of per-tool arguments) that was not present before.
3. Determine the correct fix: either (a) the handler/middleware must not require `task_id` for tools that are user-scoped, or (b) the E2E helper must include `task_id` in the request envelope (not arguments) when the routing requires it.

### Bug 5 Investigation Status

Not fixed.
Documented 2026-03-24.
