#!/usr/bin/env python3
# CyNodeAI development setup (Python). No bash dependency. E2E via test_scripts.
# Run from repo root: PYTHONPATH=. python scripts/setup_dev.py <command>

import argparse
import os
import sys

from scripts import setup_dev_config
from scripts import setup_dev_impl

PROJECT_ROOT = setup_dev_config.PROJECT_ROOT


def _default_inference_env():
    """Return default inference env for node startup and e2e."""
    setup_dev_config.ensure_runtime()
    host = setup_dev_config.CONTAINER_HOST_ALIAS or "host.containers.internal"
    return {
        "INFERENCE_PROXY_IMAGE": os.environ.get(
            "INFERENCE_PROXY_IMAGE", "cynodeai-inference-proxy:dev"
        ),
        "OLLAMA_UPSTREAM_URL": os.environ.get(
            "OLLAMA_UPSTREAM_URL", f"http://{host}:11434"
        ),
    }


def show_help():
    """Print usage (parity with setup-dev.sh help)."""
    print("CyNodeAI Development Setup (Python)")
    print("")
    print("Usage: PYTHONPATH=. python scripts/setup_dev.py <command> [OPTIONS]")
    print("")
    print("Commands:")
    print("  start-db        Start PostgreSQL container (standalone)")
    print("  stop-db         Stop PostgreSQL container")
    print("  clean-db        Stop and remove PostgreSQL container and volume")
    print("  migrate         (No-op; migrations run when control-plane starts)")
    print("  build           Build orchestrator and node binaries (just build)")
    print("  build-e2e-images Build inference-proxy and cynode-sba images")
    print("  start           Start all services (compose stack + node)")
    print("  stop            Stop all services")
    print("  restart         Stop then start all services")
    print("  clean           Stop all services and remove postgres container/volume")
    print("  test-e2e        Run E2E via scripts/test_scripts/run_e2e.py")
    print("  full-demo       Full demo: build, E2E images, start, E2E, optionally stop")
    print("                  Use --stop-on-success to stop after E2E passes")
    print("  help            Show this message")
    print("")
    print("Startup (start / full-demo / restart): default is prescribed sequence.")
    print("  --ollama-in-stack   Bypass: run OLLAMA in orchestrator compose.")
    print("  --pma-via-compose   Bypass: start PMA via compose after node.")
    print("  SETUP_DEV_OLLAMA_IN_STACK=1, SETUP_DEV_PMA_VIA_COMPOSE=1  Same as flags.")
    print("")
    print("Environment (same as setup-dev.sh):")
    print("  POSTGRES_PORT, ORCHESTRATOR_PORT, CONTROL_PLANE_PORT, ADMIN_PASSWORD")
    print("  NODE_PSK, WORKER_PORT, E2E_FORCE_REBUILD, SETUP_DEV_FORCE_BUILD, STOP_ON_SUCCESS_ENV")
    print("  INFERENCE_PROXY_IMAGE, OLLAMA_UPSTREAM_URL, CONTAINER_HOST_ALIAS")


def _resolve_bypass(_name, arg_value, env_key):
    """True if bypass is enabled via arg or env (e.g. SETUP_DEV_OLLAMA_IN_STACK=1)."""
    if arg_value:
        return True
    return os.environ.get(env_key, "").strip() == "1"


