# Implementation of setup_dev commands (no bash). Parity with setup-dev.sh.

import contextlib
import os
import signal
import subprocess
import sys
import traceback
import time
import urllib.error
import urllib.request

# Import after repo root is on path
import scripts.setup_dev_config as _cfg
import scripts.setup_dev_build_cache as _build_cache

# Mutable state set at start of start_orchestrator_stack_compose; used by stop_all.
OLLAMA_TEARDOWN_STATE = {"was_running_before": False}

# Dev-only secure store master key (32 bytes, base64). Same as worker_node nodemanager_test.go.
# Used when CYNODE_SECURE_STORE_MASTER_KEY_B64 is not set so node-manager can run PMA.
_DEV_SECURE_STORE_MASTER_KEY_B64 = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="


@contextlib.contextmanager
def _popen_no_wait(*args, **kwargs):
    """Context manager for Popen; on exit we do not wait (daemon, cleaned by stop_all)."""
    proc = subprocess.Popen(*args, **kwargs)
    try:
        yield proc
    finally:
        pass  # Do not wait; process is long-lived, stop_all kills by pid


def log_info(msg):
    print(f"[INFO] {msg}", file=sys.stderr)


def log_warn(msg):
    print(f"[WARN] {msg}", file=sys.stderr)
    sys.stderr.flush()


def log_error(msg):
    print(f"[ERROR] {msg}", file=sys.stderr)


def run(cmd, env=None, cwd=None, timeout=300, check=True):
    """Run cmd (list). Return True on success."""
    e = os.environ.copy()
    if env:
        e.update(env)
    try:
        r = subprocess.run(
            cmd,
            cwd=cwd or _cfg.PROJECT_ROOT,
            env=e,
            timeout=timeout,
            check=False,
            shell=False,
        )
        if check and r.returncode:
            return False
        return not r.returncode
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return False


def runtime_cmd(*args):
    """Return [RUNTIME, ...args]. Ensures runtime is set."""
    if not _cfg.ensure_runtime():
        log_error("podman or docker required")
        return None
    return [_cfg.RUNTIME] + list(args)


def container_exists(name, running=True):
    """Check if container exists (and optionally is running)."""
    cmd = runtime_cmd("ps", "--format", "{{.Names}}")
    if not cmd:
        return False
    flag = [] if running else ["-a"]
    try:
        r = subprocess.run(
            cmd + flag,
            capture_output=True,
            text=True,
            timeout=10,
            check=False,
            shell=False,
        )
        return name in (r.stdout or "").strip().splitlines()
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return False


def start_postgres():
    """Start standalone PostgreSQL container (start-db)."""
    if not _cfg.ensure_runtime():
        return False
    rt = _cfg.RUNTIME
    log_info("Starting PostgreSQL container...")
    if container_exists(_cfg.POSTGRES_CONTAINER_NAME, running=True):
        log_info("PostgreSQL container already running")
        return True
    if container_exists(_cfg.POSTGRES_CONTAINER_NAME, running=False):
        log_info("Starting existing PostgreSQL container")
        run([rt, "start", _cfg.POSTGRES_CONTAINER_NAME], check=False)
        log_info("Waiting for PostgreSQL to be ready...")
        for _ in range(30):
            r = subprocess.run(
                [rt, "exec", _cfg.POSTGRES_CONTAINER_NAME, "pg_isready",
                 "-U", _cfg.POSTGRES_USER, "-d", _cfg.POSTGRES_DB],
                capture_output=True,
                timeout=5,
                check=False,
                shell=False,
            )
            if not r.returncode:
                log_info("PostgreSQL is ready")
                return True
            time.sleep(1)
        log_error("PostgreSQL failed to become ready within 30s")
        return False
    run([
        rt, "run", "-d", "--name", _cfg.POSTGRES_CONTAINER_NAME,
        "-e", f"POSTGRES_USER={_cfg.POSTGRES_USER}",
        "-e", f"POSTGRES_PASSWORD={_cfg.POSTGRES_PASSWORD}",
        "-e", f"POSTGRES_DB={_cfg.POSTGRES_DB}",
        "-p", f"{_cfg.POSTGRES_PORT}:5432",
        "-v", "cynodeai-postgres-data:/var/lib/postgresql/data",
        _cfg.POSTGRES_IMAGE,
    ])
    log_info("Waiting for PostgreSQL to be ready...")
    for _ in range(60):
        r = subprocess.run(
            [rt, "exec", _cfg.POSTGRES_CONTAINER_NAME, "pg_isready",
             "-U", _cfg.POSTGRES_USER, "-d", _cfg.POSTGRES_DB],
            capture_output=True,
            timeout=5,
            check=False,
            shell=False,
        )
        if not r.returncode:
            log_info("PostgreSQL is ready")
            return True
        time.sleep(1)
    log_error("PostgreSQL failed to start within 60 seconds")
    return False


