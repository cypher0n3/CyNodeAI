# E2E: Ollama container image tag matches expected GPU variant.
# After stack bring-up, the node-manager starts Ollama with the variant derived from
# GPU capability (cuda vs rocm). This test independently detects GPU (nvidia-smi,
# rocm-smi), sums VRAM per vendor, and asserts the running Ollama image tag matches.
# Traces: REQ-WORKER-0253 (orchestrator-directed variant); REQ-ORCHES-0149.

import unittest

from scripts.test_scripts import config, helpers
from scripts.test_scripts import gpu_variant


def _image_tag(image: str | None) -> str | None:
    """Extract tag from image string (e.g. ollama/ollama:rocm -> rocm)."""
    if not image or ":" not in image:
        return None
    return image.rsplit(":", 1)[-1]


def _tag_matches_expected(actual_tag: str | None, expected_variant: str) -> bool:
    """Ollama: rocm has :rocm tag; cuda uses default image (no tag or :latest)."""
    if expected_variant == "rocm":
        return actual_tag == "rocm"
    if expected_variant == "cuda":
        return actual_tag in (None, "", "latest")
    return False


class TestGPUVariantOllama(unittest.TestCase):
    """E2E: Ollama container image tag matches expected variant for host GPU."""

    tags = ["suite_worker_node", "gpu_variant", "inference"]
    prereqs = []

    def test_ollama_image_tag_matches_expected_gpu_variant(self):
        """Assert Ollama container image tag matches variant from independent GPU detection."""
        expected = gpu_variant.detect_expected_ollama_variant()
        if expected is None:
            self.skipTest("No GPU detected (nvidia-smi and rocm-smi both absent or empty)")
        if not helpers.ollama_container_running():
            self.skipTest(
                f"Ollama container ({config.OLLAMA_CONTAINER_NAME}) not running; "
                "start stack with node-manager (e.g. just setup-dev start)"
            )
        image = helpers.get_ollama_container_image()
        self.assertIsNotNone(image, "Ollama container running but image not found")
        actual_tag = _image_tag(image)
        self.assertTrue(
            _tag_matches_expected(actual_tag, expected),
            f"Ollama image tag {actual_tag!r} does not match expected variant {expected!r} "
            f"(rocm->:rocm; cuda->no tag or :latest) from GPU detection",
        )
