# Shared state for E2E parity tests (one test per script). Config dir and task/jwt IDs.

import os
import shutil
import tempfile

# Set by e2e_01_login; cleaned by e2e_09_logout
config_dir = None
config_path = None

# Set by respective tests
task_id = None
inf_task_id = None
prompt_task_id = None
sba_task_id = None
node_jwt = None


def init_config():
    """Create temp config dir and path. Idempotent."""
    global config_dir, config_path
    if config_dir is not None:
        return
    config_dir = tempfile.mkdtemp(prefix="cynodeai_e2e_")
    config_path = os.path.join(config_dir, "config.yaml")


def cleanup_config():
    """Remove temp config dir."""
    global config_dir, config_path
    if config_dir and os.path.isdir(config_dir):
        shutil.rmtree(config_dir, ignore_errors=True)
    config_dir = None
    config_path = None
