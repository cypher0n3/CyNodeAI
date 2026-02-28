# Scripts

- [Overview](#overview)

## Overview

- **setup-dev.sh** - Bash dev setup (unchanged).
  Commands: start-db, stop-db, clean-db, migrate, build, start, stop, test-e2e, full-demo, help.
- **setup_dev.py** - Python dev setup with the same commands.
  **No bash dependency**; all logic in Python.
  - **start-db, stop-db, clean-db** - Standalone Postgres container (podman/docker).
  - **migrate** - No-op (migrations when control-plane starts).
  - **build** - `just build`.
  - **build-e2e-images** - Build inference-proxy and cynode-sba images.
  - **start** - Build, compose up (orchestrator stack), wait for control-plane, start node-manager.
  - **stop** - Kill node-manager, free worker port, compose down, rm containers.
  - **test-e2e** - Run `scripts/test_scripts/run_e2e.py --parity-only`.
  - **full-demo** - Build, build E2E images, start, run Python E2E, optionally stop. `--stop-on-success` or `STOP_ON_SUCCESS_ENV=1`.
  - **help** - Show usage.

Run from repo root (PYTHONPATH required so `scripts` package resolves):

```bash
PYTHONPATH=. python scripts/setup_dev.py build
PYTHONPATH=. python scripts/setup_dev.py test-e2e
PYTHONPATH=. python scripts/setup_dev.py full-demo --stop-on-success
```

Environment variables are the same as for setup-dev.sh (see `python scripts/setup_dev.py help`).