def stop_postgres():
    """Stop standalone PostgreSQL container."""
    if not _cfg.ensure_runtime():
        return False
    log_info("Stopping PostgreSQL container...")
    subprocess.run(
        [_cfg.RUNTIME, "stop", _cfg.POSTGRES_CONTAINER_NAME],
        capture_output=True,
        timeout=30,
        check=False,
        shell=False,
    )
    return True


def clean_postgres():
    """Stop and remove PostgreSQL container and volume."""
    if not _cfg.ensure_runtime():
        return False
    rt = _cfg.RUNTIME
    log_info("Cleaning up PostgreSQL container and volume...")
    subprocess.run(
        [rt, "stop", _cfg.POSTGRES_CONTAINER_NAME],
        capture_output=True, check=False, shell=False,
    )
    subprocess.run(
        [rt, "rm", _cfg.POSTGRES_CONTAINER_NAME],
        capture_output=True, check=False, shell=False,
    )
    subprocess.run(
        [rt, "volume", "rm", "cynodeai-postgres-data"],
        capture_output=True, check=False, shell=False,
    )
    return True


def build_binaries():
    """Run just build-dev (faster; no strip/upx)."""
    log_info("Building all dev binaries (just build-dev)...")
    if not run(["just", "build-dev"], timeout=600):
        log_error("just build-dev failed")
        return False
    log_info(
        "Dev binaries built: orchestrator/bin, worker_node/bin, cynork/bin, agents/bin"
    )
    return True


def _image_exists(tag):
    """Return True if the image tag exists locally (podman/docker images -q tag)."""
    cmd = runtime_cmd("images", "-q", tag)
    if not cmd:
        return False
    try:
        r = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=10,
            check=False,
            shell=False,
        )
        return bool((r.stdout or "").strip())
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return False


def build_orchestrator_compose_images(force=False):
    """Build control-plane, user-gateway, cynode-pma images so compose up uses latest code.

    If force is False, skip build when stamp hash matches and all three images exist.
    Set force=True (e.g. from E2E_FORCE_REBUILD) to always rebuild.
    """
    if not _cfg.ensure_runtime():
        return False
    if not os.path.isfile(_cfg.COMPOSE_FILE):
        log_error(f"Compose file not found: {_cfg.COMPOSE_FILE}")
        return False
    stamp_path = os.path.join(_cfg.PROJECT_ROOT, "tmp", "setup_dev_compose_images.stamp")
    compose_images = [
        "cynodeai-control-plane:dev",
        "cynodeai-user-gateway:dev",
        "cynodeai-cynode-pma:dev",
    ]
    if not force:
        current_hash = _build_cache.compute_compose_images_hash(
            _cfg.COMPOSE_FILE, _cfg.PROJECT_ROOT
        )
        stored = _build_cache.read_stamp(stamp_path)
        if (
            stored == current_hash
            and all(_image_exists(tag) for tag in compose_images)
        ):
            log_info("Orchestrator compose images up to date.")
            return True
    log_info("Building orchestrator compose images (control-plane, user-gateway, cynode-pma)...")
    if not run(
        [_cfg.RUNTIME, "compose", "-f", _cfg.COMPOSE_FILE,
         "--profile", "pma", "build", "control-plane", "user-gateway", "cynode-pma"],
        cwd=_cfg.PROJECT_ROOT,
        timeout=600,
    ):
        log_error("Orchestrator compose build failed")
        return False
    os.makedirs(os.path.dirname(stamp_path), exist_ok=True)
    _build_cache.write_stamp(
        stamp_path,
        _build_cache.compute_compose_images_hash(_cfg.COMPOSE_FILE, _cfg.PROJECT_ROOT),
    )
    return True


