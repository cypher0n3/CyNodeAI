# E2E: auth negative - whoami without login fails.

import os
import shutil
import tempfile
import unittest

from scripts.test_scripts import config, helpers


class TestAuthNegative(unittest.TestCase):
    def test_whoami_without_login_fails(self):
        config_dir = tempfile.mkdtemp(prefix="cynodeai_e2e_noauth_")
        try:
            config_path = os.path.join(config_dir, "config.yaml")
            with open(config_path, "w", encoding="utf-8") as f:
                f.write("gateway_url: " + config.USER_API + "\n")
            ok, _, err = helpers.run_cynork(["auth", "whoami"], config_path)
            self.assertFalse(ok, "whoami without token should fail")
            self.assertTrue(
                "not logged in" in (err or "").lower() or "login" in (err or "").lower(),
                f"expected login error in stderr: {err}",
            )
        finally:
            shutil.rmtree(config_dir, ignore_errors=True)
