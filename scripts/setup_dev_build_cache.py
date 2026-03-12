# Build cache helpers for setup_dev: stamps and context hashes (incremental container builds).

import hashlib
import os


def read_stamp(path):
    """Read stamp file; return contents as string or None if missing/unreadable."""
    if not path or not os.path.isfile(path):
        return None
    try:
        with open(path, encoding="utf-8") as f:
            return f.read().strip()
    except OSError:
        return None


def write_stamp(path, value):
    """Write value to stamp file; create parent dirs if needed."""
    dirpath = os.path.dirname(path)
    if dirpath:
        os.makedirs(dirpath, exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(value)


def _path_fingerprint(root, rel_paths):
    """Hash of (path, mtime, size) for all files under rel_paths under root."""
    h = hashlib.sha256()
    entries = []
    for rel in rel_paths:
        abspath = os.path.join(root, rel)
        if not os.path.isdir(abspath) and not os.path.isfile(abspath):
            entries.append((rel, 0, 0))
            continue
        for dirname, _subdirs, filenames in os.walk(abspath, topdown=True):
            for name in sorted(filenames):
                p = os.path.join(dirname, name)
                try:
                    st = os.stat(p)
                    r = os.path.relpath(p, root)
                    entries.append((r.replace("\\", "/"), st.st_mtime_ns, st.st_size))
                except OSError:
                    pass
    for t in sorted(entries):
        h.update(repr(t).encode("utf-8"))
    return h.hexdigest()


def compute_context_hash(containerfile_path, context_root, extra_paths=None):
    """Hash of containerfile content and context paths (incremental image build).

    containerfile_path: path relative to context_root.
    context_root: repo root (build context).
    extra_paths: relative dirs to include (e.g. ["go_shared_libs", "worker_node"]).
    """
    h = hashlib.sha256()
    abspath = os.path.join(context_root, containerfile_path)
    if os.path.isfile(abspath):
        with open(abspath, "rb") as f:
            h.update(f.read())
    if extra_paths:
        h.update(_path_fingerprint(context_root, extra_paths).encode("utf-8"))
    return h.hexdigest()


def compute_compose_images_hash(compose_path, root):
    """Compute a hash for orchestrator compose images (control-plane, user-gateway, cynode-pma).

    Includes: compose file content, the three service Containerfiles, and dirs they use
    (orchestrator, agents, go_shared_libs).
    """
    h = hashlib.sha256()
    if os.path.isfile(compose_path):
        with open(compose_path, "rb") as f:
            h.update(f.read())
    # Compose build context is repo root; services use these Containerfiles (relative to root).
    containerfiles = [
        "orchestrator/cmd/control-plane/Containerfile",
        "orchestrator/cmd/user-gateway/Containerfile",
        "agents/cmd/cynode-pma/Containerfile",
    ]
    for cf in containerfiles:
        abspath = os.path.join(root, cf)
        if os.path.isfile(abspath):
            with open(abspath, "rb") as f:
                h.update(f.read())
    dir_fp = _path_fingerprint(root, ["orchestrator", "agents", "go_shared_libs"])
    h.update(dir_fp.encode("utf-8"))
    return h.hexdigest()


def rebuild_inference_proxy(force, root, runtime, cb, no_cache=False):
    """Build inference-proxy. cb: run, image_exists, log_info, log_error. no_cache: --no-cache."""
    run_fn = cb["run"]
    image_exists_fn = cb["image_exists"]
    log_info_fn = cb["log_info"]
    log_error_fn = cb["log_error"]
    dockerfile_rel = "worker_node/cmd/inference-proxy/Containerfile"
    tag = "cynodeai-inference-proxy:dev"
    if not os.path.isfile(os.path.join(root, dockerfile_rel)):
        log_error_fn(f"Dockerfile not found: {dockerfile_rel}")
        return False
    cache_dir = os.environ.get("E2E_IMAGE_CACHE_DIR", os.path.join(root, "tmp", "e2e-image-cache"))
    stamp_path = os.path.join(cache_dir, tag.replace(":", "-") + ".stamp")
    current_hash = compute_context_hash(
        dockerfile_rel, root, extra_paths=["go_shared_libs", "worker_node"]
    )
    if not force and read_stamp(stamp_path) == current_hash and image_exists_fn(tag):
        log_info_fn(f"Image {tag} up to date.")
        return True
    log_info_fn(f"Building {tag}...")
    no_cache_args = ["--no-cache"] if no_cache else []
    build_cmd = [runtime, "build"] + no_cache_args + ["-f", dockerfile_rel, "-t", tag, "."]
    if not run_fn(build_cmd, timeout=600):
        return False
    os.makedirs(cache_dir, exist_ok=True)
    write_stamp(stamp_path, current_hash)
    return True


def rebuild_cynode_sba(force, root, runtime, cb, no_cache=False):
    """Build cynode-sba. cb: run, image_exists, log_info, log_error. no_cache: add --no-cache."""
    run_fn = cb["run"]
    image_exists_fn = cb["image_exists"]
    log_info_fn = cb["log_info"]
    log_error_fn = cb["log_error"]
    dockerfile_rel = "agents/cmd/cynode-sba/Containerfile"
    tag = "cynodeai-cynode-sba:dev"
    if not os.path.isfile(os.path.join(root, dockerfile_rel)):
        log_error_fn(f"Dockerfile not found: {dockerfile_rel}")
        return False
    cache_dir = os.environ.get("E2E_IMAGE_CACHE_DIR", os.path.join(root, "tmp", "e2e-image-cache"))
    stamp_path = os.path.join(cache_dir, tag.replace(":", "-") + ".stamp")
    current_hash = compute_context_hash(
        dockerfile_rel, root, extra_paths=["go_shared_libs", "agents"]
    )
    if not force and read_stamp(stamp_path) == current_hash and image_exists_fn(tag):
        log_info_fn(f"Image {tag} up to date.")
        return True
    log_info_fn(f"Building {tag}...")
    no_cache_args = ["--no-cache"] if no_cache else []
    build_cmd = [runtime, "build"] + no_cache_args + ["-f", dockerfile_rel, "-t", tag, "."]
    if not run_fn(build_cmd, timeout=600):
        return False
    os.makedirs(cache_dir, exist_ok=True)
    write_stamp(stamp_path, current_hash)
    return True