def get_ollama_was_running_before():
    """Return whether the ollama container was running before we started the stack."""
    return OLLAMA_TEARDOWN_STATE["was_running_before"]


def start_orchestrator_stack_compose(extra_env=None, ollama_in_stack=False):
    """Compose down, rm stray containers, compose up -d with env.

    ollama_in_stack: if True (bypass), include ollama profile so OLLAMA runs in compose.
    Prescribed (default False): orchestrator starts without OLLAMA per
    docs/tech_specs/orchestrator_bootstrap.md CYNAI.BOOTST.OrchestratorIndependentStartup
    and prescribed startup sequence (node starts inference when instructed via config).
    """
    if not _cfg.ensure_runtime():
        return False
    if not os.path.isfile(_cfg.COMPOSE_FILE):
        log_error(f"Compose file not found: {_cfg.COMPOSE_FILE}")
        return False
    OLLAMA_TEARDOWN_STATE["was_running_before"] = container_exists(
        _cfg.OLLAMA_CONTAINER_NAME, running=True
    )
    log_info("=== Orchestrator stack startup ===")
    if ollama_in_stack:
        log_info("  (bypass: OLLAMA in stack)")
    else:
        log_info("  (prescribed: no OLLAMA in stack; node starts inference when instructed)")
    log_info(
        f"  postgres :5432, control-plane :{_cfg.CONTROL_PLANE_PORT}, "
        f"user-gateway :{_cfg.ORCHESTRATOR_PORT}"
    )
    # PMA is always required; build cynode-pma so the node can run it (avoid 403 on default image).
    log_info("  Building cynode-pma image...")
    if not run(
        [_cfg.RUNTIME, "compose", "-f", _cfg.COMPOSE_FILE,
         "--profile", "pma", "build", "cynode-pma"],
        cwd=_cfg.PROJECT_ROOT,
        timeout=600,
    ):
        log_error("cynode-pma image build failed")
        return False
    if ollama_in_stack:
        log_info("  Building control-plane image (PMA-in-stack)...")
        if not run(
            [_cfg.RUNTIME, "compose", "-f", _cfg.COMPOSE_FILE,
             "--profile", "pma", "build", "control-plane"],
            cwd=_cfg.PROJECT_ROOT,
            timeout=600,
        ):
            log_error("control-plane image build failed")
            return False
    env = _cfg.compose_env()
    if extra_env:
        env.update(extra_env)
    env["PMA_IMAGE"] = "cynodeai-cynode-pma:dev"
    env["NODE_PMA_OLLAMA_BASE_URL"] = "http://host.containers.internal:11434"
    # Prescribed: node starts PMA; do not start PMA as control-plane subprocess.
    env["PMA_ENABLED"] = "false"
    # Down: always use all profiles so every profile service is torn down.
    subprocess.run(
        [_cfg.RUNTIME, "compose", "-f", _cfg.COMPOSE_FILE,
         "--profile", "ollama", "--profile", "optional", "--profile", "pma", "down"],
        cwd=_cfg.PROJECT_ROOT,
        capture_output=True,
        timeout=60,
        check=False,
        shell=False,
    )
    subprocess.run(
        [_cfg.RUNTIME, "rm", "-f", _cfg.CONTROL_PLANE_CONTAINER_NAME,
         _cfg.USER_GATEWAY_CONTAINER_NAME, _cfg.PMA_CONTAINER_NAME],
        capture_output=True,
        check=False,
        shell=False,
    )
    # Up: optional always; ollama only when bypass. PMA is started by the worker when instructed.
    # Ref: orchestrator_bootstrap.md Orchestrator Independent Startup, PMA Startup.
    up_profiles = ["--profile", "optional"]
    if ollama_in_stack:
        up_profiles.extend(["--profile", "ollama"])
    if not run(
        [_cfg.RUNTIME, "compose", "-f", _cfg.COMPOSE_FILE] + up_profiles + ["up", "-d"],
        env=env, timeout=600,
    ):
        log_error("Compose up failed")
        return False
    log_info("Orchestrator stack started.")
    return True


