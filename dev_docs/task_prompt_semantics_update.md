# Task Prompt Semantics: Intended Behavior and Doc Updates

- [Summary](#summary)
- [Current vs Intended Behavior](#current-vs-intended-behavior)
- [Document Changes Made](#document-changes-made)
- [References](#references)

## Summary

**Date:** 2026-02-20

The MVP plans, tech specs, and requirements have been updated to reflect the **intended** behavior for task creation.
A natural-language prompt is interpreted by the system, which then decides whether to call the AI model and/or run tasks in the sandbox.
The prompt is not the literal shell command executed in the sandbox.

## Current vs Intended Behavior

| Aspect | Current (Phase 1 implementation) | Intended |
|--------|-----------------------------------|----------|
| User input | `--prompt "..."` or request body `prompt` | Same: natural-language prompt |
| Interpretation | None; prompt is passed through as the sandbox command (e.g. `sh -c "<prompt>"`) | System interprets prompt and decides: call model and/or run sandbox job(s) |
| Result | Natural-language text like "Tell me what model you are" is executed as a shell command and fails (e.g. `Tell: not found`) | System may call the model with the prompt and/or generate and run appropriate sandbox commands |

## Document Changes Made

1. **dev_docs/post_phase1_mvp_plan.md** - Added intended task semantics and current implementation gap under scope and status.
2. **docs/tech_specs/_main.md** - Phase 1 / workflow: clarified that user-facing task creation accepts a natural-language prompt and the system is responsible for interpretation (model and/or sandbox); Phase 1 implementation may pass prompt as command until that layer exists.
3. **docs/tech_specs/cli_management_app.md** - Documented task create semantics: natural-language prompt is the default, system interprets (inference by default); prompt must not be executed as literal shell command unless user requests raw execution (script, commands, or raw-command flag).
4. **docs/requirements/orches.md** - Added REQ-ORCHES-0125: task creation accepts natural-language prompt; system must interpret (model and/or sandbox); prompt must not be literal shell command unless user explicitly requests raw command.
5. **docs/tech_specs/user_api_gateway.md** - Task submission: create tasks with natural-language prompt; system interprets and may call model and/or dispatch sandbox work.
6. **features/cynork/cynork_cli.feature** - Clarified that the "Create task and get result" scenario uses a literal command for MVP testing; intended semantics are natural-language prompt with system interpretation.

## References

- REQ-ORCHES-0121 (create tasks via User API Gateway)
- REQ-ORCHES-0125 (task prompt interpretation, new)
- `docs/tech_specs/user_api_gateway.md` - Task submission and management
- `docs/tech_specs/cli_management_app.md` - Task create command
- `docs/tech_specs/worker_api.md` - Worker API still receives a **command** from the orchestrator; interpretation happens above that layer.
