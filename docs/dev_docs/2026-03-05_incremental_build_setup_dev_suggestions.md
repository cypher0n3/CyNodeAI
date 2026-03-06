# Incremental Build for `setup-dev` `full-demo` (Python)

## Overview

`just setup-dev full-demo --stop-on-success` currently rebuilds all containers every time even when nothing changed.
Go binaries are fine to rebuild every run (`just build-dev` is fast); the goal is to make **container** builds incremental only.
Changes only in Python (`scripts/setup_dev_impl.py`, optional helper in `scripts/`).
No changes to the justfile.

## E2E Container Images (`build_e2e_images`)

Today this step always runs `podman`/`docker` build for `cynodeai-inference-proxy:dev` and `cynodeai-cynode-sba:dev`.

- Cache dir: `os.environ.get("E2E_IMAGE_CACHE_DIR", os.path.join(PROJECT_ROOT, "tmp", "e2e-image-cache"))`.
  Create if missing.
- Per-image stamp: for each `(Containerfile, tag)` (e.g. `worker_node/cmd/inference-proxy/Containerfile` -> `cynodeai-inference-proxy:dev`):
  - Compute a build-context hash: e.g. hash of (Containerfile content + sorted list of relative paths and mtimes under the build context, e.g. `worker_node/` or repo root as used by the current build).
    Include the binary path the image expects (e.g. `worker_node/bin/inference-proxy`) if it is baked in.
  - Stamp file: `{cache_dir}/{sanitized_tag}.stamp` containing the hash.
  - If `E2E_FORCE_REBUILD` is set, or stamp is missing, or stored hash != current hash, or image does not exist: run build; then write stamp.
  - If image exists and stamp matches current hash: skip and log "Image {tag} up to date."
- Force: respect `E2E_FORCE_REBUILD` (already documented in help) to ignore cache and rebuild.

Build context for both images is repo root (`.`).
Hash the Containerfile(s) plus any paths COPY'd or ADD'ed (or conservatively: go_shared_libs, worker_node, agents, and the specific bin/ outputs they use).
Keep hashing cheap: prefer mtime + size for large trees, or content hash only for Containerfile and go.mod/sum.

## Orchestrator Compose Images (`build_orchestrator_compose_images`)

Today this step always runs `compose build control-plane user-gateway cynode-pma`.

- Inputs: compose file mtime/content, and for each service (control-plane, user-gateway, cynode-pma): Dockerfile/Containerfile path, its content, and the build context (e.g. `orchestrator/` or repo root) - key files or mtimes that affect the image.
- Stamp: one stamp file under e.g. `tmp/setup_dev_compose_images.stamp` storing a hash of the above.
  If stamp exists and equals current hash, and the three images exist locally (e.g. `podman images -q cynodeai-control-plane:dev` and similarly for user-gateway and cynode-pma): skip `compose build` and log "Orchestrator compose images up to date."
- Force: same `E2E_FORCE_REBUILD` or `SETUP_DEV_FORCE_BUILD`: delete stamp and rebuild.

Compose build uses Dockerfiles referenced in `orchestrator/docker-compose.yml`.
Parse the compose file (or run from repo root and use compose's context) to get Dockerfile path and context per service; fingerprint those paths + the binaries they copy (e.g. orchestrator/bin, agents/bin).

## Flow in `setup_dev_impl` and `setup_dev.py`

- full-demo: keep calling `build_binaries()` every time (no cache); call `build_e2e_images(force=False)` and `build_orchestrator_compose_images(force=False)` with cache.
  When `E2E_FORCE_REBUILD` (or `SETUP_DEV_FORCE_BUILD`) is set, pass `force=True` so both image steps skip cache.
- Explicit commands `build-e2e-images`: respect the same cache so `just setup-dev build-e2e-images` is incremental.
- Log clearly when a container step is skipped ("... up to date") so users see that incremental behavior is working.

## Optional Shared Fingerprint Helper

- Add e.g. `scripts/setup_dev_build_cache.py` with:
  - `compute_context_hash(containerfile_path, context_root, extra_paths=None) -> str`
  - `read_stamp(path) -> str | None` / `write_stamp(path, value)`
- Keep stamp files and cache dir under `tmp/` (or `E2E_IMAGE_CACHE_DIR` for E2E images) so they remain local and can be gitignored.

## Summary

- **Go binaries:** unchanged; keep running `just build-dev` every time.
- **E2E images:** skip when per-image context hash matches and image exists; cache in `tmp/e2e-image-cache/` (or `E2E_IMAGE_CACHE_DIR`); force with `E2E_FORCE_REBUILD`.
- **Compose images:** skip when composite hash matches and all three images exist; cache in `tmp/setup_dev_compose_images.stamp`; force with `E2E_FORCE_REBUILD` or `SETUP_DEV_FORCE_BUILD`.

This aligns with the mvp_plan mention of "conditional container image rebuild: build-context hash cached under tmp/e2e-image-cache" and the documented `E2E_FORCE_REBUILD` / `E2E_IMAGE_CACHE_DIR`, and keeps all logic in Python without changing the justfile.
