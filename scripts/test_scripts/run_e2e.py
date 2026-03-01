#!/usr/bin/env python3
# Run E2E suite (discovers e2e_*.py in scripts/test_scripts). Use from repo root:
#   PYTHONPATH=. python scripts/test_scripts/run_e2e.py [OPTIONS]
# Services must be up (e.g. just setup-dev start then run this, or full-demo).

import argparse
import os
import subprocess
import sys
import time
import unittest

from scripts.test_scripts import config, helpers

# Repo root (parent of scripts/)
_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def parse_args():
    """Parse our options; return (namespace, remaining argv for unittest)."""
    p = argparse.ArgumentParser(
        description="Run E2E tests (discovers e2e_*.py in scripts/test_scripts).",
        epilog="Pass -k PATTERN, -v, -f to filter/verbosity/failfast (unittest).",
    )
    p.add_argument(
        "--no-build",
        action="store_true",
        help="Skip building cynork-dev; use existing binary (faster re-runs)",
    )
    p.add_argument(
        "--skip-ollama",
        action="store_true",
        help="Skip Ollama inference smoke and one-shot chat (set E2E_SKIP_INFERENCE_SMOKE)",
    )
    p.add_argument(
        "--list",
        action="store_true",
        help="List test names and exit (no run)",
    )
    return p.parse_known_args()


def ensure_cynork_dev():
    """Build cynork-dev via just (parity with setup-dev.sh). Return True on success."""
    try:
        r = subprocess.run(
            ["just", "build-cynork-dev"],
            cwd=config.PROJECT_ROOT,
            capture_output=True,
            text=True,
            timeout=120,
            check=False,
        )
        if r.returncode:
            print("Error: just build-cynork-dev failed", file=sys.stderr)
            if r.stderr:
                print(r.stderr, file=sys.stderr)
            return False
    except FileNotFoundError:
        print("Error: just not found. Install just to build cynork-dev.", file=sys.stderr)
        return False
    except subprocess.TimeoutExpired:
        print("Error: just build-cynork-dev timed out", file=sys.stderr)
        return False
    if not os.path.isfile(config.CYNORK_BIN) or not os.access(config.CYNORK_BIN, os.X_OK):
        print("Error: cynork-dev binary not found at path below.", file=sys.stderr)
        print(config.CYNORK_BIN, file=sys.stderr)
        return False
    return True


def main():
    opts, unknown = parse_args()
    # Leave only script name + unittest args (-k, -v, -f) for unittest
    sys.argv = [sys.argv[0]] + unknown

    loader = unittest.TestLoader()
    start_dir = os.path.join(_ROOT, "scripts", "test_scripts")
    suite = loader.discover(start_dir, pattern="e2e_*.py")

    if opts.list:
        def iter_tests(suite_or_case):
            try:
                for t in suite_or_case:
                    yield from iter_tests(t)
            except TypeError:
                yield suite_or_case
        for t in iter_tests(suite):
            print(t.id())
        sys.exit(0)

    if not opts.no_build and not ensure_cynork_dev():
        sys.exit(1)
    if opts.no_build:
        if not os.path.isfile(config.CYNORK_BIN) or not os.access(config.CYNORK_BIN, os.X_OK):
            print(
                "Error: cynork-dev not found. Run without --no-build or: just build-cynork-dev",
                file=sys.stderr,
            )
            print(config.CYNORK_BIN, file=sys.stderr)
            sys.exit(1)

    if opts.skip_ollama:
        os.environ["E2E_SKIP_INFERENCE_SMOKE"] = "1"

    if not helpers.wait_for_gateway():
        print("Error: user-gateway not ready (healthz) after 30s", file=sys.stderr)
        sys.exit(1)
    time.sleep(3)

    if not helpers.run_ollama_inference_smoke():
        print("Error: Ollama inference smoke failed", file=sys.stderr)
        sys.exit(1)

    runner = unittest.runner.TextTestRunner(verbosity=2)
    result = runner.run(suite)
    sys.exit(0 if result.wasSuccessful() else 1)


if __name__ == "__main__":
    main()