def stop_orchestrator_stack_compose(leave_ollama_running=False):
    """Compose down. If leave_ollama_running, bring ollama back up after down."""
    if not _cfg.ensure_runtime():
        return False
    log_info("Stopping orchestrator stack...")
    if os.path.isfile(_cfg.COMPOSE_FILE):
        # Use same profiles as up so optional, ollama, and pma are stopped.
        subprocess.run(
            [_cfg.RUNTIME, "compose", "-f", _cfg.COMPOSE_FILE,
             "--profile", "ollama", "--profile", "optional", "--profile", "pma", "down"],
            cwd=_cfg.PROJECT_ROOT,
            capture_output=True,
            timeout=60,
            check=False,
            shell=False,
        )
        if leave_ollama_running:
            log_info("Restarting ollama (was running before tests).")
            run(
                [_cfg.RUNTIME, "compose", "-f", _cfg.COMPOSE_FILE, "--profile", "ollama",
                 "up", "-d", "ollama"],
                env=_cfg.compose_env(),
                cwd=_cfg.PROJECT_ROOT,
                timeout=60,
            )
    return True


def wait_for_control_plane_listening():
    """Wait for control-plane /readyz 200 or 503.

    Unused by normal startup: node-manager polls orchestrator readiness (worker_node spec).
    Kept for optional flows that need control-plane up without starting a node.
    """
    url = f"http://127.0.0.1:{_cfg.CONTROL_PLANE_PORT}/readyz"
    log_info(f"Waiting for control-plane at {url} (up to 90s)...")
    for _ in range(90):
        try:
            req = urllib.request.Request(url)
            with urllib.request.urlopen(req, timeout=2) as resp:
                code = resp.getcode()
                if code in (200, 503):
                    log_info(f"Control-plane is listening (readyz {code})")
                    return True
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(1)
    log_error("Control-plane not listening after 90s")
    return False


def wait_for_control_plane_stopped(timeout_sec=15):
    """Wait until control-plane is no longer reachable (e.g. after stop_all). True when stopped."""
    url = f"http://127.0.0.1:{_cfg.CONTROL_PLANE_PORT}/readyz"
    for _ in range(timeout_sec):
        try:
            req = urllib.request.Request(url)
            with urllib.request.urlopen(req, timeout=1) as _:
                pass
        except (OSError, urllib.error.URLError):
            log_info("Control-plane is stopped (readyz unreachable)")
            return True
        time.sleep(1)
    log_warn(f"Control-plane still reachable after {timeout_sec}s")
    return False


