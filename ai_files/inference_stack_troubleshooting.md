# Inference Stack Troubleshooting (Agents and Operators)

- [Purpose](#purpose)
- [Mindset](#mindset)
- [Symptoms](#symptoms)
- [Checklist (order matters)](#checklist-order-matters)
- [Reference](#reference)

## Purpose

Use this guide when **Python E2E** tests that touch **chat, PMA, SSE streaming, or SBA inference**
are **slow**, **timeout**, return **502** / **non-2xx**, or pull the **wrong Ollama image** (for
example CPU/CUDA instead of ROCm on AMD hosts).

It complements normative docs in [`docs/requirements/`](../docs/requirements/) and
[`docs/tech_specs/`](../docs/tech_specs/), and practical host guidance in
[`docs/system_reqs/`](../docs/system_reqs/README.md).

## Mindset

The project **controls** dev stacks, images, and `just` recipes.
Treat persistent reds as **bugs**
to fix (host packages, node-manager detection, orchestrator payloads, gateway, tests), not as vague
"environment problems."
See [`meta.md`](../meta.md) (Controlled Stack and Test Failures).

Hardware limits (RAM, VRAM, disk) are the main carve-out; even then, prefer **clear failures** over
unbounded hangs.

## Symptoms

- **`cynork chat` or HTTP chat** hits **multi-minute timeouts** or empty replies.
- **SSE / streaming** tests show **`502 Bad Gateway`**, **`PMA proxy stream returned 502`**, or
  **ConnectionError** mid-stream.
- **Wrong container image:** Ollama runs as **`ollama/ollama`** (default) instead of
  **`ollama/ollama:rocm`** on an AMD system that should use ROCm.
- **SBA / sandbox** tasks fail pulling **`cynodeai-inference-proxy`** or proxy start errors (separate
  from Ollama variant; fix image build and `INFERENCE_PROXY_IMAGE` per development setup).

## Checklist (Order Matters)

Work through the following in order; skipping earlier steps wastes time.

**Step 1: GPU CLI tools on the host `PATH` (node-manager).**
On **AMD** systems, **`rocm-smi` is
required** so node-manager can detect AMD GPUs and the orchestrator can select the **ROCm** inference
variant.
Without it, AMD may be invisible and the wrong Ollama image may be chosen.
See
[`docs/system_reqs/amd.md`](../docs/system_reqs/amd.md) (ROCm System Management CLI).
On **NVIDIA**
systems, **`nvidia-smi`** is the corresponding probe.

**Step 2: What the node would report.**
Run the node-manager binary (for example
`./worker_node/bin/cynodeai-wnm-dev`) with **`--print-gpu-detect`**.
In the JSON,
**`merged_detect_gpu`** should list expected vendors and VRAM when tools work.
If
**`rocm_smi.lookup_error`** or **`exec_error`** is set, fix install and **`PATH`** before debugging
Go code.

**Step 3: Orchestrator variant and image.**
If steps 1--2 look correct but the wrong image still
runs, trace **`inference_backend.variant`** and **`inference_backend.image`** in the node
configuration payload (orchestrator decision and node-manager rules in requirements and tech
specs).

**Step 4: Gateway, PMA, and Ollama health.**
Timeouts and **502** on chat or streams often mean the
inference backend behind PMA is unhealthy, overloaded, or misconfigured after variant selection.
Verify Ollama on the expected port and check PMA and user-gateway logs for repeated upstream
failures.

**Step 5: SBA and inference-proxy image.**
Failures that mention **proxy** pull or sandbox start
refer to **`INFERENCE_PROXY_IMAGE`** and local image build (for example `just` recipes for
`cynodeai-inference-proxy:dev`), not only the ROCm/CUDA Ollama tag.

## Reference

- AMD host requirements: [`docs/system_reqs/amd.md`](../docs/system_reqs/amd.md)  
- NVIDIA host requirements: [`docs/system_reqs/nvidia.md`](../docs/system_reqs/nvidia.md)  
- Common host requirements: [`docs/system_reqs/common.md`](../docs/system_reqs/common.md)  
- Node-manager GPU diagnostic flag: [`worker_node/README.md`](../worker_node/README.md) (GPU and
  Ollama image section)  
- Python E2E harness: [`scripts/test_scripts/README.md`](../scripts/test_scripts/README.md)
