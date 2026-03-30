---
name: Per-Session-Binding PMA Provisioning
overview: |
  Implement per-session-binding PMA provisioning across the orchestrator,
  worker node, and client layers.
  The orchestrator must provision one cynode-pma managed service instance per
  session binding, greedily on interactive login, route cynodeai.pm chat to the
  correct instance, and tear down stale instances.
  The worker node must reconcile multiple concurrent PMA instances.
  Clients must carry session context for binding attribution.
  BDD and E2E tests validate the full lifecycle.
todos:
  - id: pma-001
    content: "Read REQ-ORCHES-0188, REQ-ORCHES-0190, REQ-ORCHES-0191, REQ-ORCHES-0162, REQ-ORCHES-0151 in `docs/requirements/orches.md` for the normative session-binding, provisioning, routing, and teardown requirements."
    status: pending
  - id: pma-002
    content: "Read REQ-WORKER-0176 and REQ-WORKER-0175 in `docs/requirements/worker.md` for multi-instance PMA and proxy requirements."
    status: pending
    dependencies:
      - pma-001
  - id: pma-003
    content: "Read `docs/tech_specs/orchestrator_bootstrap.md` section PMA Instance per Session Binding (~lines 229-242) and `docs/tech_specs/orchestrator.md` PmaGreedyProvisioningOnLogin (~lines 361-368) for implementation guidance."
    status: pending
    dependencies:
      - pma-002
  - id: pma-004
    content: "Read `orchestrator/internal/models/models.go` to identify where session binding and service_id tracking must be added or extended."
    status: pending
    dependencies:
      - pma-003
  - id: pma-005
    content: "Read `orchestrator/internal/handlers/` for existing managed-service and session handling code that must be extended."
    status: pending
    dependencies:
      - pma-004
  - id: pma-006
    content: "Add a `SessionBinding` model or extend existing session model with: user ID, session/thread lineage, bound `service_id`, binding state (active, teardown-pending)."
    status: pending
    dependencies:
      - pma-005
  - id: pma-007
    content: "Add unit tests: creating a session binding must produce a unique binding key; two bindings for the same user+session must resolve to the same key; different users or sessions must produce different keys."
    status: pending
    dependencies:
      - pma-006
  - id: pma-008
    content: "Run `go test -v -run TestSessionBinding ./orchestrator/internal/models/...` and confirm failures."
    status: pending
    dependencies:
      - pma-007
  - id: pma-009
    content: "Implement the session binding key derivation: stable key from user ID + session/thread lineage per the tech spec."
    status: pending
    dependencies:
      - pma-008
  - id: pma-010
    content: "Add store methods: `UpsertSessionBinding`, `GetSessionBindingByKey`, `ListActiveBindingsForUser`."
    status: pending
    dependencies:
      - pma-009
  - id: pma-011
    content: "Re-run `go test -v -run TestSessionBinding ./orchestrator/internal/models/...` and confirm green."
    status: pending
    dependencies:
      - pma-010
  - id: pma-012
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pma-011
  - id: pma-013
    content: "Validation gate -- do not proceed to Task 2 until all checks pass."
    status: pending
    dependencies:
      - pma-012
  - id: pma-014
    content: "Generate task completion report for Task 1. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pma-013
  - id: pma-015
    content: "Do not start Task 2 until Task 1 closeout is done."
    status: pending
    dependencies:
      - pma-014
  - id: pma-016
    content: "Read `orchestrator/internal/handlers/` for the auth/login flow and identify where greedy provisioning should be triggered (after successful auth, before first chat)."
    status: pending
    dependencies:
      - pma-015
  - id: pma-017
    content: "Read `docs/tech_specs/orchestrator.md` PmaGreedyProvisioningOnLogin for the exact trigger: authenticate via User API Gateway and obtain interactive session."
    status: pending
    dependencies:
      - pma-016
  - id: pma-018
    content: "Add unit tests: on successful auth that establishes an interactive session, orchestrator must ensure desired managed-service state includes a PMA instance for that binding before returning the auth response."
    status: pending
    dependencies:
      - pma-017
  - id: pma-019
    content: "Add unit tests: greedy provisioning must push node configuration and issue PMA MCP credentials with invocation class `user_gateway_session`."
    status: pending
    dependencies:
      - pma-018
  - id: pma-020
    content: "Add unit tests: provisioning must NOT be deferred until first `model=cynodeai.pm` chat message."
    status: pending
    dependencies:
      - pma-019
  - id: pma-021
    content: "Run `go test -v -run 'TestGreedyProvision' ./orchestrator/internal/handlers/...` and confirm failures."
    status: pending
    dependencies:
      - pma-020
  - id: pma-022
    content: "Implement greedy provisioning: after auth success and interactive session establishment, resolve or create session binding, ensure PMA in desired managed-service state, push node config, issue MCP credentials."
    status: pending
    dependencies:
      - pma-021
  - id: pma-023
    content: "Re-run `go test -v -run 'TestGreedyProvision' ./orchestrator/internal/handlers/...` and confirm green."
    status: pending
    dependencies:
      - pma-022
  - id: pma-024
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pma-023
  - id: pma-025
    content: "Validation gate -- do not proceed to Task 3 until all checks pass."
    status: pending
    dependencies:
      - pma-024
  - id: pma-026
    content: "Generate task completion report for Task 2. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pma-025
  - id: pma-027
    content: "Do not start Task 3 until Task 2 closeout is done."
    status: pending
    dependencies:
      - pma-026
  - id: pma-028
    content: "Read `orchestrator/internal/handlers/` chat routing code to identify where `model=cynodeai.pm` is resolved to a backend endpoint."
    status: pending
    dependencies:
      - pma-027
  - id: pma-029
    content: "Add unit tests: chat request with `model=cynodeai.pm` must route to the worker-mediated endpoint for the PMA instance tied to the active session binding."
    status: pending
    dependencies:
      - pma-028
  - id: pma-030
    content: "Add unit tests: routing must track `service_id` + binding in control-plane state (REQ-ORCHES-0151)."
    status: pending
    dependencies:
      - pma-029
  - id: pma-031
    content: "Add unit tests: chat request when no PMA instance is provisioned for the binding must return a clear error (not route to a wrong instance)."
    status: pending
    dependencies:
      - pma-030
  - id: pma-032
    content: "Run `go test -v -run 'TestPmaRouting' ./orchestrator/internal/handlers/...` and confirm failures."
    status: pending
    dependencies:
      - pma-031
  - id: pma-033
    content: "Implement routing: resolve `model=cynodeai.pm` to the worker-mediated endpoint for the PMA instance matching the request's session binding; look up `service_id` via the binding key."
    status: pending
    dependencies:
      - pma-032
  - id: pma-034
    content: "Re-run `go test -v -run 'TestPmaRouting' ./orchestrator/internal/handlers/...` and confirm green."
    status: pending
    dependencies:
      - pma-033
  - id: pma-035
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pma-034
  - id: pma-036
    content: "Validation gate -- do not proceed to Task 4 until all checks pass."
    status: pending
    dependencies:
      - pma-035
  - id: pma-037
    content: "Generate task completion report for Task 3. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pma-036
  - id: pma-038
    content: "Do not start Task 4 until Task 3 closeout is done."
    status: pending
    dependencies:
      - pma-037
  - id: pma-039
    content: "Read REQ-ORCHES-0191 for teardown triggers: session end, logout, idle beyond policy, credential expiry."
    status: pending
    dependencies:
      - pma-038
  - id: pma-040
    content: "Add unit tests: on logout, orchestrator must update desired state to stop the PMA instance for that binding and invalidate PMA MCP credentials."
    status: pending
    dependencies:
      - pma-039
  - id: pma-041
    content: "Add unit tests: on idle timeout beyond policy, orchestrator must teardown the PMA instance."
    status: pending
    dependencies:
      - pma-040
  - id: pma-042
    content: "Add unit tests: on credential expiry, orchestrator must teardown and not leave idle containers."
    status: pending
    dependencies:
      - pma-041
  - id: pma-043
    content: "Run `go test -v -run 'TestPmaTeardown' ./orchestrator/internal/handlers/...` and confirm failures."
    status: pending
    dependencies:
      - pma-042
  - id: pma-044
    content: "Implement teardown: on session end, logout, idle timeout, or credential expiry, update desired state to remove the PMA entry for that binding, invalidate MCP credentials, and push updated config to the worker node."
    status: pending
    dependencies:
      - pma-043
  - id: pma-045
    content: "Implement idle-timeout scanner: background goroutine that periodically checks active bindings against idle policy and triggers teardown for stale ones."
    status: pending
    dependencies:
      - pma-044
  - id: pma-046
    content: "Re-run `go test -v -run 'TestPmaTeardown' ./orchestrator/internal/handlers/...` and confirm green."
    status: pending
    dependencies:
      - pma-045
  - id: pma-047
    content: "Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pma-046
  - id: pma-048
    content: "Validation gate -- do not proceed to Task 5 until all checks pass."
    status: pending
    dependencies:
      - pma-047
  - id: pma-049
    content: "Generate task completion report for Task 4. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pma-048
  - id: pma-050
    content: "Do not start Task 5 until Task 4 closeout is done."
    status: pending
    dependencies:
      - pma-049
  - id: pma-051
    content: "Read `worker_node/cmd/node-manager/main.go` managed service reconciliation loop to understand how `managed_services.services[]` entries are processed."
    status: pending
    dependencies:
      - pma-050
  - id: pma-052
    content: "Read REQ-WORKER-0176 for multiple concurrent PMA instances and REQ-WORKER-0175 for independent health, restart, and proxy UDS per instance."
    status: pending
    dependencies:
      - pma-051
  - id: pma-053
    content: "Add unit tests: reconciliation must handle multiple PMA entries in `managed_services.services[]` with distinct `service_id` values, each getting independent health checks and proxy UDS."
    status: pending
    dependencies:
      - pma-052
  - id: pma-054
    content: "Add unit tests: proxy must fail closed if a binding's token cannot be resolved."
    status: pending
    dependencies:
      - pma-053
  - id: pma-055
    content: "Run `go test -v -run 'TestMultiPMA' ./worker_node/cmd/node-manager/...` and confirm failures."
    status: pending
    dependencies:
      - pma-054
  - id: pma-056
    content: "Extend the managed service reconciliation loop to handle multiple PMA entries: each gets its own container, UDS path, health probe, and restart policy."
    status: pending
    dependencies:
      - pma-055
  - id: pma-057
    content: "Extend the proxy to resolve tokens per `service_id`; fail closed (reject) if token resolution fails for a binding."
    status: pending
    dependencies:
      - pma-056
  - id: pma-058
    content: "Re-run `go test -v -run 'TestMultiPMA' ./worker_node/cmd/node-manager/...` and confirm green."
    status: pending
    dependencies:
      - pma-057
  - id: pma-059
    content: "Run `just lint-go` on changed files and `go test -cover ./worker_node/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pma-058
  - id: pma-060
    content: "Validation gate -- do not proceed to Task 6 until all checks pass."
    status: pending
    dependencies:
      - pma-059
  - id: pma-061
    content: "Generate task completion report for Task 5. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pma-060
  - id: pma-062
    content: "Do not start Task 6 until Task 5 closeout is done."
    status: pending
    dependencies:
      - pma-061
  - id: pma-063
    content: "Read `cynork/internal/gateway/client.go` and `cynork/internal/tui/model.go` to identify how the gateway session (JWT, refresh, thread context) is carried in requests."
    status: pending
    dependencies:
      - pma-062
  - id: pma-064
    content: "Determine whether the orchestrator can infer the session binding from existing auth + thread IDs, or if additional API fields are needed."
    status: pending
    dependencies:
      - pma-063
  - id: pma-065
    content: "If additional API fields are needed: add session-binding context to the gateway client request headers or body; update the orchestrator to extract them."
    status: pending
    dependencies:
      - pma-064
  - id: pma-066
    content: "Add unit tests: gateway requests must carry sufficient context for the orchestrator to attribute them to the correct session binding."
    status: pending
    dependencies:
      - pma-065
  - id: pma-067
    content: "Run `go test -v -run TestSessionContext ./cynork/internal/gateway/...` and confirm green (or failure if changes needed)."
    status: pending
    dependencies:
      - pma-066
  - id: pma-068
    content: "Run `just lint-go` on changed files and `go test -cover ./cynork/...`; confirm 90% threshold."
    status: pending
    dependencies:
      - pma-067
  - id: pma-069
    content: "Validation gate -- do not proceed to Task 7 until all checks pass."
    status: pending
    dependencies:
      - pma-068
  - id: pma-070
    content: "Generate task completion report for Task 6. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pma-069
  - id: pma-071
    content: "Do not start Task 7 until Task 6 closeout is done."
    status: pending
    dependencies:
      - pma-070
  - id: pma-072
    content: "Create BDD feature files in `features/` for per-session-binding PMA: session creates binding, second session gets distinct instance, logout triggers teardown, greedy provisioning does not wait for first chat."
    status: pending
    dependencies:
      - pma-071
  - id: pma-073
    content: "Tag BDD scenarios with `@req_ORCHES_0188`, `@req_ORCHES_0190`, `@req_ORCHES_0191`, `@req_WORKER_0176`."
    status: pending
    dependencies:
      - pma-072
  - id: pma-074
    content: "Add or extend E2E test `scripts/test_scripts/e2e_0830_pma_session_binding.py` with tags `[suite_orchestrator, full_demo, pma_inference]` and prereqs `[gateway, config, auth, node_register]`: second user/session yields distinct PMA instance, logout teardown, greedy before first chat."
    status: pending
    dependencies:
      - pma-073
  - id: pma-075
    content: "Run `just test-bdd` for the new BDD scenarios."
    status: pending
    dependencies:
      - pma-074
  - id: pma-076
    content: "Run `just e2e --tags pma_inference` (or `just e2e --tags no_inference` for contract-level tests) to verify the full lifecycle."
    status: pending
    dependencies:
      - pma-075
  - id: pma-077
    content: "Run `just lint-go` on all changed files across all modules."
    status: pending
    dependencies:
      - pma-076
  - id: pma-078
    content: "Run `just ci` locally and confirm all targets pass."
    status: pending
    dependencies:
      - pma-077
  - id: pma-079
    content: "Validation gate -- do not proceed to Task 8 until all checks pass."
    status: pending
    dependencies:
      - pma-078
  - id: pma-080
    content: "Generate task completion report for Task 7. Mark completed steps `- [x]`."
    status: pending
    dependencies:
      - pma-079
  - id: pma-081
    content: "Do not start Task 8 until Task 7 closeout is done."
    status: pending
    dependencies:
      - pma-080
  - id: pma-082
    content: "Update `docs/dev_docs/_todo.md` to mark section 5 items as complete with links to this plan."
    status: pending
    dependencies:
      - pma-081
  - id: pma-083
    content: "Verify no follow-up work was left undocumented."
    status: pending
    dependencies:
      - pma-082
  - id: pma-084
    content: "Run `just docs-check` on all changed documentation."
    status: pending
    dependencies:
      - pma-083
  - id: pma-085
    content: "Run `just e2e --tags no_inference` as final E2E regression gate."
    status: pending
    dependencies:
      - pma-084
  - id: pma-086
    content: "Generate final plan completion report: tasks completed, overall validation, remaining risks."
    status: pending
    dependencies:
      - pma-085
  - id: pma-087
    content: "Mark all completed steps in the plan with `- [x]`. (Last step.)"
    status: pending
    dependencies:
      - pma-086
