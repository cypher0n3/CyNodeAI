# E2E: When node state dir is available, secure store token files have envelope structure
# and do not contain plaintext credentials. Skip when NODE_STATE_DIR unset.

import json
import os
import unittest

from scripts.test_scripts import config


class TestSecureStoreEnvelopeStructure(unittest.TestCase):
    """E2E: Assert secure store token files (when state dir available) are encrypted envelopes."""

    tags = ["suite_worker_node"]

    def test_agent_token_files_are_envelopes_and_not_plaintext(self):
        """State dir secrets/agent_tokens files are valid envelopes, no plaintext token."""
        if not config.NODE_STATE_DIR:
            self.skipTest("NODE_STATE_DIR not set")
        secrets_dir = os.path.join(
            config.NODE_STATE_DIR, "secrets", "agent_tokens"
        )
        if not os.path.isdir(secrets_dir):
            self.skipTest("secrets/agent_tokens dir not present (no managed services)")
        token = (config.WORKER_API_BEARER_TOKEN or "").strip()
        self.assertTrue(token, "WORKER_API_BEARER_TOKEN required to assert not in files")
        found = 0
        for name in os.listdir(secrets_dir):
            if not name.endswith(".json.enc"):
                continue
            path = os.path.join(secrets_dir, name)
            if not os.path.isfile(path):
                continue
            found += 1
            with open(path, encoding="utf-8") as f:
                raw = f.read()
            self.assertNotIn(
                token,
                raw,
                f"{name} must not contain bearer token in plaintext",
            )
            data = json.loads(raw)
            self.assertIn("version", data, f"{name} must have version")
            self.assertIn("algorithm", data, f"{name} must have algorithm")
            self.assertIn("payload_b64", data, f"{name} must have payload_b64")
            self.assertTrue(
                isinstance(data.get("payload_b64"), str) and len(data["payload_b64"]) > 0,
                f"{name} payload_b64 must be non-empty string",
            )
        if not found:
            self.skipTest("no .json.enc files in secrets/agent_tokens")
