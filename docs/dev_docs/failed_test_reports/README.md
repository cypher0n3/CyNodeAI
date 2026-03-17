# Failed E2E Test Reports

## Purpose

This directory holds **failed E2E report** artifacts: one markdown file per failed test run.
Each report documents why a specific E2E test failed, which code paths were involved, and what needs to be fixed.

Reports are temporary working documents; see the [dev_docs cleanup requirement](../README.md#pre-merge-cleanup-requirement).

## Report Naming

Files are named:

```text
YYYY-MM-DD_e2e_NNN_test_name.md
```

where `NNN` is the **historical** e2e number at the time the report was written (e.g. `e2e_050`, `e2e_127`).

## E2E Test Renames (Context)

E2E tests were renumbered into a logical 4-digit order (e.g. `e2e_0010_*` … `e2e_0800_*`).
Scripts live under [scripts/test_scripts/](../../../scripts/test_scripts/); see [scripts/test_scripts/README.md](../../../scripts/test_scripts/README.md) for the current list and tags.

- **Report filenames** in this directory were **not** renamed (e.g. `2026-03-16_e2e_050_test_task_create.md` stays as-is) so existing links keep working.
- **Report content** was updated to reference the **new** script names and paths (e.g. `e2e_0420_task_create.py`, `e2e_0610_sse_streaming.py`).
- When a report says "script: `e2e_0420_task_create.py`", the corresponding report file may still be named with the old number (`e2e_050_*`).
  Use the script name in the report body as the source of truth for locating the test file.

## See Also

- [scripts/test_scripts/README.md](../../../scripts/test_scripts/README.md) — E2E test list, tags, and how to run
- [dev_docs README](../README.md) — Temporary doc policy and cleanup