def cmd_start(opts):
    """Run full startup sequence: build binaries, compose up, start node. PMA only when bypass.

    Order (normal startup for start / full-demo / restart):
    1. build_binaries() - just build-dev
    2. start_orchestrator_stack_compose() - postgres, control-plane, user-gateway, optional profile
    3. start_node() - node-manager binary (polls orchestrator /readyz itself per worker_node spec),
       then worker-api on WORKER_PORT; script waits for worker-api healthz
    4. (bypass only) If --pma-via-compose: script starts PMA via compose and waits for healthz.
       Prescribed: script does not start or wait for PMA; worker node starts PMA when orchestrator
       directs (orchestrator_bootstrap.md, worker_node managed services).
    """
    opts.extra_env = getattr(opts, "extra_env", None) or {}
    defaults = _default_inference_env()
    for k, v in defaults.items():
        opts.extra_env.setdefault(k, v)
    ollama_in_stack = getattr(opts, "ollama_in_stack", False)
    pma_via_compose = getattr(opts, "pma_via_compose", False)
    if not setup_dev_impl.build_binaries():
        return False
    # Prescribed sequence (docs/tech_specs/orchestrator_bootstrap.md): do not build compose
    # images here; start uses existing images. full-demo builds them before calling cmd_start.
    if not setup_dev_impl.start_orchestrator_stack_compose(
        extra_env=opts.extra_env, ollama_in_stack=ollama_in_stack
    ):
        setup_dev_impl.stop_all()
        return False
    # Node-manager polls orchestrator /readyz (worker_node startup); script does not wait.
    port = setup_dev_config.WORKER_PORT
    setup_dev_impl.log_info(
        f"Starting node (node-manager polls control-plane, worker-api :{port})..."
    )
    extra = getattr(opts, "extra_env", None)
    if not setup_dev_impl.start_node(extra_env=extra):
        setup_dev_impl.stop_all()
        return False
    # PMA: prescribed path = worker node starts PMA when orchestrator directs; script does nothing.
    # Bypass (--pma-via-compose) = script starts PMA via compose and waits for healthz.
    if pma_via_compose:
        if not setup_dev_impl.start_pma_after_inference_path(
            extra_env=extra, pma_via_compose=True
        ):
            setup_dev_impl.stop_all()
            return False
    else:
        setup_dev_impl.log_info(
            "Prescribed: PMA by worker when orchestrator directs; script does not start/wait."
        )
    setup_dev_impl.log_info(
        f"Services started: User API http://localhost:{setup_dev_config.ORCHESTRATOR_PORT} "
        f"Control-plane http://localhost:{setup_dev_config.CONTROL_PLANE_PORT} "
        f"Worker API http://localhost:{setup_dev_config.WORKER_PORT}"
    )
    setup_dev_impl.log_info("  Use 'test-e2e' to run E2E, 'stop' to stop.")
    return True


def _force_rebuild():
    """True if E2E_FORCE_REBUILD or SETUP_DEV_FORCE_BUILD is set (skip incremental cache)."""
    return bool(
        os.environ.get("E2E_FORCE_REBUILD") or os.environ.get("SETUP_DEV_FORCE_BUILD")
    )


def cmd_full_demo(opts):
    """Build binaries, E2E images, orchestrator images; start stack; run E2E; optionally stop."""
    if not setup_dev_impl.build_binaries():
        return False
    if not setup_dev_impl.build_e2e_images(force=_force_rebuild()):
        return False
    if not setup_dev_impl.build_orchestrator_compose_images(force=_force_rebuild()):
        return False
    setup_dev_config.ensure_runtime()
    host = setup_dev_config.CONTAINER_HOST_ALIAS or "host.containers.internal"
    opts.extra_env = opts.extra_env or {}
    opts.extra_env.setdefault(
        "INFERENCE_PROXY_IMAGE",
        os.environ.get("INFERENCE_PROXY_IMAGE", "cynodeai-inference-proxy:dev"),
    )
    opts.extra_env.setdefault(
        "OLLAMA_UPSTREAM_URL",
        os.environ.get("OLLAMA_UPSTREAM_URL", f"http://{host}:11434"),
    )
    # So the worker reports a PMA endpoint the gateway (in container) can reach (REQ-ORCHES-0162).
    opts.extra_env.setdefault(
        "PMA_ADVERTISED_URL",
        f"http://{host}:{setup_dev_config.PMA_PORT}",
    )
    if not cmd_start(opts):
        return False
    # Let node register, apply config, send ack so it is dispatchable before E2E creates tasks.
    if not setup_dev_impl.wait_for_orchestrator_readyz(timeout_sec=120):
        return False
    e2e_env = {"INFERENCE_PROXY_IMAGE": opts.extra_env["INFERENCE_PROXY_IMAGE"]}
    # e2e_122 asserts on node state dir; must match node WORKER_API_STATE_DIR.
    e2e_env["NODE_STATE_DIR"] = setup_dev_config.NODE_STATE_DIR
    ok = setup_dev_impl.run_python_e2e(
        extra_env=e2e_env, e2e_args=["--tags", "full_demo"]
    )
    if opts.stop_on_success and ok:
        setup_dev_impl.log_info("Demo completed! Stopping services (--stop-on-success).")
        leave_ollama = setup_dev_impl.get_ollama_was_running_before()
        setup_dev_impl.stop_all(leave_ollama_running=leave_ollama)
    elif ok:
        setup_dev_impl.log_info("Demo completed! Services still running. Use 'stop' to stop.")
    else:
        setup_dev_impl.stop_all()
    return ok


