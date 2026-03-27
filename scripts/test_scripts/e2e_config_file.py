"""Minimal cynork config.yaml for E2E (gateway_url only; tokens stay in sidecar)."""

import os

from scripts.test_scripts import config


def ensure_minimal_gateway_config_yaml(config_path):
    """If ``config_path`` is missing, write ``gateway_url`` only. Return ``(ok, detail)``."""
    if not config_path:
        return False, "config path missing"
    if os.path.isfile(config_path):
        return True, "exists"
    d = os.path.dirname(os.path.abspath(config_path))
    if d:
        try:
            os.makedirs(d, mode=0o700, exist_ok=True)
        except OSError as exc:
            return False, f"create config dir: {exc}"
    try:
        with open(config_path, "w", encoding="utf-8") as f:
            f.write(f"gateway_url: {config.USER_API}\n")
    except OSError as exc:
        return False, f"write config: {exc}"
    return True, "written"
