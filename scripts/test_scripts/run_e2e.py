#!/usr/bin/env python3
"""Run E2E suite (discovers e2e_*.py in scripts/test_scripts).

Use from repo root: PYTHONPATH=. python scripts/test_scripts/run_e2e.py [OPTIONS]
Services must be up (e.g. just setup-dev start, or full-demo).
"""

import argparse
import os
import subprocess
import sys
import unittest

from scripts.test_scripts import config, helpers
from scripts.test_scripts import e2e_tags

# Repo root (parent of scripts/)
_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def parse_args():
    """Parse our options; return (namespace, remaining argv for unittest)."""
    p = argparse.ArgumentParser(
        description="Run E2E tests (discovers e2e_*.py in scripts/test_scripts).",
        epilog=(
            "Pass -k PATTERN, -v, -f to filter/verbosity/failfast (unittest). "
            "Tags match features/README.md suite tags (e.g. suite_orchestrator)."
        ),
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
    p.add_argument(
        "--tags",
        type=str,
        default="",
        help="Comma-separated suite tags to include (e.g. suite_worker_node,suite_cynork)",
    )
    p.add_argument(
        "--exclude-tags",
        type=str,
        default="",
        help="Comma-separated tags to exclude (e.g. wip)",
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


def _discover_suite(opts):
    """Discover E2E tests and apply tag filter. Return TestSuite."""
    loader = unittest.TestLoader()
    start_dir = os.path.join(_ROOT, "scripts", "test_scripts")
    suite = loader.discover(start_dir, pattern="e2e_*.py")
    include_tags = [t.strip() for t in (opts.tags or "").split(",") if t.strip()]
    exclude_tags = [t.strip() for t in (opts.exclude_tags or "").split(",") if t.strip()]
    if include_tags or exclude_tags:
        suite = e2e_tags.filter_suite_by_tags(
            suite, include_tags=include_tags or None, exclude_tags=exclude_tags or None
        )
    return suite


def _iter_tests(suite_or_case):
    """Recursively yield individual test cases from a suite."""
    try:
        for t in suite_or_case:
            yield from _iter_tests(t)
    except TypeError:
        yield suite_or_case


def _ensure_cynork_ready(opts):
    """Build or verify cynork-dev; exit 1 on failure."""
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


def _run_prereq_checks():
    """Wait for gateway and run Ollama smoke; exit 1 on failure."""
    if not helpers.wait_for_gateway():
        print("Error: user-gateway not ready (healthz) after 30s", file=sys.stderr)
        sys.exit(1)
    if not helpers.wait_for_gateway_readyz(timeout_sec=30):
        print("Error: user-gateway readyz not 200 after 30s", file=sys.stderr)
        sys.exit(1)
    if not helpers.run_ollama_inference_smoke():
        print("Error: Ollama inference smoke failed", file=sys.stderr)
        sys.exit(1)


def _proxy_pma_only(opts):
    """True when only suite_proxy_pma is requested (minimal services; no gateway/cynork)."""
    include = [t.strip() for t in (opts.tags or "").split(",") if t.strip()]
    return include == ["suite_proxy_pma"]


def main():
    """Discover and run E2E tests; exit 0 on success, 1 on failure or setup error."""
    opts, unknown = parse_args()
    sys.argv = [sys.argv[0]] + unknown

    suite = _discover_suite(opts)
    if opts.list:
        for t in _iter_tests(suite):
            print(t.id())
        sys.exit(0)

    if _proxy_pma_only(opts):
        # Proxy + PMA tests start their own minimal services; no gateway or cynork.
        pass
    else:
        _ensure_cynork_ready(opts)
        if opts.skip_ollama:
            os.environ["E2E_SKIP_INFERENCE_SMOKE"] = "1"
        _run_prereq_checks()

    result = unittest.runner.TextTestRunner(verbosity=2).run(suite)
    sys.exit(0 if result.wasSuccessful() else 1)


if __name__ == "__main__":
    main()
