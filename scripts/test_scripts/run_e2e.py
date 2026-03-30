#!/usr/bin/env python3
"""Run E2E suite (discovers e2e_*.py in scripts/test_scripts).

Use from repo root: PYTHONPATH=. python scripts/test_scripts/run_e2e.py [OPTIONS]
Services must be up (e.g. just setup-dev start, or full-demo).

Use --single TEST_ID to run only that test or module (no prereqs).
E.g. just e2e --single e2e_0020_gateway_health_readyz.

Timeouts (tune via env; see scripts/test_scripts/config.py):
  E2E_CYNORK_TIMEOUT — default limit for cynork subprocess when tests omit timeout=.
  E2E_SSE_REQUEST_TIMEOUT — HTTP read timeout for SSE streaming requests.
  OLLAMA_SMOKE_CHAT_TIMEOUT — Ollama /api/chat deadline for the ollama prereq smoke.
  --timeout (only with --single) — optional outer wall clock; default 0 = unlimited.
"""

import argparse
import os
import subprocess
import sys
import unittest

from scripts.test_scripts import config, helpers, ollama_e2e_helpers
from scripts.test_scripts import e2e_tags
import scripts.test_scripts.e2e_state as state

# Repo root (parent of scripts/)
_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def parse_args():
    """Parse our options; return (namespace, remaining argv for unittest)."""
    p = argparse.ArgumentParser(
        description="Run E2E tests (discovers e2e_*.py in scripts/test_scripts).",
        epilog=(
            "Use --single TEST_ID to run one test or module only; stack must be up. "
            "Tags: suite_* (suite_orchestrator, suite_worker_node, ...), full_demo, "
            "inference, pma_inference, sba_inference, auth, task, chat, worker, pma."
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
        help="Comma-separated tags to include (e.g. full_demo, suite_worker_node, inference)",
    )
    p.add_argument(
        "--exclude-tags",
        type=str,
        default="",
        help="Comma-separated tags to exclude (e.g. wip)",
    )
    p.add_argument(
        "--single",
        type=str,
        metavar="TEST_ID",
        default="",
        help=(
            "Run only this test or module; no prereq modules. E.g. e2e_0010_cli_version_and_status "
            "or e2e_0020_gateway_health_readyz. Stack must be up. "
            "Tests that need auth/task state may fail."
        ),
    )
    p.add_argument(
        "--timeout",
        type=int,
        default=0,
        metavar="SECS",
        help=(
            "When --single: optional max seconds for the whole run (subprocess kill). "
            "Default 0 = no limit (long inference runs are not cut off). "
            "Set e.g. 3600 in CI if you need a hard cap."
        ),
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


def _normalize_single_test_id(test_id):
    """Return unittest-style test name; prefix scripts.test_scripts. if needed."""
    id_str = (test_id or "").strip()
    if not id_str:
        return ""
    if id_str.startswith("scripts."):
        return id_str
    return "scripts.test_scripts." + id_str


def _suite_for_single_test(loader, single_id):
    """Build suite: only the requested test or module. No prereqs. Raise on load error."""
    suite = unittest.suite.TestSuite()
    try:
        suite.addTests(loader.loadTestsFromName(single_id))
    except (AttributeError, ModuleNotFoundError, ValueError) as e:
        raise SystemExit(f"Error loading test {single_id}: {e}") from e
    return suite


def _discover_suite(opts):
    """Discover E2E tests and apply tag filter (or single-test + prereqs). Return TestSuite."""
    loader = unittest.TestLoader()
    single_id = _normalize_single_test_id(getattr(opts, "single", "") or "")
    if single_id:
        return _suite_for_single_test(loader, single_id)
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


def _collect_suite_prereqs(suite):
    """Return union of whitelisted prereqs from all tests in the suite."""
    required = set()
    for test in _iter_tests(suite):
        if isinstance(test, unittest.TestCase):
            required |= e2e_tags.get_prereqs_for_test(test)
    return required


def _run_single_prereq(name, opts):
    """Run one prereq step. Return True on success, False on failure."""
    if name == e2e_tags.PREREQ_GATEWAY:
        if not helpers.wait_for_gateway():
            print("Error: user-gateway not ready (healthz) after 30s", file=sys.stderr)
            return False
        if not helpers.wait_for_gateway_readyz(timeout_sec=30):
            print("Error: user-gateway readyz not 200 after 30s", file=sys.stderr)
            return False
    elif name == e2e_tags.PREREQ_CONFIG:
        state.init_config()
    elif name == e2e_tags.PREREQ_AUTH:
        if not _ensure_shared_auth_config():
            return False
    elif name == e2e_tags.PREREQ_NODE_REGISTER:
        if not helpers.ensure_node_registered():
            print(
                "Error: node register prereq failed (control-plane /v1/nodes/register)",
                file=sys.stderr,
            )
            return False
    elif name == e2e_tags.PREREQ_TASK_ID:
        if not helpers.ensure_e2e_task(state.CONFIG_PATH):
            print(
                "Warning: ensure_e2e_task failed; tests requiring state.TASK_ID "
                "may skip or fail.",
                file=sys.stderr,
            )
            return False
    elif name == e2e_tags.PREREQ_OLLAMA:
        if opts.skip_ollama:
            os.environ["E2E_SKIP_INFERENCE_SMOKE"] = "1"
        else:
            if not ollama_e2e_helpers.run_ollama_inference_smoke():
                print("Error: Ollama inference smoke failed", file=sys.stderr)
                return False
            include_tags = [t.strip() for t in (opts.tags or "").split(",") if t.strip()]
            if "pma_inference" in include_tags:
                if not ollama_e2e_helpers.ollama_container_running():
                    print(
                        "Error: pma_inference needs stack started with Ollama.",
                        file=sys.stderr,
                    )
                    return False
                # After a fresh stack restart, Ollama may load a multi-GB model into GPU/ROCm
                # for 5–10+ minutes before the first chat returns 2xx; 180s is too tight.
                if not helpers.wait_for_pma_chat_ready(
                    timeout_sec=600, poll_interval=5
                ):
                    print("Error: PMA chat not ready within 600s", file=sys.stderr)
                    return False
    return True


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


def _ensure_shared_auth_config():
    """Ensure shared E2E auth config exists with a valid token for dependent tests.

    Returns True on success, False on failure (caller records prereq failure).
    """
    def _login():
        ok, out, err = helpers.run_cynork(
            ["auth", "login", "-u", "admin", "--password-stdin"],
            state.CONFIG_PATH,
            input_text=f"{config.ADMIN_PASSWORD}\n",
        )
        if not ok:
            print("Error: auth login prereq failed for shared E2E config", file=sys.stderr)
            if out:
                print(out, file=sys.stderr)
            if err:
                print(err, file=sys.stderr)
            return False
        return True

    state.init_config()
    token = helpers.read_token_from_config(state.CONFIG_PATH)
    if not token:
        return _login()
    ok, out, err = helpers.run_cynork(["auth", "whoami"], state.CONFIG_PATH, timeout=30)
    if ok:
        return True
    merged = f"{out}\n{err}".lower()
    if "invalid or expired token" in merged:
        refreshed, _, _ = helpers.run_cynork(["auth", "refresh"], state.CONFIG_PATH, timeout=30)
        if refreshed:
            recheck_ok, _, _ = helpers.run_cynork(
                ["auth", "whoami"], state.CONFIG_PATH, timeout=30
            )
            if recheck_ok:
                return True
    return _login()


def _proxy_pma_only(opts):
    """True when only suite_proxy_pma is requested (minimal services; no gateway/cynork)."""
    include = [t.strip() for t in (opts.tags or "").split(",") if t.strip()]
    return include == ["suite_proxy_pma"]


class _PrereqFilterSuite(unittest.TestSuite):
    """Run prereqs per-test; skip if a required prereq previously failed.

    Prereqs that already succeeded are not re-run, except those in
    PREREQ_ALWAYS_RERUN (e.g. auth) which run before every test that needs them.
    """

    def __init__(self, suite, failed_prereqs, succeeded_prereqs, opts):
        super().__init__()
        self._suite = suite
        self._failed_prereqs = failed_prereqs
        self._succeeded_prereqs = succeeded_prereqs
        self._opts = opts

    def run(self, result, *_):
        for case in _iter_tests(self._suite):
            if not isinstance(case, unittest.TestCase):
                continue
            prereqs = e2e_tags.get_prereqs_for_test(case)
            if prereqs & self._failed_prereqs:
                result.startTest(case)
                reason = "Prereq(s) failed: " + ", ".join(
                    sorted(prereqs & self._failed_prereqs)
                )
                result.addSkip(case, reason)
                result.stopTest(case)
                continue
            # Run only prereqs this test needs that haven't succeeded (or are always rerun).
            skip = False
            for name in e2e_tags.PREREQ_ORDER:
                if name not in prereqs:
                    continue
                if name in self._succeeded_prereqs and name not in e2e_tags.PREREQ_ALWAYS_RERUN:
                    continue
                if not _run_single_prereq(name, self._opts):
                    self._failed_prereqs.add(name)
                    result.startTest(case)
                    result.addSkip(case, f"Prereq(s) failed: {name}")
                    result.stopTest(case)
                    skip = True
                    break
                self._succeeded_prereqs.add(name)
            if not skip:
                case.run(result)


def main():
    """Discover and run E2E tests; exit 0 on success, 1 on failure or setup error."""
    argv_full = list(sys.argv)
    opts, unknown = parse_args()
    sys.argv = [sys.argv[0]] + unknown

    # When --single and timeout > 0, run in subprocess so we can enforce a hard cap
    # (avoids hung runs).
    single_id = _normalize_single_test_id(getattr(opts, "single", "") or "")
    timeout_sec = getattr(opts, "timeout", 300) or 0
    if single_id and timeout_sec > 0 and os.environ.get("E2E_NO_TIMEOUT_WRAP") != "1":
        env = os.environ.copy()
        env["E2E_NO_TIMEOUT_WRAP"] = "1"
        # Must use argv_full[1:]: argparse strips known flags from sys.argv before this point.
        argv = [sys.executable, os.path.abspath(__file__)] + argv_full[1:]
        try:
            proc = subprocess.run(
                argv,
                cwd=_ROOT,
                env=env,
                timeout=timeout_sec,
                check=False,
            )
            sys.exit(proc.returncode)
        except subprocess.TimeoutExpired:
            print(
                f"Error: run exceeded {timeout_sec}s (--timeout); terminated.",
                file=sys.stderr,
            )
            sys.exit(1)

    suite = _discover_suite(opts)
    if opts.list:
        for t in _iter_tests(suite):
            print(t.id())
        sys.exit(0)

    failed_prereqs = set()
    succeeded_prereqs = set()
    if _proxy_pma_only(opts):
        # Proxy + PMA tests start their own minimal services; no gateway or cynork.
        pass
    else:
        _ensure_cynork_ready(opts)

    runnable = _PrereqFilterSuite(suite, failed_prereqs, succeeded_prereqs, opts)
    result = unittest.runner.TextTestRunner(verbosity=2).run(runnable)
    ok = result.wasSuccessful() and not failed_prereqs
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
