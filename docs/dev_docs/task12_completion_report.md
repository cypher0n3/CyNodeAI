# Task 12 Completion Report - E2E in CI (`no_inference`)

## Summary

Added GitHub Actions job `e2e-no-inference` in `.github/workflows/ci.yml` after `build`, `test-go-cover`, and `test-bdd`.
The job creates the Python venv (`just venv`), sets `CONTAINER_RUNTIME=docker`, starts the dev stack (`just setup-dev start`), runs `just e2e --tags no_inference`, and stops the stack in an `always()` cleanup step (`just setup-dev stop`).

The workflow header comment now documents that E2E is part of CI (not only local/self-hosted).

## Validation

- `just ci` (local): build-dev, lint, vulncheck-go, test-go-cover, bdd-ci - green after `cynode-sba` coverage tests.
- Full `just e2e --tags no_inference` requires a running Docker stack; validate on the next push/PR run of `e2e-no-inference` (image builds can take significant time on `ubuntu-latest`).

## Plan

YAML `st-122`-`st-128` and Task 12 markdown checklists updated in `docs/dev_docs/_plan_003_short_term.md`.
