# Python E2E Tests Added (2026-02-28)

## Summary

Added Python E2E tests to cover more of the current implementation, including SBA tests that require inference where applicable.

## Naming (2026-02-28 Renames)

Modules use `e2e_NNN_descriptive_name.py` with zero-padded NNN in steps of 10 (010, 020, ... 200).
Gaps (e.g. 015 between 010 and 020) allow inserting new tests without renumbering.
Full list and run order: `scripts/test_scripts/README.md`.

## Running

- Full suite: `just e2e` or `PYTHONPATH=. python scripts/test_scripts/run_e2e.py`.
- Without Ollama smoke: `just e2e --skip-ollama` (inference-related tests skip via skipTest).
- List tests: `PYTHONPATH=. python scripts/test_scripts/run_e2e.py --list`.

## Lint

`PYTHONPATH=. flake8 scripts/test_scripts --max-line-length=100` and `pylint` on `scripts/test_scripts` pass.
