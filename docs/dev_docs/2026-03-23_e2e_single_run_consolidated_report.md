# E2E Single-Module Run: Consolidated Failures and Skips

- [Scope](#scope)
- [Stance: controlled stack means bugs, not excuses](#stance-controlled-stack-means-bugs-not-excuses)
- [How the run was executed](#how-the-run-was-executed)
- [Modules that passed](#modules-that-passed)
- [Modules that failed](#modules-that-failed)
- [Skips and whether they are acceptable](#skips-and-whether-they-are-acceptable)
- [Interpretation](#interpretation)

## Scope

This report consolidates results from re-running the Python E2E modules touched by the
`prepare_e2e_cynork_auth()` / gateway-login sidecar work, using `just e2e --no-build --single <module>`.
It captures what **still failed** or was **skipped** during that re-run and classifies it as **product
or automation debt**, not as an unavoidable external condition.

## Stance: Controlled Stack Means Bugs, Not Excuses

We **control** the dev and CI stack: compose files, `just` recipes, images we build, services we
start, and configuration we ship.
Aside from **hardware limits** (disk, RAM, GPU, raw CPU), anything
that prevents a test from passing is a **bug** somewhere in that system: wrong or missing setup
automation, a service misconfiguration, a flaky implementation, a missing image in the pipeline, or
a test that no longer matches the spec.

Framing failures as "the environment" externalizes responsibility and is **wrong** for this repo.
The report below uses **symptom buckets** to narrow **where** to fix (gateway, worker, images,
tests), not to dismiss failures.

## How the Run Was Executed

1. **Batch A:** Sequential `just e2e --no-build --single <module>` for
   `e2e_0050`, `e2e_0420`, `e2e_0490`, `e2e_0500`, then `e2e_0510`.
    The shell exited on first
   failing recipe after `0510`.
2. **Batch B:** Same pattern for `e2e_0530` through `e2e_0780`, with failures counted but not
   aborting the loop.

Total wall time for Batch B was large when chat and streaming paths blocked or retried for a long
time.
Slow runs are a **signal** (timeouts, hung dependencies), not a separate category of excuse.

## Latest Update (2026-03-24)

- **E2E (post inference / GPU path recovery):** A follow-up single-module pass reported **11 modules OK** and **6 failed** (narrower than the larger Batch B sweep summarized below for March 23).
  Remaining failures still map to the symptom buckets in [Modules that failed](#modules-that-failed).
- **PMA `node.list`:** The response `tool routing not implemented` came from the control-plane MCP gateway when `node.list` had no entry in `mcpToolRoutes` (`orchestrator/internal/mcpgateway/handlers.go`).
  That was a **missing handler registration**, not an allowlist-only issue. **`node.list`** and **`node.get`** are now routed in `orchestrator/internal/mcpgateway/node_tools.go` against the orchestrator node registry (`ListNodes`, `GetNodeBySlug`). **`node.refresh_config`** is still not implemented at the gateway and will return **501** until added.

## Modules That Passed

These completed with **OK** (no failures in the module):

- `e2e_0050_auth_whoami`
- `e2e_0420_task_create` (all three tests)
- `e2e_0490_skills_gateway`
- `e2e_0500_workflow_api` (all three tests)
- `e2e_0570_pma_chat_context`
- `e2e_0720_sba_task_result_contract` (see [Skips](#skips-and-whether-they-are-acceptable))
- `e2e_0765_tui_composer_editor`
- `e2e_0770_auth_refresh`
- `e2e_0780_auth_logout`

These show that **local auth via E2E sidecar** (whoami, task CRUD, skills, workflow, refresh, logout,
composer PTY) can work under `--single` without a prior login module.

## Modules That Failed

The list below is from the March 2026 single-module re-run on one machine.
Each failure is treated
as a **defect** to trace: implementation, test, or **our** setup/CI (images, services, recipes) until
proven otherwise.

### Batch a Stopped at `e2e_0510_task_inference`

- **Symptom:** Inference task finished with `status='failed'` instead of `completed`.
- **Bug class:** Worker/sandbox/inference path or test expectation; **not** the auth sidecar change
  by itself.
    Needs investigation (logs, worker, proxy, model availability).

### Batch B: Fifteen Modules Reported Failed

Failures cluster into symptom buckets (multiple tests per module in some cases).
Each bucket lists
**what to fix**, not an external force.

#### Chat and Models (`e2e_0530`, `e2e_0540`, `e2e_0550`, `e2e_0560`)

- `cynork chat` subprocess **timeouts** (150--300s).
- Concurrent HTTP chat tests: **non-2xx** responses.
- **Fix direction:** PMA, gateway, and Ollama integration must meet latency and correctness targets
  under the documented dev setup; missing `CONFIG_PATH` was ruled out for this workstream.

#### Capable Model (`e2e_0580`)

- Chat timeouts; direct `curl` to `/v1/chat/completions` with **empty body** on failure paths.
- **Fix direction:** Same as chat bucket, plus ensure **OLLAMA_CAPABLE_MODEL** is pulled and
  reachable per the setup we own (`just` / full-demo / documented prereqs).

#### SSE and Streaming (`e2e_0610`, `e2e_0620`, `e2e_0630`, `e2e_0640`)

- **`502 Bad Gateway`** / `PMA proxy stream returned 502 Bad Gateway` in streamed payloads.
- **`requests.exceptions.ConnectionError`** / remote end closed connection (long streams).
- Assertions on **missing `cynodeai.iteration_start`** or empty accumulated content when errors
  appear mid-stream.
- **Fix direction:** Gateway and PMA streaming behavior and stability; connection handling under load.
  Auth wiring was already validated elsewhere.

#### TUI Streaming (`e2e_0650`)

- Some tests **ok** or **skipped** when the stream shape does not include optional events.
- **Failure:** `iteration_start` not seen when the contract was not met.
- **Fix direction:** Same streaming/PMA stack as HTTP SSE tests; PTY harness is not the primary
  suspect unless HTTP tests are green and TUI alone fails.

#### SBA (`e2e_0710`, `e2e_0730`, `e2e_0740`)

- **`e2e_0710`:** SBA task did not complete successfully.
- **`e2e_0730` / `e2e_0740`:** Worker stderr showed Podman could not pull
  `cynodeai-inference-proxy:dev` (manifest access denied on the default registry path).
- **Fix direction:** **Our** image build and load path (`just build-e2e-images`, `INFERENCE_PROXY_IMAGE`,
  local tag availability).
    That is a pipeline or setup bug, not "the user's machine."

#### TUI PTY and Slash Commands (`e2e_0750`, `e2e_0760`)

- Mixed **ok** and **fail** (e.g. in-flight landmark, send/receive round-trip).
- **Fix direction:** Product or test timing against real streaming; if assistant path is broken,
  fix the path; if the test is racy, fix the test.

Batch B ended with **15** failing modules (counting each module once).

## Skips and Whether They Are Acceptable

Conditional skips document **optional** behavior.
They are **bugs** if they mask a required spec on
the canonical stack we claim to support.

### Module `e2e_0720` SBA Result Contract

- **Behavior:** Ran **OK** with **one test skipped:** message included
  `SBA_TASK_ID not set (ensure_e2e_sba_task failed or inference unavailable)`.
- **Acceptable?**
  Only as a **temporary** gap.
  For the stack we say is supported, either an earlier
  test must set `SBA_TASK_ID` or `ensure_e2e_sba_task` must succeed.
    Otherwise the contract test
  **did not run**; that is incomplete coverage, not success.

### Module `e2e_0650` TUI Streaming Behavior

- **Behavior:** Some tests **skipped** when streams did not contain amendment-related events.
- **Acceptable?**
  Only when the **spec** allows absence of those events.
  If the spec requires them
  on the reference stack, skipping is a **bug** (stack or test), not a neutral outcome.

### Optional Inference Skips

- Tests that honor `E2E_SKIP_INFERENCE_SMOKE` are **by design** when that flag is set.
- This run's dominant issues were **failures**, not skips: those need fixes per the buckets above.

## Interpretation

- **Auth / sidecar / `prepare_e2e_cynork_auth()`:** Passing modules show login-related preconditions
  can be met for CLI, workflow, PMA project chat, TUI composer, refresh, and logout under `--single`.
- **Everything else red:** Treat as **bugs**: gateway/PMA/Ollama behavior, streaming and proxy
  correctness, worker and sandbox paths, image availability from **our** build recipes, and tests
  that are wrong or too brittle.
- **Hardware:** The only carve-out is resource limits; even then, tests should **fail clearly** or
  skip with an explicit resource check, not hang without ownership.

This file lives under `docs/dev_docs/` as a **working note**; remove or fold into a permanent runbook
before merging to the default branch if policy requires.
