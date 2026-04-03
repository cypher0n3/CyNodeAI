# Traces: REQ-ORCHES-0188, REQ-ORCHES-0190, REQ-ORCHES-0192;
# docs/tech_specs/orchestrator_bootstrap.md (managed PMA warm pool helpers for E2E).

"""Podman/docker PMA container polling (split from helpers for maintainability index)."""

import subprocess
import time


def container_runtime_ps_available():
    """True if ``podman ps`` or ``docker ps`` succeeds (exit 0)."""
    for bin_name in ("podman", "docker"):
        try:
            proc = subprocess.run(
                [bin_name, "ps"],
                capture_output=True,
                text=True,
                timeout=15,
                check=False,
            )
        except (FileNotFoundError, OSError, subprocess.TimeoutExpired):
            continue
        if not proc.returncode:
            return True
    return False


def list_runtime_managed_pma_container_names():
    """Running container names matching ``cynodeai-managed-pma`` (podman or docker).

    Node-manager does not upsert these into worker telemetry ``container_inventory``;
    this matches observable state in orchestrator_bootstrap / PMA warm-pool specs.
    """
    needle = "cynodeai-managed-pma"
    names = []
    for bin_name in ("podman", "docker"):
        try:
            proc = subprocess.run(
                [bin_name, "ps", "--format", "{{.Names}}"],
                capture_output=True,
                text=True,
                timeout=15,
                check=False,
            )
        except (FileNotFoundError, OSError, subprocess.TimeoutExpired):
            continue
        if proc.returncode or not (proc.stdout or "").strip():
            continue
        for line in proc.stdout.splitlines():
            n = line.strip()
            if needle in n:
                names.append(n)
        if names:
            return sorted(set(names))
    return []


# Worker-managed PMA uses orchestrator warm-pool ids: cynodeai-managed-pma-pool-<n>
_PMA_POOL_SUBSTR = "cynodeai-managed-pma-pool-"

# When at least this many warm-pool containers already run (e.g. after chat E2Es), gateway
# login usually binds idle slots — no new container *names* vs a prior snapshot. Full-suite
# ordering then must not require ``wait_for_at_least_new_pma_sb_container_names`` to succeed.
PMA_POOL_E2E_REUSE_NAME_THRESHOLD = 3


def pma_pool_login_unlikely_to_add_new_names(before_pool_count: int) -> bool:
    """True when login probably reuses existing pool slots (no new names vs ``before``)."""
    return int(before_pool_count) >= PMA_POOL_E2E_REUSE_NAME_THRESHOLD


def count_runtime_pma_pool_container_names(names):
    """Count warm-pool PMA containers (``…-managed-pma-pool-…``)."""
    if not names:
        return 0
    return sum(1 for n in names if _PMA_POOL_SUBSTR in n)


def count_runtime_pma_sb_container_names(names):
    """Deprecated alias for :func:`count_runtime_pma_pool_container_names`."""
    return count_runtime_pma_pool_container_names(names)


def wait_for_runtime_pma_sb_count_at_least(min_sb, timeout_sec=180, poll_sec=4):
    """Poll until warm-pool PMA container count >= ``min_sb``.

    Returns (ok, last_names, last_sb_count).
    """
    deadline = time.monotonic() + float(timeout_sec)
    last_names = []
    last_sb = 0
    while time.monotonic() < deadline:
        last_names = list_runtime_managed_pma_container_names()
        last_sb = count_runtime_pma_pool_container_names(last_names)
        if last_sb >= min_sb:
            return True, last_names, last_sb
        time.sleep(float(poll_sec))
    return False, last_names, last_sb


def runtime_pma_pool_container_name_set():
    """Set of running warm-pool PMA container names."""
    return {
        n
        for n in list_runtime_managed_pma_container_names()
        if _PMA_POOL_SUBSTR in n
    }


def runtime_pma_sb_container_name_set():
    """Deprecated alias for :func:`runtime_pma_pool_container_name_set`."""
    return runtime_pma_pool_container_name_set()


def wait_for_at_least_new_pma_sb_container_names(
    before_names, min_new, timeout_sec=180, poll_sec=4
):
    """Poll until at least ``min_new`` new warm-pool PMA containers vs ``before_names``.

    ``before_names`` is a set of full container names. Returns
    (ok, new_names_set, all_pool_names_set).
    """
    deadline = time.monotonic() + float(timeout_sec)
    before = set(before_names)
    last_new = set()
    last_all = set()
    while time.monotonic() < deadline:
        last_all = runtime_pma_pool_container_name_set()
        last_new = last_all - before
        if len(last_new) >= int(min_new):
            return True, last_new, last_all
        time.sleep(float(poll_sec))
    return False, last_new, last_all


def wait_until_managed_container_names_absent(container_names, timeout_sec=180, poll_sec=4):
    """Poll until none of ``container_names`` appear in managed PMA container names."""
    want_gone = frozenset(str(n).strip() for n in container_names if str(n).strip())
    if not want_gone:
        return True
    deadline = time.monotonic() + float(timeout_sec)
    while time.monotonic() < deadline:
        current = set(list_runtime_managed_pma_container_names())
        if want_gone.isdisjoint(current):
            return True
        time.sleep(float(poll_sec))
    return False


def wait_until_some_pma_sb_names_removed(
    names_before_logout, timeout_sec=180, poll_sec=4
):
    """Poll until at least one ``names_before_logout`` name is gone (logout / teardown).

    ``names_before_logout`` is usually ``new_sb`` from
    ``wait_for_at_least_new_pma_sb_container_names``.
    """
    want = frozenset(str(n).strip() for n in names_before_logout if str(n).strip())
    if not want:
        return True
    deadline = time.monotonic() + float(timeout_sec)
    while time.monotonic() < deadline:
        cur = runtime_pma_pool_container_name_set()
        if not want.issubset(cur):
            return True
        time.sleep(float(poll_sec))
    return False


def wait_until_runtime_pma_pool_at_most(max_slots, timeout_sec=180, poll_sec=4):
    """Poll until warm-pool PMA container count <= ``max_slots`` (idle baseline after teardown)."""
    cap = int(max_slots)
    deadline = time.monotonic() + float(timeout_sec)
    while time.monotonic() < deadline:
        if len(runtime_pma_pool_container_name_set()) <= cap:
            return True
        time.sleep(float(poll_sec))
    return False


def wait_until_runtime_pma_sb_empty(timeout_sec=180, poll_sec=4):
    """Poll until only the minimum warm pool remains (default: one ``pma-pool-*`` container)."""
    return wait_until_runtime_pma_pool_at_most(1, timeout_sec, poll_sec)


def wait_until_runtime_pma_sb_count_below(previous_count, timeout_sec=180, poll_sec=4):
    """Poll until ``len(runtime_pma_pool_container_name_set()) < previous_count``."""
    prev = int(previous_count)
    deadline = time.monotonic() + float(timeout_sec)
    while time.monotonic() < deadline:
        if len(runtime_pma_pool_container_name_set()) < prev:
            return True
        time.sleep(float(poll_sec))
    return False
