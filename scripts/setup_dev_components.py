# Per-component start/stop/restart/rebuild. Imported by setup_dev_impl to stay under line limit.

import os
import time

import scripts.setup_dev_build_cache as _build_cache
import scripts.setup_dev_config as _cfg
import scripts.setup_dev_impl as _impl


def _compose_up(service_names, profiles=None, extra_env=None):
    """Run compose up -d for given service(s). Returns True on success."""
    if not _cfg.ensure_runtime() or not os.path.isfile(_cfg.COMPOSE_FILE):
        return False
    args = [_cfg.RUNTIME, "compose", "-f", _cfg.COMPOSE_FILE]
    if profiles:
        for p in profiles:
            args.extend(["--profile", p])
    args.extend(["up", "-d"] + list(service_names))
    env = _cfg.compose_env()
    if extra_env:
        env.update(extra_env)
    return _impl.run(args, env=env, cwd=_cfg.PROJECT_ROOT, timeout=120)


def _compose_stop(service_names, profiles=None):
    """Run compose stop for given service(s). Returns True on success."""
    if not _cfg.ensure_runtime() or not os.path.isfile(_cfg.COMPOSE_FILE):
        return False
    args = [_cfg.RUNTIME, "compose", "-f", _cfg.COMPOSE_FILE]
    if profiles:
        for p in profiles:
            args.extend(["--profile", p])
    args.extend(["stop"] + list(service_names))
    return _impl.run(args, env=_cfg.compose_env(), cwd=_cfg.PROJECT_ROOT, timeout=60)


_COMPOSE_SERVICE_PROFILES = {
    "postgres": (["optional"], "postgres"),
    "control-plane": (["optional"], "control-plane"),
    "user-gateway": (["optional"], "user-gateway"),
    "mcp-gateway": (["optional"], "mcp-gateway"),
    "api-egress": (["optional"], "api-egress"),
    "ollama": (["ollama"], "ollama"),
}


def component_start(name, extra_env=None):
    """Start a single component. Returns True on success."""
    if name == "node-manager":
        return _impl.start_node(extra_env=extra_env)
    if name in _COMPOSE_SERVICE_PROFILES:
        profiles, svc = _COMPOSE_SERVICE_PROFILES[name]
        return _compose_up([svc], profiles=profiles, extra_env=extra_env)
    _impl.log_error(f"Unknown component: {name}")
    return False


def component_stop(name):
    """Stop a single component. Returns True on success."""
    if name == "node-manager":
        return _impl.stop_node(stop_managed_containers=True)
    if name in _COMPOSE_SERVICE_PROFILES:
        profiles, svc = _COMPOSE_SERVICE_PROFILES[name]
        return _compose_stop([svc], profiles=profiles)
    _impl.log_error(f"Unknown component: {name}")
    return False


def component_restart(name, extra_env=None):
    """Stop then start a single component. Returns True on success."""
    if not component_stop(name):
        return False
    if name in _COMPOSE_SERVICE_PROFILES:
        time.sleep(2)
    return component_start(name, extra_env=extra_env)


def component_rebuild(name, force=False, no_cache=False):
    """Rebuild image or binary for a component. no_cache: pass --no-cache to build. Returns True."""
    if not _cfg.ensure_runtime():
        return False
    rt = _cfg.RUNTIME
    root = _cfg.PROJECT_ROOT
    run = _impl.run
    log_error = _impl.log_error
    build_extra = ["--no-cache"] if no_cache else []

    if name == "control-plane":
        return run(
            [rt, "compose", "-f", _cfg.COMPOSE_FILE, "build"] + build_extra + ["control-plane"],
            cwd=root, timeout=600,
        )
    if name == "user-gateway":
        return run(
            [rt, "compose", "-f", _cfg.COMPOSE_FILE, "build"] + build_extra + ["user-gateway"],
            cwd=root, timeout=600,
        )
    if name == "mcp-gateway":
        return run(
            [rt, "compose", "-f", _cfg.COMPOSE_FILE, "build"] + build_extra + ["mcp-gateway"],
            cwd=root, timeout=600,
        )
    if name == "api-egress":
        return run(
            [rt, "compose", "-f", _cfg.COMPOSE_FILE, "build"] + build_extra + ["api-egress"],
            cwd=root, timeout=600,
        )
    if name == "cynode-pma":
        pma_dockerfile = os.path.join(root, "agents", "cmd", "cynode-pma", "Containerfile")
        if not os.path.isfile(pma_dockerfile):
            log_error(f"Containerfile not found: {pma_dockerfile}")
            return False
        cmd = [rt, "build"] + build_extra + [
            "-f", pma_dockerfile, "-t", "cynodeai-cynode-pma:dev", root,
        ]
        return run(cmd, cwd=root, timeout=600)
    if name == "worker-api":
        if no_cache:
            cfile = "worker_node/cmd/worker-api/Containerfile"
            cmd = [rt, "build"] + build_extra + ["-f", cfile, "-t", "cynodeai-worker-api", "."]
            return run(cmd, cwd=root, timeout=600)
        return run(["just", "build-worker-api-image"], cwd=root, timeout=600)
    if name == "inference-proxy":
        cb = {
            "run": run, "image_exists": _impl.image_exists,
            "log_info": _impl.log_info, "log_error": log_error,
        }
        return _build_cache.rebuild_inference_proxy(force, root, rt, cb, no_cache=no_cache)
    if name == "cynode-sba":
        cb = {
            "run": run, "image_exists": _impl.image_exists,
            "log_info": _impl.log_info, "log_error": log_error,
        }
        return _build_cache.rebuild_cynode_sba(force, root, rt, cb, no_cache=no_cache)
    if name == "node-manager":
        return run(["just", "build-node-manager-dev"], cwd=root, timeout=300)

    log_error(f"Unknown or non-rebuildable component: {name}")
    return False
