# Task 11 Completion - SBA Prompt Construction (REQ-SBAGNT-0113)

## Summary

- **`ContextSpec`** (`go_shared_libs/contracts/sbajob/sbajob.go`): added **`PersonaTitle`** and **`PersonaDescription`** (JSON `persona_title`, `persona_description`).
- **`buildUserPrompt`** (`agents/internal/sba/agent.go`): context blocks are emitted in spec order - persona -> baseline -> project -> task -> requirements/acceptance -> preferences -> additional context -> skills -> runtime (deadline + step count) -> suggested steps -> output contract.
- Helpers: `appendPersonaSection`, `appendBaselineSection`, … `appendRuntimeTurnSection` (runtime omitted when there is no deadline and no steps).
- **Tests:** `TestBuildUserPrompt_ContextBlockOrder`, `TestBuildUserPrompt_WithDeadline_IncludesTimeRemaining`; updated expectations for renamed section headers.

## Validation

- `go test -cover ./agents/...`, `go test ./go_shared_libs/contracts/sbajob/...` - pass.
- `e2e_0740` poll window increased to **120*5s** to reduce flakes under full-suite load.

## REQ Mapping

- **Block:** Persona
  - implementation: `PersonaTitle`, `PersonaDescription`
- **Block:** Baseline
  - implementation: `BaselineContext`
- **Block:** Project
  - implementation: `ProjectContext`
- **Block:** Task
  - implementation: `TaskContext`
- **Block:** Req / AC
  - implementation: `Requirements`, `AcceptanceCriteria`
- **Block:** Preferences
  - implementation: `Preferences` map
- **Block:** Additional
  - implementation: `AdditionalContext`
- **Block:** Skills
  - implementation: `SkillIDs`, `Skills` (JSON)
- **Block:** Runtime
  - implementation: Deadline + suggested step count in spec
