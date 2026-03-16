# Failed E2E Report: e2e_140_sba_task_inference.test_sba_task_with_inference_prompt

## 1 Summary

Test `e2e_140_sba_task_inference.TestSbaInference.test_sba_task_with_inference_prompt` failed because the SBA task did not reach a terminal status within the polling window; the test reported "status=None result=None".
The test creates an SBA task with an LLM prompt, then calls `helpers.create_and_poll_sba_task()`; it asserted task_id at line 39 (so create may have returned a task_id), then failed at the "did not finish" assertion (lines 38-41) because status was not in ("completed", "failed").

## 2 Why the Failure Occurred

- **Observed:** `AssertionError: SBA inference task did not finish: status=None result=None`
- **Root cause:** `create_and_poll_sba_task` returns (task_id, status, result_data); after polling, status and result_data were None.
  So either (1) task create returned a task_id but the subsequent poll loop never got a valid task result response, or (2) the task result API returned responses that did not parse to a status/result, or (3) the polling loop exited without a terminal status (e.g. timeout or empty responses).
- **Effect:** The test failed at the assertion that status is in ("completed", "failed").

## 3 Specific Code Paths Involved

Relevant code paths:

### 3.1 Python Test Path

- [e2e_140_sba_task_inference.py](../../../scripts/test_scripts/e2e_140_sba_task_inference.py) lines 25-41: Calls `create_and_poll_sba_task` with SBA + prompt args; asserts task_id not None, then asserts status in ("completed", "failed"); failure at "did not finish" with status=None, result=None.
- [helpers.py](../../../scripts/test_scripts/helpers.py): `create_and_poll_sba_task` creates task, parses task_id, polls task result until terminal status or max iterations; returns (task_id, status, result_data).

### 3.2 Backend Path

- SBA task with inference prompt; orchestrator, SBA agent, and inference path.
  Task result must be returned and include a terminal status; if polling times out or result structure is missing status, the helper returns None.

## 4 Related Specs, Requirements, and Feature Files

Traceability for this test:

### 4.1 Requirements References

- [REQ-SBAGNT-0106](../../requirements/sbagnt.md), [REQ-SBAGNT-0109](../../requirements/sbagnt.md): SBA result contract and inference path.

### 4.2 Tech Specs

- [cynode_sba.md](../../tech_specs/cynode_sba.md): SBA task with inference and result contract.

### 4.3 Feature Files

- SBA inference E2E/feature coverage.

## 5 Implementation Deviation

- **Spec/requirement intent:** SBA task with inference prompt MUST reach a terminal status (completed or failed) and expose result/sba_result so the test can assert.
- **Observed behavior:** After create and polling, status and result were None; the task did not "finish" from the test's perspective.
- **Deviation:** Either (1) task result API or polling logic does not return/parse status and result correctly, (2) the task never reaches a terminal status within the poll window, or (3) create_and_poll_sba_task exits early (e.g. create timeout) and returns None for status/result even when task_id was set.

## 6 What Needs to Be Fixed in the Implementation

The following describes the two failure modes and required changes.

### 6.1 Root Cause (Cascade vs. Result Shape)

- **Create timeout (primary):** If status/result are None because the test never gets a task_id or create times out, the same blocking create path as e2e_050 applies.
  Fix task create per [2026-03-16_e2e_050_test_task_create.md](2026-03-16_e2e_050_test_task_create.md) section 6.
- **Poll/result shape:** If task_id is set but status or result stay None: (1) Ensure GET /v1/tasks/{id} and GET /v1/tasks/{id}/result return status and result in the structure the test expects (e.g. `result.job_result.sba_result` or equivalent per [worker_node.md](../../tech_specs/worker_node.md)). (2) Ensure the SBA task reaches a terminal status within the test's poll window; if the task runs inference and blocks server-side, it may never complete in time.

### 6.2 Exact Code or Config Changes

- Fix create blocking first; then if the test still fails, verify task result API response shape and polling interval/max wait in the test match the implementation.