def wait_for_orchestrator_readyz(timeout_sec=120):
    """Wait for control-plane /readyz 200 (inference path + worker-reported PMA ready)."""
    url = f"http://127.0.0.1:{_cfg.CONTROL_PLANE_PORT}/readyz"
    log_info(f"Waiting for orchestrator ready at {url} (up to {timeout_sec}s)...")
    for _ in range(timeout_sec):
        try:
            req = urllib.request.Request(url)
            with urllib.request.urlopen(req, timeout=2) as resp:
                if resp.getcode() == 200:
                    log_info("Orchestrator is ready (readyz 200)")
                    return True
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(1)
    log_error(f"Orchestrator not ready (readyz 200) after {timeout_sec}s")
    return False


def _log_proc_streams(proc):
    """Log stdout/stderr of a process that has exited (for diagnostics)."""
    for label, stream in [("stdout", proc.stdout), ("stderr", proc.stderr)]:
        if not stream:
            continue
        try:
            out = stream.read()
            if out:
                decoded = out.decode("utf-8", errors="replace").strip()
                log_error(f"node-manager {label}:\n{decoded}")
        except OSError:
            pass


def start_node(extra_env=None):
    """Start node-manager in background; wait for worker-api healthz.

    The node-manager is responsible for polling the orchestrator control-plane /readyz before
    registering (worker_node startup procedure; see docs/tech_specs/worker_node.md). This script
    does not wait for control-plane; it starts the node immediately after compose up.
    extra_env: optional dict merged into node process env (e.g.
    INFERENCE_PROXY_IMAGE, OLLAMA_UPSTREAM_URL for full-demo).
    """
    env = os.environ.copy()
    if extra_env:
        env.update(extra_env)
    env["ORCHESTRATOR_URL"] = f"http://localhost:{_cfg.CONTROL_PLANE_PORT}"
    env["NODE_REGISTRATION_PSK"] = _cfg.NODE_PSK
    env["NODE_SLUG"] = _cfg.NODE_SLUG
    env["NODE_NAME"] = _cfg.NODE_NAME
    env["NODE_MANAGER_WORKER_API_BIN"] = _cfg.NODE_MANAGER_WORKER_API_BIN
    env["LISTEN_ADDR"] = f":{_cfg.WORKER_PORT}"
    env["NODE_ADVERTISED_WORKER_API_URL"] = os.environ.get(
        "NODE_ADVERTISED_WORKER_API_URL",
        f"http://{_cfg.CONTAINER_HOST_ALIAS}:{_cfg.WORKER_PORT}",
    )
    env["CONTAINER_RUNTIME"] = os.environ.get("CONTAINER_RUNTIME", "podman")
    # Known state dir for E2E secure store (e2e_122); node writes secrets here.
    state_dir = (
        os.environ.get("WORKER_API_STATE_DIR")
        or os.environ.get("NODE_STATE_DIR")
        or _cfg.NODE_STATE_DIR
    )
    try:
        os.makedirs(state_dir, 0o700, exist_ok=True)
    except OSError as e:
        log_error(f"failed to create node state dir {state_dir}: {e}")
        return False
    env["WORKER_API_STATE_DIR"] = state_dir
    # Secure store master key required for managed service (PMA) token lifecycle. Dev-only default.
    if not os.environ.get("CYNODE_SECURE_STORE_MASTER_KEY_B64", "").strip():
        env["CYNODE_SECURE_STORE_MASTER_KEY_B64"] = _DEV_SECURE_STORE_MASTER_KEY_B64
    if not os.path.isfile(_cfg.NODE_MANAGER_BIN):
        log_error(f"node-manager not found: {_cfg.NODE_MANAGER_BIN}")
        return False
    log_info("=== Node startup (node-manager -> worker-api) ===")
    with _popen_no_wait(
        [_cfg.NODE_MANAGER_BIN],
        cwd=_cfg.PROJECT_ROOT,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        shell=False,
    ) as proc:
        try:
            with open(_cfg.NODE_MANAGER_PID_FILE, "w", encoding="utf-8") as f:
                f.write(str(proc.pid))
        except OSError:
            pass
        log_info(f"Node-manager started (PID {proc.pid}); waiting for worker-api...")
        for _ in range(30):
            if proc.poll() is not None:
                log_error("Failed to start node-manager (process exited)")
                _log_proc_streams(proc)
                return False
            try:
                req = urllib.request.Request(
                    f"http://localhost:{_cfg.WORKER_PORT}/healthz"
                )
                with urllib.request.urlopen(req, timeout=2) as resp:
                    if resp.getcode() == 200:
                        log_info(
                            f"Worker API listening on http://localhost:{_cfg.WORKER_PORT}"
                        )
                        return True
            except (OSError, urllib.error.URLError):
                pass
            time.sleep(1)
        log_warn(f"Worker API did not respond on :{_cfg.WORKER_PORT} within 30s")
    return True


