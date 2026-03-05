# Task Result Empty Stdout for SBA + Inference Prompt (Analysis)

## Metadata

- Date: 2026-03-04
- Status: analysis / dev doc (no code changes)
- Context: User ran `cynork` task with `--use-inference --use-sba -p "Reply back with the current time."` and saw `status=completed` but empty `stdout` and only `sba_result.steps[0].output = "sba-run\n"` instead of the expected time reply.

## Observed Behavior

- Task completes successfully.
- Job result: `stdout=""`, `stderr=""`, `sba_result.status="success"` with one step: `type=run_command`, `output="sba-run\n"`.
- No user-facing "reply" (e.g. current time) appears in task result stdout or in a dedicated reply field.

## Root Causes

Three layers explain why the task result shows empty stdout and only `sba-run` in SBA step output.

### 1. Orchestrator: SBA Job Spec Has Hardcoded Steps, Not Inference-Driven Work

For `createTaskSBA` (prompt + `--use-sba`), the orchestrator builds the job via `buildSBAJobPayload` in `orchestrator/internal/handlers/tasks.go`:

- The **prompt** ("Reply back with the current time.") is correctly placed in **`context.task_context`**.
- The **steps** are **hardcoded** to a single step: `run_command` with `argv: ["echo", "sba-run"]`.

So the job spec does not ask the SBA to "use inference to answer the prompt."
It asks the SBA to run a fixed command `echo sba-run`.
The prompt is available in context but is not used to drive agent behavior because the steps are pre-filled and executed as-is.

Spec reference: `docs/tech_specs/cynode_sba.md` describes the SBA as an agent that uses inference to plan and execute.
Context (requirements, acceptance criteria, task context) should drive the agent's todo list and actions.
For a pure "reply with the current time" task, the job would typically have **no pre-filled steps** (or only suggested steps) and requirements/context that state the goal, so the SBA can use the LLM to decide to run a command (e.g. `date`) and/or produce a text reply.

### 2. Worker: SBA Always Runs With `SBA_DIRECT_STEPS=1`

In `worker_node/cmd/worker-api/executor/executor.go`, when running the SBA container, the node **always** injects `SBA_DIRECT_STEPS=1` (see `buildSBARunArgs`).
The comment states: "Direct step execution (no LLM) so jobs with only run_command/write_file/etc. succeed without Ollama in container."

Effect:

- The SBA runner does **not** enter an LLM loop; it only **executes the steps** in the job spec.
- So the only thing that runs is the single step `echo sba-run`, producing step output `"sba-run\n"`.
- The prompt in `context.task_context` is never sent to a model; no "reply with the current time" is generated.

For "prompt + inference + SBA" to produce an LLM-generated reply, the SBA must run **without** direct-steps mode (or the orchestrator must send a different job shape that is executed in direct mode but already contains a step that produces the reply, e.g. a step that calls inference and writes the answer).

### 3. Task Result Stdout is Container Stdout, Not Derived From SBA Result

- The Worker API captures **container process stdout/stderr** and returns them in `RunJobResponse.stdout` and `RunJobResponse.stderr` (see `worker_api.md` and executor code).
- The SBA process, when running in direct-steps mode, writes the structured result to `/job/result.json`; it does **not** necessarily print the user-facing "reply" to process stdout.
- The orchestrator's task result aggregates job **stdout** from the stored job result (`aggregateLogsFromJobs` in `orchestrator/internal/handlers/tasks.go`).
  It does **not** derive a "reply" from `sba_result` (e.g. from a final step output or a dedicated result field).

So even if the SBA had produced a "current time" string in `sba_result.steps[].output` or in a hypothetical `sba_result.final_reply` field, that would not currently be merged into the task-level stdout the CLI displays.
The SBA result contract in `cynode_sba.md` defines `steps`, `artifacts`, `failure_code`, `failure_message`; there is no specified "final reply" or "answer" field that the orchestrator or CLI maps to task result stdout.

## Summary (By Layer)

- **Layer:** Orchestrator
  - current behavior: Builds SBA job with prompt in `context.task_context` but steps = `[run_command echo sba-run]`.
  - needed for "reply with current time": Build job so the SBA is asked to fulfill the prompt (e.g. empty or suggested steps + context as requirement), or add a step that calls inference and writes the reply.
- **Layer:** Worker
  - current behavior: Always sets `SBA_DIRECT_STEPS=1`; no LLM loop.
  - needed for "reply with current time": For inference-driven SBA tasks, either do not set direct-steps (so SBA uses LLM) or accept that direct-steps only runs pre-defined steps.
- **Layer:** Result surface
  - current behavior: Task result stdout = aggregated job `RunJobResponse.stdout` (container capture).
    SBA result is in `job.result.sba_result` but not merged into stdout.
  - needed for "reply with current time": Either have the SBA write the reply to process stdout, or define a convention (e.g. `sba_result.final_reply` or last step output) and have orchestrator/CLI merge it into task result stdout.

## References

- `docs/tech_specs/cynode_sba.md` - SBA execution model, context, result contract.
- `docs/tech_specs/worker_api.md` - Run Job response fields, node-mediated SBA result.
- `docs/tech_specs/cli_management_app_commands_tasks.md` - Task result output (stdout/stderr when terminal).
- `orchestrator/internal/handlers/tasks.go` - `buildSBAJobPayload`, `createTaskSBA`, `aggregateLogsFromJobs`.
- `worker_node/cmd/worker-api/executor/executor.go` - `runJobSBA`, `buildSBARunArgs`, `SBA_DIRECT_STEPS=1`, stdout capture.

## Suggested Next Steps (For Implementation, Not in This Doc)

1. **Orchestrator:** For prompt+use_sba+use_inference, build a job spec that expresses the prompt as the task/requirement and either leaves steps empty (so the SBA agent loop can plan) or includes inference + reply steps as appropriate for the runner mode.
2. **Worker:** Make direct-steps mode conditional (e.g. from job spec or request) when inference is required, so the SBA can run the LLM loop for "answer this prompt" tasks.
3. **Result:** Define where the "reply" lives (e.g. SBA stdout, or a field in the SBA result contract) and ensure the task result API and CLI surface it as the primary user-facing output for prompt-style SBA tasks.
