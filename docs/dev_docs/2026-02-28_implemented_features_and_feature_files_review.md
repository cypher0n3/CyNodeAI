# Implemented Features vs Feature Files Review

- [Summary](#summary)
- [Checks Run](#checks-run)
- [Feature Files by Suite](#feature-files-by-suite-19-files-79-scenarios)
- [E2E Test Modules to Feature File Mapping](#e2e-test-modules-to-feature-file-mapping)
- [Change Made](#change-made)
- [References](#references)

## Summary

**Date:** 2026-02-28

- All 19 feature files under `features/` pass **`just validate-feature-files`** (suite tags, narrative block, scenario traceability).
- All feature files pass **`just lint-gherkin`** (gherkin-lint with `.gherkin-lintrc`).
- Implemented E2E coverage (Python `scripts/test_scripts/e2e_*.py`) is mapped to feature files below.
  Two cynork CLI scenarios (auth refresh, logout) were added to `features/cynork/cynork_status_auth.feature` so E2E tests e2e_08_refresh and e2e_09_logout have explicit feature coverage.

## Checks Run

- `just validate-feature-files` - OK
- `just lint-gherkin` - OK (gherkin-lint on `features/`)

## Feature Files by Suite (19 Files, 79 Scenarios)

- **Suite:** orchestrator
  - file: initial_auth.feature
  - scenarios: 4
- **Suite:** orchestrator
  - file: node_registration_and_config.feature
  - scenarios: 7
- **Suite:** orchestrator
  - file: orchestrator_startup.feature
  - scenarios: 1
- **Suite:** orchestrator
  - file: orchestrator_task_lifecycle.feature
  - scenarios: 10
- **Suite:** worker_node
  - file: node_manager_config_startup.feature
  - scenarios: 3
- **Suite:** worker_node
  - file: worker_node_sandbox_execution.feature
  - scenarios: 10
- **Suite:** worker_node
  - file: worker_node_sba.feature
  - scenarios: 5
- **Suite:** agents
  - file: sba_contract.feature
  - scenarios: 5
- **Suite:** agents
  - file: sba_lifecycle.feature
  - scenarios: 1
- **Suite:** agents
  - file: sba_runner.feature
  - scenarios: 3
- **Suite:** agents
  - file: sba_tools.feature
  - scenarios: 1
- **Suite:** agents
  - file: sba_failure_codes.feature
  - scenarios: 1
- **Suite:** cynork
  - file: cynork_status_auth.feature
  - scenarios: 8
- **Suite:** cynork
  - file: cynork_shell.feature
  - scenarios: 1
- **Suite:** cynork
  - file: cynork_tasks.feature
  - scenarios: 10
- **Suite:** cynork
  - file: cynork_chat.feature
  - scenarios: 2
- **Suite:** cynork
  - file: cynork_admin.feature
  - scenarios: 8
- **Suite:** e2e
  - file: single_node_happy_path.feature
  - scenarios: 2
- **Suite:** e2e
  - file: chat_openai_compatible.feature
  - scenarios: 3

## E2E Test Modules to Feature File Mapping

- **E2E module:** e2e_00_status_version
  - coverage: cynork_status_auth (status)
- **E2E module:** e2e_01_login
  - coverage: cynork_status_auth (login), initial_auth (API login)
- **E2E module:** e2e_01b_auth_negative
  - coverage: cynork_status_auth (whoami without token)
- **E2E module:** e2e_02_whoami
  - coverage: cynork_status_auth (whoami)
- **E2E module:** e2e_03_task_create
  - coverage: orchestrator_task_lifecycle, cynork_tasks
- **E2E module:** e2e_03b_task_list
  - coverage: orchestrator_task_lifecycle (list tasks), cynork_tasks
- **E2E module:** e2e_04_task_get
  - coverage: orchestrator_task_lifecycle, cynork_tasks
- **E2E module:** e2e_05_task_result
  - coverage: orchestrator_task_lifecycle, cynork_tasks
- **E2E module:** e2e_05b_inference_task
  - coverage: single_node_happy_path (inference in sandbox), orchestrator_task_lifecycle
- **E2E module:** e2e_05c_prompt_task
  - coverage: orchestrator_task_lifecycle (prompt/LLM task)
- **E2E module:** e2e_05d_models_and_chat
  - coverage: chat_openai_compatible, cynork_chat
- **E2E module:** e2e_05e_sba_task
  - coverage: worker_node_sba, sba_*
- **E2E module:** e2e_05e2_sba_result_contract
  - coverage: sba_contract
- **E2E module:** e2e_05e3_sba_inference
  - coverage: worker_node_sba
- **E2E module:** e2e_05f_task_logs
  - coverage: orchestrator_task_lifecycle, cynork_tasks
- **E2E module:** e2e_05g_task_cancel
  - coverage: orchestrator_task_lifecycle, cynork_tasks
- **E2E module:** e2e_06_node_register
  - coverage: node_registration_and_config
- **E2E module:** e2e_07_capability
  - coverage: node_registration_and_config
- **E2E module:** e2e_08_refresh
  - coverage: cynork_status_auth (auth refresh) - scenario added
- **E2E module:** e2e_09_logout
  - coverage: cynork_status_auth (logout) - scenario added

## Change Made

- **features/cynork/cynork_status_auth.feature**: Added two scenarios so CLI auth behavior exercised by e2e_08_refresh and e2e_09_logout is specified in Gherkin:
  - "Auth refresh renews session" (`@req_identy_0105`, `@spec_cynai_client_cliauth`)
  - "Logout clears stored session" (`@req_identy_0106`, `@spec_cynai_client_cliauth`)

## References

- Feature file conventions: `features/README.md`, `docs/docs_standards/spec_authoring_writing_and_validation.md`
- Validator: `.ci_scripts/validate_feature_files.py`
- Gherkin config: `.gherkin-lintrc`
- E2E layout: `scripts/test_scripts/README.md`