def _run_start_db():
    return setup_dev_impl.start_postgres()


def _run_stop_db():
    return setup_dev_impl.stop_postgres()


def _run_clean_db():
    return setup_dev_impl.clean_postgres()


def _run_build():
    return setup_dev_impl.build_binaries()


def _run_build_e2e_images():
    return setup_dev_impl.build_e2e_images(force=_force_rebuild())


def _run_stop():
    return setup_dev_impl.stop_all()


def _run_test_e2e():
    return setup_dev_impl.run_python_e2e(extra_env=_default_inference_env())


def _run_restart(args):
    """Stop all then start (parity with dev-setup.sh restart)."""
    setup_dev_impl.stop_all()
    setup_dev_impl.wait_for_control_plane_stopped(timeout_sec=15)

    class Opts:
        extra_env = None
        ollama_in_stack = _resolve_bypass(
            "ollama_in_stack", getattr(args, "ollama_in_stack", False),
            "SETUP_DEV_OLLAMA_IN_STACK"
        )
        pma_via_compose = _resolve_bypass(
            "pma_via_compose", getattr(args, "pma_via_compose", False),
            "SETUP_DEV_PMA_VIA_COMPOSE"
        )
    return cmd_start(Opts())


def _run_clean():
    """Stop all services and remove postgres container/volume (parity with dev-setup.sh clean)."""
    setup_dev_impl.stop_all()
    return setup_dev_impl.clean_postgres()


def run_command(args):
    """Dispatch parsed args to the right handler. Returns exit code 0 or 1."""
    handlers = {
        "start-db": _run_start_db,
        "stop-db": _run_stop_db,
        "clean-db": _run_clean_db,
        "build": _run_build,
        "build-e2e-images": _run_build_e2e_images,
        "stop": _run_stop,
        "clean": _run_clean,
        "test-e2e": _run_test_e2e,
    }
    if args.command == "migrate":
        setup_dev_impl.log_info("Migrations run when control-plane starts.")
        return 0
    if args.command == "start":
        class Opts:
            extra_env = None
            ollama_in_stack = _resolve_bypass(
                "ollama_in_stack", getattr(args, "ollama_in_stack", False),
                "SETUP_DEV_OLLAMA_IN_STACK"
            )
            pma_via_compose = _resolve_bypass(
                "pma_via_compose", getattr(args, "pma_via_compose", False),
                "SETUP_DEV_PMA_VIA_COMPOSE"
            )
        return 0 if cmd_start(Opts()) else 1
    if args.command == "full-demo":
        class FullDemoOpts:
            stop_on_success = args.stop_on_success or bool(
                os.environ.get("STOP_ON_SUCCESS_ENV", "")
            )
            extra_env = None
            ollama_in_stack = _resolve_bypass(
                "ollama_in_stack", getattr(args, "ollama_in_stack", False),
                "SETUP_DEV_OLLAMA_IN_STACK"
            )
            pma_via_compose = _resolve_bypass(
                "pma_via_compose", getattr(args, "pma_via_compose", False),
                "SETUP_DEV_PMA_VIA_COMPOSE"
            )
        return 0 if cmd_full_demo(FullDemoOpts()) else 1
    if args.command == "restart":
        return 0 if _run_restart(args) else 1
    if args.command in handlers:
        return 0 if handlers[args.command]() else 1
    return 1


def main():
    parser = argparse.ArgumentParser(
        description="CyNodeAI dev setup (Python). No bash. E2E via scripts/test_scripts.",
        epilog="Env vars: see 'help'.",
    )
    parser.add_argument(
        "command",
        choices=[
            "start-db", "stop-db", "clean-db", "migrate", "build", "build-e2e-images",
            "start", "stop", "restart", "clean", "test-e2e", "full-demo", "help",
        ],
        help="Command to run",
    )
    parser.add_argument(
        "--stop-on-success",
        action="store_true",
        help="For full-demo: stop services after E2E passes",
    )
    parser.add_argument(
        "--ollama-in-stack",
        action="store_true",
        help="Bypass: run OLLAMA in orchestrator compose (prescribed: node starts inference)",
    )
    parser.add_argument(
        "--pma-via-compose",
        action="store_true",
        help="Bypass: start PMA via compose after node (prescribed: orchestrator instructs worker)",
    )
    args = parser.parse_args()
    if args.command == "help":
        show_help()
        return 0
    return run_command(args)


if __name__ == "__main__":
    sys.exit(main())