---

# Per-Session-Binding PMA Provisioning Plan

## Goal

Implement per-session-binding PMA provisioning so that each interactive user session gets its own `cynode-pma` managed service instance.
This spans the orchestrator (binding model, greedy provisioning, routing, teardown), worker node (multi-instance reconciliation), and client layers (session context propagation).

## References

- Requirements: [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188), [REQ-ORCHES-0190](../requirements/orches.md#req-orches-0190), [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191), [REQ-ORCHES-0162](../requirements/orches.md#req-orches-0162), [REQ-ORCHES-0151](../requirements/orches.md#req-orches-0151), [REQ-WORKER-0176](../requirements/worker.md#req-worker-0176), [REQ-WORKER-0175](../requirements/worker.md#req-worker-0175)
- Tech specs: [`docs/tech_specs/orchestrator_bootstrap.md`](../tech_specs/orchestrator_bootstrap.md) (PmaInstancePerSessionBinding), [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) (PmaGreedyProvisioningOnLogin)
- Review reports: [`2026-03-29_review_report_1_orchestrator.md`](2026-03-29_review_report_1_orchestrator.md), [`2026-03-29_review_report_2_worker_node.md`](2026-03-29_review_report_2_worker_node.md)
- Implementation: `orchestrator/`, `worker_node/`, `cynork/`

## Constraints

- Requirements take precedence over tech specs; tech specs take precedence over current code.
- Follow BDD/TDD: failing tests before implementation.
- All changes must pass `just ci` before the task is considered complete.
- No changes that reduce test coverage below the 90% per-package threshold.
- No modifications to linter rules or suppression comments.
- Each task's validation gate must pass before starting the next task.
- Bootstrap/readiness PMA (REQ-ORCHES-0150) is a separate concern addressed in the longer-term plan; this plan covers per-binding instances only.

## Execution Plan

Tasks follow the data-flow order: binding model first, then provisioning, routing, teardown, worker multi-instance, client context, and finally integration tests.

### Task 1: Orchestrator -- Session Binding Model

Persist or derive a stable session binding key so each interactive session maps to at most one PMA `service_id`.

#### Task 1 Requirements and Specifications

- [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188) -- one managed `cynode-pma` per session binding
- [`docs/tech_specs/orchestrator_bootstrap.md`](../tech_specs/orchestrator_bootstrap.md) -- PmaInstancePerSessionBinding (~lines 229-242)

#### Discovery (Task 1) Steps

- [ ] Read REQ-ORCHES-0188, REQ-ORCHES-0190, REQ-ORCHES-0191, REQ-ORCHES-0162, REQ-ORCHES-0151 in `docs/requirements/orches.md` for the normative session-binding, provisioning, routing, and teardown requirements.
- [ ] Read REQ-WORKER-0176 and REQ-WORKER-0175 in `docs/requirements/worker.md` for multi-instance PMA and proxy requirements.
- [ ] Read `docs/tech_specs/orchestrator_bootstrap.md` section PMA Instance per Session Binding (~lines 229-242) and `docs/tech_specs/orchestrator.md` PmaGreedyProvisioningOnLogin (~lines 361-368) for implementation guidance.
- [ ] Read `orchestrator/internal/models/models.go` to identify where session binding and service_id tracking must be added or extended.
- [ ] Read `orchestrator/internal/handlers/` for existing managed-service and session handling code that must be extended.

#### Red (Task 1)

- [ ] Add a `SessionBinding` model or extend existing session model with: user ID, session/thread lineage, bound `service_id`, binding state (active, teardown-pending).
- [ ] Add unit tests: creating a session binding must produce a unique binding key; two bindings for the same user+session must resolve to the same key; different users or sessions must produce different keys.
- [ ] Run `go test -v -run TestSessionBinding ./orchestrator/internal/models/...` and confirm failures.

#### Green (Task 1)

- [ ] Implement the session binding key derivation: stable key from user ID + session/thread lineage per the tech spec.
- [ ] Add store methods: `UpsertSessionBinding`, `GetSessionBindingByKey`, `ListActiveBindingsForUser`.
- [ ] Re-run `go test -v -run TestSessionBinding ./orchestrator/internal/models/...` and confirm green.

#### Refactor (Task 1)

No additional refactor needed.

#### Testing (Task 1)

- [ ] Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 2 until all checks pass.

#### Closeout (Task 1)

- [ ] Generate task completion report for Task 1.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 2 until Task 1 closeout is done.

---

### Task 2: Orchestrator -- Greedy Provisioning (REQ-ORCHES-0190)

On successful auth and interactive session establishment, provision a PMA instance for that binding before the first chat message.

#### Task 2 Requirements and Specifications

- [REQ-ORCHES-0190](../requirements/orches.md#req-orches-0190) -- greedy provisioning
- [`docs/tech_specs/orchestrator.md`](../tech_specs/orchestrator.md) -- PmaGreedyProvisioningOnLogin (~lines 361-368)

#### Discovery (Task 2) Steps

- [ ] Read `orchestrator/internal/handlers/` for the auth/login flow and identify where greedy provisioning should be triggered (after successful auth, before first chat).
- [ ] Read `docs/tech_specs/orchestrator.md` PmaGreedyProvisioningOnLogin for the exact trigger: authenticate via User API Gateway and obtain interactive session.

#### Red (Task 2)

- [ ] Add unit tests: on successful auth that establishes an interactive session, orchestrator must ensure desired managed-service state includes a PMA instance for that binding before returning the auth response.
- [ ] Add unit tests: greedy provisioning must push node configuration and issue PMA MCP credentials with invocation class `user_gateway_session`.
- [ ] Add unit tests: provisioning must NOT be deferred until first `model=cynodeai.pm` chat message.
- [ ] Run `go test -v -run 'TestGreedyProvision' ./orchestrator/internal/handlers/...` and confirm failures.

#### Green (Task 2)

- [ ] Implement greedy provisioning: after auth success and interactive session establishment, resolve or create session binding, ensure PMA in desired managed-service state, push node config, issue MCP credentials.
- [ ] Re-run `go test -v -run 'TestGreedyProvision' ./orchestrator/internal/handlers/...` and confirm green.

#### Refactor (Task 2)

No additional refactor needed.

#### Testing (Task 2)

- [ ] Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 3 until all checks pass.

#### Closeout (Task 2)

- [ ] Generate task completion report for Task 2.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 3 until Task 2 closeout is done.

---

### Task 3: Orchestrator -- Routing (REQ-ORCHES-0162)

Resolve `model=cynodeai.pm` chat to the worker-mediated endpoint for the PMA instance tied to the active session binding.

#### Task 3 Requirements and Specifications

- [REQ-ORCHES-0162](../requirements/orches.md#req-orches-0162) -- route `cynodeai.pm` to the correct PMA instance
- [REQ-ORCHES-0151](../requirements/orches.md#req-orches-0151) -- track `service_id` + binding in control-plane state

#### Discovery (Task 3) Steps

- [ ] Read `orchestrator/internal/handlers/` chat routing code to identify where `model=cynodeai.pm` is resolved to a backend endpoint.

#### Red (Task 3)

- [ ] Add unit tests: chat request with `model=cynodeai.pm` must route to the worker-mediated endpoint for the PMA instance tied to the active session binding.
- [ ] Add unit tests: routing must track `service_id` + binding in control-plane state (REQ-ORCHES-0151).
- [ ] Add unit tests: chat request when no PMA instance is provisioned for the binding must return a clear error (not route to a wrong instance).
- [ ] Run `go test -v -run 'TestPmaRouting' ./orchestrator/internal/handlers/...` and confirm failures.

#### Green (Task 3)

- [ ] Implement routing: resolve `model=cynodeai.pm` to the worker-mediated endpoint for the PMA instance matching the request's session binding; look up `service_id` via the binding key.
- [ ] Re-run `go test -v -run 'TestPmaRouting' ./orchestrator/internal/handlers/...` and confirm green.

#### Refactor (Task 3)

No additional refactor needed.

#### Testing (Task 3)

- [ ] Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 4 until all checks pass.

#### Closeout (Task 3)

- [ ] Generate task completion report for Task 3.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 4 until Task 3 closeout is done.

---

### Task 4: Orchestrator -- Teardown (REQ-ORCHES-0191)

On session end, logout, idle beyond policy, or credential expiry: stop the PMA instance, invalidate MCP credentials, avoid unbounded idle containers.

#### Task 4 Requirements and Specifications

- [REQ-ORCHES-0191](../requirements/orches.md#req-orches-0191) -- stale PMA teardown
- [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188) -- one PMA per session binding (teardown frees the binding)

#### Discovery (Task 4) Steps

- [ ] Read REQ-ORCHES-0191 for teardown triggers: session end, logout, idle beyond policy, credential expiry.

#### Red (Task 4)

- [ ] Add unit tests: on logout, orchestrator must update desired state to stop the PMA instance for that binding and invalidate PMA MCP credentials.
- [ ] Add unit tests: on idle timeout beyond policy, orchestrator must teardown the PMA instance.
- [ ] Add unit tests: on credential expiry, orchestrator must teardown and not leave idle containers.
- [ ] Run `go test -v -run 'TestPmaTeardown' ./orchestrator/internal/handlers/...` and confirm failures.

#### Green (Task 4)

- [ ] Implement teardown: on session end, logout, idle timeout, or credential expiry, update desired state to remove the PMA entry for that binding, invalidate MCP credentials, and push updated config to the worker node.
- [ ] Implement idle-timeout scanner: background goroutine that periodically checks active bindings against idle policy and triggers teardown for stale ones.
- [ ] Re-run `go test -v -run 'TestPmaTeardown' ./orchestrator/internal/handlers/...` and confirm green.

#### Refactor (Task 4)

No additional refactor needed.

#### Testing (Task 4)

- [ ] Run `just lint-go` on changed files and `go test -cover ./orchestrator/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 5 until all checks pass.

#### Closeout (Task 4)

- [ ] Generate task completion report for Task 4.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 5 until Task 4 closeout is done.

---

### Task 5: Worker Node -- Multi-Instance PMA Reconciliation

The worker node must reconcile multiple `managed_services.services[]` PMA entries with distinct `service_id` values, providing independent health, restart, and proxy UDS per instance.

#### Task 5 Requirements and Specifications

- [REQ-WORKER-0176](../requirements/worker.md#req-worker-0176) -- multiple concurrent PMA instances
- [REQ-WORKER-0175](../requirements/worker.md#req-worker-0175) -- independent health, restart, proxy UDS

#### Discovery (Task 5) Steps

- [ ] Read `worker_node/cmd/node-manager/main.go` managed service reconciliation loop to understand how `managed_services.services[]` entries are processed.
- [ ] Read REQ-WORKER-0176 for multiple concurrent PMA instances and REQ-WORKER-0175 for independent health, restart, and proxy UDS per instance.

#### Red (Task 5)

- [ ] Add unit tests: reconciliation must handle multiple PMA entries in `managed_services.services[]` with distinct `service_id` values, each getting independent health checks and proxy UDS.
- [ ] Add unit tests: proxy must fail closed if a binding's token cannot be resolved.
- [ ] Run `go test -v -run 'TestMultiPMA' ./worker_node/cmd/node-manager/...` and confirm failures.

#### Green (Task 5)

- [ ] Extend the managed service reconciliation loop to handle multiple PMA entries: each gets its own container, UDS path, health probe, and restart policy.
- [ ] Extend the proxy to resolve tokens per `service_id`; fail closed (reject) if token resolution fails for a binding.
- [ ] Re-run `go test -v -run 'TestMultiPMA' ./worker_node/cmd/node-manager/...` and confirm green.

#### Refactor (Task 5)

No additional refactor needed.

#### Testing (Task 5)

- [ ] Run `just lint-go` on changed files and `go test -cover ./worker_node/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 6 until all checks pass.

#### Closeout (Task 5)

- [ ] Generate task completion report for Task 5.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 6 until Task 5 closeout is done.

---

### Task 6: Clients -- Gateway Contract (Session Context)

Ensure the gateway session (JWT, refresh, thread context) carries what the orchestrator needs to attribute requests to the correct session binding.

#### Task 6 Requirements and Specifications

- [`docs/tech_specs/cynork/cynork_cli.md`](../tech_specs/cynork/cynork_cli.md) -- gateway session management
- [REQ-ORCHES-0188](../requirements/orches.md#req-orches-0188) -- session binding attribution

#### Discovery (Task 6) Steps

- [ ] Read `cynork/internal/gateway/client.go` and `cynork/internal/tui/model.go` to identify how the gateway session (JWT, refresh, thread context) is carried in requests.
- [ ] Determine whether the orchestrator can infer the session binding from existing auth + thread IDs, or if additional API fields are needed.

#### Red (Task 6)

- [ ] If additional API fields are needed: add session-binding context to the gateway client request headers or body; update the orchestrator to extract them.
- [ ] Add unit tests: gateway requests must carry sufficient context for the orchestrator to attribute them to the correct session binding.

#### Green (Task 6)

- [ ] Run `go test -v -run TestSessionContext ./cynork/internal/gateway/...` and confirm green (or failure if changes needed).

#### Refactor (Task 6)

No additional refactor needed.

#### Testing (Task 6)

- [ ] Run `just lint-go` on changed files and `go test -cover ./cynork/...`; confirm 90% threshold.
- [ ] Validation gate -- do not proceed to Task 7 until all checks pass.

#### Closeout (Task 6)

- [ ] Generate task completion report for Task 6.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 7 until Task 6 closeout is done.

---

### Task 7: E2E and BDD Tests

Validate the full per-session-binding PMA lifecycle with BDD and E2E tests.

#### Task 7 Requirements and Specifications

- All requirements from Tasks 1-6
- [`docs/tech_specs/orchestrator_bootstrap.md`](../tech_specs/orchestrator_bootstrap.md) -- readiness
- Feature file conventions in `features/`

#### Discovery (Task 7) Steps

No additional discovery; all context gathered in Tasks 1-6.

#### Red (Task 7)

- [ ] Create BDD feature files in `features/` for per-session-binding PMA: session creates binding, second session gets distinct instance, logout triggers teardown, greedy provisioning does not wait for first chat.
- [ ] Tag BDD scenarios with `@req_ORCHES_0188`, `@req_ORCHES_0190`, `@req_ORCHES_0191`, `@req_WORKER_0176`.
- [ ] Add or extend E2E test `scripts/test_scripts/e2e_0830_pma_session_binding.py` with tags `[suite_orchestrator, full_demo, pma_inference]` and prereqs `[gateway, config, auth, node_register]`: second user/session yields distinct PMA instance, logout teardown, greedy before first chat.

#### Green (Task 7)

- [ ] Run `just test-bdd` for the new BDD scenarios.
- [ ] Run `just e2e --tags pma_inference` (or `just e2e --tags no_inference` for contract-level tests) to verify the full lifecycle.

#### Refactor (Task 7)

No additional refactor needed.

#### Testing (Task 7)

- [ ] Run `just lint-go` on all changed files across all modules.
- [ ] Run `just ci` locally and confirm all targets pass.
- [ ] Validation gate -- do not proceed to Task 8 until all checks pass.

#### Closeout (Task 7)

- [ ] Generate task completion report for Task 7.
  Mark completed steps `- [x]`.
- [ ] Do not start Task 8 until Task 7 closeout is done.

---

### Task 8: Documentation and Closeout

- [ ] Update `docs/dev_docs/_todo.md` to mark section 5 items as complete with links to this plan.
- [ ] Verify no follow-up work was left undocumented.
- [ ] Run `just docs-check` on all changed documentation.
- [ ] Run `just e2e --tags no_inference` as final E2E regression gate.
- [ ] Generate final plan completion report: tasks completed, overall validation, remaining risks.
- [ ] Mark all completed steps in the plan with `- [x]`. (Last step.)
