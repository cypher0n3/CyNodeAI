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
