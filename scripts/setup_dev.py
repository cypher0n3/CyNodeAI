#!/usr/bin/env python3
# CyNodeAI development setup (Python). No bash dependency. E2E via test_scripts.
# Run from repo root: PYTHONPATH=. python scripts/setup_dev.py <command>

import argparse
import os
import sys
import time

from scripts import setup_dev_config
from scripts import setup_dev_impl

PROJECT_ROOT = setup_dev_config.PROJECT_ROOT


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
    print("  test-e2e        Run E2E via scripts/test_scripts/run_e2e.py")
    print("  full-demo       Full demo: build, E2E images, start, E2E, optionally stop")
    print("                  Use --stop-on-success to stop after E2E passes")
    print("  help            Show this message")
    print("")
    print("Environment (same as setup-dev.sh):")
    print("  POSTGRES_PORT, ORCHESTRATOR_PORT, CONTROL_PLANE_PORT, ADMIN_PASSWORD")
    print("  NODE_PSK, WORKER_PORT, E2E_FORCE_REBUILD, STOP_ON_SUCCESS_ENV")
    print("  INFERENCE_PROXY_IMAGE, OLLAMA_UPSTREAM_URL, CONTAINER_HOST_ALIAS")


def cmd_start(opts):
    """Build, start compose stack, wait for control-plane, start node."""
    if not setup_dev_impl.build_binaries():
        return False
    if not setup_dev_impl.start_orchestrator_stack_compose(extra_env=opts.extra_env):
        setup_dev_impl.stop_all()
        return False
    if not setup_dev_impl.wait_for_control_plane_listening():
        setup_dev_impl.stop_all()
        return False
    setup_dev_impl.log_info(
        f"Starting node (node-manager then worker-api on :{setup_dev_config.WORKER_PORT})..."
    )
    extra = getattr(opts, "extra_env", None)
    if not setup_dev_impl.start_node(extra_env=extra):
        setup_dev_impl.stop_all()
        return False
    setup_dev_impl.log_info(
        f"Services started: User API http://localhost:{setup_dev_config.ORCHESTRATOR_PORT} "
        f"Control-plane http://localhost:{setup_dev_config.CONTROL_PLANE_PORT} "
        f"Worker API http://localhost:{setup_dev_config.WORKER_PORT}"
    )
    setup_dev_impl.log_info("  Use 'test-e2e' to run E2E, 'stop' to stop.")
    return True


def cmd_full_demo(opts):
    """Build binaries, E2E images, orchestrator images; start stack; run E2E; optionally stop."""
    if not setup_dev_impl.build_binaries():
        return False
    if not setup_dev_impl.build_e2e_images():
        return False
    if not setup_dev_impl.build_orchestrator_compose_images():
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
    if not cmd_start(opts):
        return False
    time.sleep(3)
    e2e_env = {"INFERENCE_PROXY_IMAGE": opts.extra_env["INFERENCE_PROXY_IMAGE"]}
    ok = setup_dev_impl.run_python_e2e(extra_env=e2e_env)
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
    return setup_dev_impl.build_e2e_images()


def _run_stop():
    return setup_dev_impl.stop_all()


def _run_test_e2e():
    return setup_dev_impl.run_python_e2e()


def run_command(args):
    """Dispatch parsed args to the right handler. Returns exit code 0 or 1."""
    handlers = {
        "start-db": _run_start_db,
        "stop-db": _run_stop_db,
        "clean-db": _run_clean_db,
        "build": _run_build,
        "build-e2e-images": _run_build_e2e_images,
        "stop": _run_stop,
        "test-e2e": _run_test_e2e,
    }
    if args.command == "migrate":
        setup_dev_impl.log_info("Migrations run when control-plane starts.")
        return 0
    if args.command == "start":
        class Opts:
            extra_env = None
        return 0 if cmd_start(Opts()) else 1
    if args.command == "full-demo":
        class FullDemoOpts:
            stop_on_success = args.stop_on_success or bool(
                os.environ.get("STOP_ON_SUCCESS_ENV", "")
            )
            extra_env = None
        return 0 if cmd_full_demo(FullDemoOpts()) else 1
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
            "start", "stop", "test-e2e", "full-demo", "help",
        ],
        help="Command to run",
    )
    parser.add_argument(
        "--stop-on-success",
        action="store_true",
        help="For full-demo: stop services after E2E passes",
    )
    args = parser.parse_args()
    if args.command == "help":
        show_help()
        return 0
    return run_command(args)


if __name__ == "__main__":
    sys.exit(main())
