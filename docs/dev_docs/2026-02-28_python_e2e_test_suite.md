# Python E2E Test Suite

## Summary

**Date:** 2026-02-28

A Python-based E2E test suite was added under `scripts/test_scripts/` with parity to the bash flow in `scripts/setup-dev.sh` (`run_e2e_test`).
The existing bash scripts are unchanged; the justfile was not modified.

## Layout

- `scripts/test_scripts/README.md` - How to run, env vars, lint
- `scripts/test_scripts/config.py` - Env-derived config (ports, cynork path, auth, etc.)
- `scripts/test_scripts/helpers.py` - `run_cynork`, `run_curl`, `wait_for_gateway`, Ollama smoke, JSON helpers
- `scripts/test_scripts/e2e_state.py` - Shared state (config dir, task_id, node_jwt) across scripts
- `scripts/test_scripts/run_e2e.py` - Entrypoint: wait for gateway + Ollama smoke, then run parity (ordered) or discover `e2e_*.py`
- One test per script: `e2e_01_login.py` ... `e2e_09_logout.py` (parity order: login, whoami, task create/get/result, inference 5b, prompt 5c, models+chat 5d, SBA 5e, node register, capability, refresh, logout)
- `scripts/test_scripts/__init__.py` - Package marker

## Parity

The suite covers the same steps as `run_e2e_test`: gateway health wait, optional Ollama inference smoke, cynork auth (login, whoami, refresh, logout), task create/get/result, inference-in-sandbox (if `INFERENCE_PROXY_IMAGE`), prompt task, models list and one-shot chat (unless `E2E_SKIP_INFERENCE_SMOKE`), SBA task, control-plane node registration and capability report.

## Running

From repo root, with stack up and cynork built:

```bash
PYTHONPATH=. python scripts/test_scripts/run_e2e.py
```

Or run all `e2e_*.py` modules:

<!-- fenced-code-under-heading allow -->
```bash
PYTHONPATH=. python scripts/test_scripts/run_e2e.py
```

## Lint

- `just lint-python` (default paths `scripts,.ci_scripts`) passes and is unchanged.
- To lint `scripts/test_scripts`, run flake8 and pylint on `scripts/test_scripts` from repo root with `PYTHONPATH=.` (see README in `scripts/test_scripts`).
