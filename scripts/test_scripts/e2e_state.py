"""Shared state for E2E parity tests (one test per script).

Holds config dir/path and task/jwt IDs set by earlier tests and consumed by later ones.
"""

import os
import shutil
import sys
import tempfile

# Set by e2e_01_login; cleaned by e2e_09_logout
CONFIG_DIR = None
CONFIG_PATH = None

# Set by respective tests
TASK_ID = None
INF_TASK_ID = None
PROMPT_TASK_ID = None
SBA_TASK_ID = None
NODE_JWT = None


def init_config():
    """Create temp config dir and path. Idempotent."""
    mod = sys.modules[__name__]
    if mod.CONFIG_DIR is not None:
        return
    mod.CONFIG_DIR = tempfile.mkdtemp(prefix="cynodeai_e2e_")
    mod.CONFIG_PATH = os.path.join(mod.CONFIG_DIR, "config.yaml")


def cleanup_config():
    """Remove temp config dir."""
    mod = sys.modules[__name__]
    if mod.CONFIG_DIR and os.path.isdir(mod.CONFIG_DIR):
        shutil.rmtree(mod.CONFIG_DIR, ignore_errors=True)
    mod.CONFIG_DIR = None
    mod.CONFIG_PATH = None
