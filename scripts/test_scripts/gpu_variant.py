"""GPU variant detection for E2E validation.

Determines expected Ollama variant (cuda vs rocm) by querying nvidia-smi and rocm-smi,
summing VRAM per vendor, and selecting the vendor with greater total VRAM.
Mirrors worker_node/internal/nodeagent/gpu.go logic for independent validation.
"""

from __future__ import annotations

import json
import subprocess


def _detect_nvidia_vram_mb() -> int:
    """Query nvidia-smi for total VRAM across all NVIDIA GPUs. Return 0 if unavailable."""
    try:
        r = subprocess.run(
            [
                "nvidia-smi",
                "--query-gpu=name,memory.total",
                "--format=csv,noheader,nounits",
            ],
            capture_output=True,
            text=True,
            timeout=10,
            check=False,
        )
        if r.returncode or not r.stdout:
            return 0
        total = 0
        for line in r.stdout.strip().splitlines():
            parts = line.split(",", 1)
            if len(parts) != 2:
                continue
            try:
                total += int(parts[1].strip())
            except ValueError:
                continue
        return total
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return 0


def _detect_rocm_vram_mb() -> int:
    """Query rocm-smi for total VRAM across all AMD GPUs. Return 0 if unavailable."""
    try:
        r = subprocess.run(
            [
                "rocm-smi",
                "--showproductname",
                "--showmeminfo",
                "vram",
                "--json",
            ],
            capture_output=True,
            text=True,
            timeout=10,
            check=False,
        )
        if r.returncode or not r.stdout:
            return 0
        raw = json.loads(r.stdout)
        if not isinstance(raw, dict):
            return 0
        total = 0
        for props in raw.values():
            if not isinstance(props, dict):
                continue
            vram_str = props.get("VRAM Total Memory (B)")
            if isinstance(vram_str, str):
                try:
                    total += int(vram_str.strip()) // (1024 * 1024)
                except ValueError:
                    pass
        return total
    except (subprocess.TimeoutExpired, FileNotFoundError, json.JSONDecodeError):
        return 0


def detect_expected_ollama_variant() -> str | None:
    """Determine expected Ollama image variant from host GPU.

    Uses total VRAM per vendor (NVIDIA vs AMD); prefers vendor with greater total.
    Returns "cuda" for NVIDIA, "rocm" for AMD, or None if no GPU detected.
    """
    nvidia_vram = _detect_nvidia_vram_mb()
    rocm_vram = _detect_rocm_vram_mb()
    if nvidia_vram == 0 and rocm_vram == 0:
        return None
    if nvidia_vram == 0:
        return "rocm"
    if rocm_vram == 0:
        return "cuda"
    return "cuda" if nvidia_vram >= rocm_vram else "rocm"