def _stop_all_step(step_name, func):
    """Run one teardown step; on exception log step, error, and traceback; do not raise."""
    _teardown_exceptions = (
        OSError,
        ValueError,
        RuntimeError,
        TypeError,
        KeyError,
        AttributeError,
        subprocess.SubprocessError,
        FileNotFoundError,
    )
    try:
        func()
    except _teardown_exceptions as e:
        log_warn(f"stop_all: {step_name} failed: {type(e).__name__}: {e}")
        traceback.print_exc(file=sys.stderr)
        sys.stderr.flush()


def stop_all(leave_ollama_running=False):
    """Kill node-manager, free worker port, compose down, rm containers.
    If leave_ollama_running, do not leave ollama stopped (restart it after down).
    Best-effort: never raises so caller can still report success (e.g. after E2E pass).
    """

    def kill_node_manager():
        if not os.path.isfile(_cfg.NODE_MANAGER_PID_FILE):
            return
        with open(_cfg.NODE_MANAGER_PID_FILE, encoding="utf-8") as f:
            pid = int(f.read().strip())
        try:
            os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            pass  # already exited
        try:
            os.remove(_cfg.NODE_MANAGER_PID_FILE)
        except OSError:
            pass

    def free_worker_port():
        subprocess.run(
            ["fuser", "-k", f"{_cfg.WORKER_PORT}/tcp"],
            capture_output=True, timeout=5, check=False, shell=False,
        )

    def free_worker_port_fallback():
        r = subprocess.run(
            ["lsof", "-t", "-i", f":{_cfg.WORKER_PORT}"],
            capture_output=True, text=True, timeout=5, check=False, shell=False,
        )
        for pid in (r.stdout or "").strip().split():
            os.kill(int(pid), signal.SIGTERM)

    def stop_managed_containers():
        """Stop and remove node-started managed service containers (e.g. cynodeai-managed-pma-*)."""
        if not _cfg.ensure_runtime():
            return
        r = subprocess.run(
            [_cfg.RUNTIME, "ps", "-aq", "--filter", "name=cynodeai-managed-"],
            capture_output=True, text=True, timeout=10, check=False, shell=False,
        )
        ids = (r.stdout or "").strip().split()
        if not ids:
            return
        for cid in ids:
            subprocess.run(
                [_cfg.RUNTIME, "stop", cid],
                capture_output=True, timeout=15, check=False, shell=False,
            )
            subprocess.run(
                [_cfg.RUNTIME, "rm", "-f", cid],
                capture_output=True, timeout=10, check=False, shell=False,
            )

    log_info("Stopping all services...")
    try:
        _stop_all_step("kill node-manager", kill_node_manager)
        _stop_all_step("stop managed containers (node PMA etc.)", stop_managed_containers)
        try:
            free_worker_port()
        except (FileNotFoundError, subprocess.TimeoutExpired) as e:
            log_warn(f"stop_all: free worker port (fuser) failed: {type(e).__name__}: {e}")
            _stop_all_step("free worker port (lsof)", free_worker_port_fallback)

        def compose_down():
            stop_orchestrator_stack_compose(leave_ollama_running=leave_ollama_running)

        _stop_all_step("compose down", compose_down)

        def rm_containers():
            if _cfg.ensure_runtime():
                subprocess.run(
                    [
                        _cfg.RUNTIME, "rm", "-f", "cynodeai-postgres",
                        _cfg.CONTROL_PLANE_CONTAINER_NAME, _cfg.USER_GATEWAY_CONTAINER_NAME,
                        _cfg.PMA_CONTAINER_NAME,
                        "cynodeai-mcp-gateway", "cynodeai-api-egress",
                    ],
                    capture_output=True, check=False, shell=False, timeout=30,
                )

        _stop_all_step("rm containers", rm_containers)
    except (OSError, ValueError, RuntimeError, TypeError, KeyError, AttributeError,
            subprocess.SubprocessError, FileNotFoundError) as e:
        log_warn(f"stop_all: teardown failed: {type(e).__name__}: {e}")
        traceback.print_exc(file=sys.stderr)
        sys.stderr.flush()
    return True


