# Tasks 2 and 3 Completion: Greedy PMA Provisioning and Routing

<!-- Date: 2026-03-31 (UTC) -->

## Task 2 (REQ-ORCHES-0190)

- **Trigger:** `GreedyProvisionPMAAfterInteractiveSession` runs after `CreateRefreshSession` on **login** and **refresh** (`auth.go`), using the refresh session row ID as the interactive session anchor in `SessionBindingLineage`.
- **Persistence:** Upserts `session_bindings` with `service_id` from `models.PMAServiceIDForBindingKey`, records MCP intent with invocation class `user_gateway_session` (`pma_greedy.go`, `GreedyPMAIssue` for tests).
- **Node push:** Bumps `config_version` on the PMA host node (ULID) so workers refetch desired state.
- **Desired state:** `buildManagedServicesDesiredState` appends one PMA managed-service entry per **active** row from `ListAllActiveSessionBindings` (distinct `service_id`), in addition to the bootstrap `PMA_SERVICE_ID` entry.
- **Tests:** `TestGreedyProvision_*` in `pma_greedy_test.go`.

## Task 3 (REQ-ORCHES-0162, REQ-ORCHES-0151)

- **Routing:** `resolvePMAEndpointCandidate(ctx, userID)` filters worker-reported PMA candidates by `service_id` matching the user's active session binding (`ListActiveBindingsForUser`).
- **Latest binding:** When several bindings exist, the implementation picks the latest by `UpdatedAt`.
- **Nil user:** `uuid.Nil` user keeps legacy "most recent ready" behavior for internal/tests without a binding.
- **Candidates:** `pmaEndpointCandidate` carries `service_id` from capability `managed_services_status`.
- **Fail closed:** If bindings exist but no ready service matches `service_id`, endpoint is empty (no fallback to a different instance).
- **Tests:** `TestPmaRouting_*` in `openai_chat_pma_routing_test.go`; `OpenAIChatHandlerDeps` includes `SessionBindingStore`.

## Validation

- `go test ./orchestrator/...` and `just lint-go` passed in this session.

## Plan Reference

`docs/dev_docs/_plan_005_pma_provisioning.md` (Tasks 2 and 3).
