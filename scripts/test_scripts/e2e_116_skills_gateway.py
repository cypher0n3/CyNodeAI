# E2E: skills list, load, get, delete via gateway (requires auth from e2e_020).
# Traces: REQ-CLIENT-0146; REQ-SKILLS-0106, 0115; skills CRUD via gateway.

import os
import tempfile
import unittest

from scripts.test_scripts import helpers
import scripts.test_scripts.e2e_state as state


class TestSkillsGateway(unittest.TestCase):
    """E2E: cynork skills list, load, get, delete against user-gateway."""

    tags = ["suite_orchestrator", "full_demo"]

    def test_skills_list_load_get_delete(self):
        """Assert skills list returns JSON; load a skill; get by id; delete."""
        if not state.CONFIG_PATH or not os.path.isfile(state.CONFIG_PATH):
            self.skipTest("config not set (run e2e_020 auth login first)")
        ok, out, _ = helpers.run_cynork(
            ["skills", "list", "-o", "json"], state.CONFIG_PATH
        )
        self.assertTrue(ok, f"skills list failed: {out}")
        data = helpers.parse_json_safe(out)
        self.assertIsNotNone(data, "skills list output not JSON")
        self.assertIn("skills", data)
        self.assertIsInstance(data["skills"], list)

        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".md", delete=False
        ) as f:
            f.write("# E2E skill content\n")
            path = f.name
        try:
            ok, out, _ = helpers.run_cynork(
                ["skills", "load", path, "-o", "json"], state.CONFIG_PATH
            )
            self.assertTrue(ok, f"skills load failed: {out}")
            load_data = helpers.parse_json_safe(out)
            self.assertIsNotNone(load_data, "skills load output not JSON")
            skill_id = load_data.get("id")
            self.assertIsNotNone(skill_id, "skills load response missing id")
        finally:
            try:
                os.unlink(path)
            except OSError:
                pass

        ok, out, _ = helpers.run_cynork(
            ["skills", "get", skill_id, "-o", "json"], state.CONFIG_PATH
        )
        self.assertTrue(ok, f"skills get failed: {out}")
        get_data = helpers.parse_json_safe(out)
        self.assertIsNotNone(get_data)
        self.assertEqual(get_data.get("id"), skill_id)
        self.assertIn("content", get_data)

        ok, _, _ = helpers.run_cynork(
            ["skills", "delete", skill_id], state.CONFIG_PATH
        )
        self.assertTrue(ok, "skills delete failed")