def build_e2e_images(force=False):
    """Build inference-proxy and cynode-sba images.

    If force is False, skip each image when its stamp hash matches and the image exists.
    Set force=True (e.g. from E2E_FORCE_REBUILD) to always rebuild.
    """
    if not _cfg.ensure_runtime():
        return False
    rt = _cfg.RUNTIME
    root = _cfg.PROJECT_ROOT
    cache_dir = os.environ.get(
        "E2E_IMAGE_CACHE_DIR",
        os.path.join(root, "tmp", "e2e-image-cache"),
    )
    # (containerfile rel, tag, extra_paths for context hash)
    images = [
        (
            "worker_node/cmd/inference-proxy/Containerfile",
            "cynodeai-inference-proxy:dev",
            ["go_shared_libs", "worker_node"],
        ),
        (
            "agents/cmd/cynode-sba/Containerfile",
            "cynodeai-cynode-sba:dev",
            ["go_shared_libs", "agents"],
        ),
    ]
    for dockerfile_rel, tag, extra_paths in images:
        if not os.path.isfile(os.path.join(root, dockerfile_rel)):
            log_error(f"Dockerfile not found: {dockerfile_rel}")
            return False
        stamp_name = tag.replace(":", "-") + ".stamp"
        stamp_path = os.path.join(cache_dir, stamp_name)
        if not force:
            current_hash = _build_cache.compute_context_hash(
                dockerfile_rel, root, extra_paths=extra_paths
            )
            stored = _build_cache.read_stamp(stamp_path)
            if stored == current_hash and _image_exists(tag):
                log_info(f"Image {tag} up to date.")
                continue
        log_info(f"Building {tag}...")
        if not run([rt, "build", "-f", dockerfile_rel, "-t", tag, "."], timeout=600):
            return False
        os.makedirs(cache_dir, exist_ok=True)
        _build_cache.write_stamp(
            stamp_path,
            _build_cache.compute_context_hash(
                dockerfile_rel, root, extra_paths=extra_paths
            ),
        )
    return True


def run_python_e2e(extra_env=None, e2e_args=None):
    """Run scripts/test_scripts/run_e2e.py (discovers e2e_*.py).

    e2e_args: optional list of extra args (e.g. ["--tags", "full_demo"]).
    """
    env = os.environ.copy()
    env["PYTHONPATH"] = _cfg.PROJECT_ROOT
    if extra_env:
        env.update(extra_env)
    run_e2e = os.path.join(
        _cfg.PROJECT_ROOT, "scripts", "test_scripts", "run_e2e.py"
    )
    if not os.path.isfile(run_e2e):
        log_error("scripts/test_scripts/run_e2e.py not found")
        return False
    cmd = [sys.executable, run_e2e]
    if e2e_args:
        cmd.extend(e2e_args)
    r = subprocess.run(
        cmd,
        cwd=_cfg.PROJECT_ROOT,
        env=env,
        check=False,
        shell=False,
    )
    return not r.returncode
