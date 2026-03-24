# AMD GPU Systems

- [1 Summary](#1-summary)
- [2 Software Requirements](#2-software-requirements)
- [3 Inference Backend Variant](#3-inference-backend-variant)
- [4 Troubleshooting Orientation](#4-troubleshooting-orientation)

## 1 Summary

AMD GPUs are a **supported MVP** path for accelerated local inference on worker nodes.
The system maps capable AMD hardware to a **ROCm**-oriented inference backend variant when the orchestrator configures the node accordingly (for Ollama, requirements reference an image tag such as `ollama/ollama:rocm` when variant is ROCm).

This page is host-oriented guidance.
Exact images and payloads are normative in requirements and tech specs.

**Role:** [Worker node hosts](worker_node.md) (inference on the worker).
**Containers:** [Common host requirements](common.md#2-container-runtime).

## 2 Software Requirements

**Container runtime:** [Common host requirements, section 2](common.md#2-container-runtime) (Podman, Compose, distribution packages).

ROCm-backed inference containers need **`/dev/kfd`**, **`/dev/dri`**, and correct **userspace libraries** on the host or injected consistently with your runtime; group membership often includes **`video`** and **`render`** (and sometimes a **`rocm`** group depending on packaging).

The following sections cover **`rocm-smi`**, **kernel and firmware**, **ROCm userspace**, and **distribution examples** for mainline Linux.

### 2.1 ROCm System Management CLI (`rocm-smi`)

**`rocm-smi`** is **required** host software alongside the ROCm stack you use for HIP and runtime libraries.
It validates that the AMDGPU stack and ROCm userspace see your GPU.

- **Arch Linux:** **`rocm-smi-lib`** provides **`/usr/bin/rocm-smi`** (Python wrapper and supporting library).
  Install it as part of your ROCm selection (often with **`rocm-hip-runtime`** and related packages).
- **Debian / Ubuntu (AMD `apt` repositories):** the package that owns **`rocm-smi`** varies by ROCm release; AMD's meta package **`rocm`** or a dedicated **`rocm-smi`** / tooling package may apply.
  Run **`dpkg -S "$(command -v rocm-smi)"`** after install to record the owning package.
- **Fedora / RHEL-family (AMD `dnf` repositories):** use the ROCm package set from [ROCm installation for Linux](https://rocm.docs.amd.com/en/latest/deploy/linux/quick_start.html), then confirm with **`rpm -qf "$(command -v rocm-smi)"`**.

### 2.2 Kernel and Firmware (All Distributions)

- The **`amdgpu`** kernel driver is normally part of the Linux kernel your distribution ships; very new GPUs may need a **newer kernel** or **firmware** from **`linux-firmware`** (on Arch the package is **`linux-firmware`**; Debian/Ubuntu/Fedora ship equivalent firmware packages).
- Install **AMD GPU firmware** updates when your distribution recommends them for your card generation.

### 2.3 ROCm Userspace (All Distributions)

ROCm provides HIP, runtime libraries, and the **`rocm-smi`** tool (see [section 2.1](#21-rocm-system-management-cli-rocm-smi)).
**Version alignment** between kernel support, ROCm packages, and container images matters; follow **AMD ROCm** documentation for your GPU and OS.

- **Canonical entry point:** [ROCm installation for Linux](https://rocm.docs.amd.com/en/latest/deploy/linux/quick_start.html) (path may redirect as AMD reorganizes docs; if broken, start from [ROCm documentation](https://rocm.docs.amd.com/)).
- AMD documents **package-manager installs** (apt, dnf) and optional **graphics** repositories for Radeon versus datacenter GPUs.

### 2.4 Distribution Examples (Mainline Linux)

These are **examples** of packages that commonly appear on **mainline** distributions.
Always confirm **GPU support** and **ROCm version** against AMD's matrix for your exact model.

#### 2.4.1 Arch Linux

Packages are in the **`extra`** repository (names track upstream ROCm versions):

- **Runtime baseline:** **`rocm-hip-runtime`**, **`rocm-language-runtime`**, **`hsa-rocr`**, **`rocm-opencl-runtime`** (when you need OpenCL), **`rocm-smi-lib`** (**`rocm-smi`**; see [section 2.1](#21-rocm-system-management-cli-rocm-smi)).
- **Broader stack:** **`rocm-hip-libraries`**, **`rocm-ml-libraries`**, or **`rocm-hip-sdk`** / **`rocm-ml-sdk`** for development-oriented installs.

Use **`pacman -Ss rocm`** to list the current meta packages for your Arch snapshot.

#### 2.4.2 Debian and Ubuntu

AMD publishes **apt** repositories (for example **`repo.radeon.com`**) per Ubuntu release and ROCm version.

- **Typical pattern:** register the **ROCm** and **graphics** apt sources AMD documents for your **Ubuntu codename**, run **`apt update`**, then install a meta package such as **`rocm`** or a smaller set such as **`rocm-hip-runtime`**, **`rocm-language-runtime`**, **`rocm-hip-libraries`** (exact names depend on the ROCm release; AMD's Ubuntu package-manager page lists them).
- **`rocm-smi`:** confirm the package that installs it on your ROCm version (see [section 2.1](#21-rocm-system-management-cli-rocm-smi)).
- **Debian:** native **`rocm`** packages may lag or differ; prefer AMD's documented **Debian/Ubuntu** path for ROCm, or use **containers** with a supported Ubuntu base for ROCm if your Debian release is unsupported.

Do **not** mix conflicting ROCm stacks from random PPAs with AMD's official repo without reading AMD's release notes.

#### 2.4.3 Fedora Packages

- AMD provides **RPM** repositories for RHEL-family systems; Fedora installs often use **`dnf`** with AMD's **`repo.radeon.com`** configuration files for the matching ROCm release.
- Install **`rocm`** or **`rocm-hip-runtime`** (and dependencies) per [ROCm installation for Linux](https://rocm.docs.amd.com/en/latest/deploy/linux/quick_start.html) **Fedora / RHEL** section when available.
- **`rocm-smi`:** verify with **`rpm -qf`** after install (see [section 2.1](#21-rocm-system-management-cli-rocm-smi)).

If your Fedora release is not yet listed, use the **closest** documented RHEL or Fedora version in AMD's matrix, or run ROCm workloads inside a **supported** container base image.

### 2.5 Quick Validation

- On the host: **`rocm-smi`** (or **`rocminfo`**) must see the GPU when ROCm is correctly installed.
- For Compose: confirm the inference service can open **`/dev/kfd`** and **`/dev/dri`** with the right **supplementary groups** for the container user.

## 3 Inference Backend Variant

Workers **MUST** use the orchestrator-supplied inference backend variant and **MUST NOT** override it with a conflicting node-local default (for example a single `OLLAMA_IMAGE` that contradicts the assigned variant).

When the variant is ROCm, image selection follows the rules in worker requirements (including defaults when `inference_backend.image` is omitted).

**Multi-vendor reporting (worker) and placement (orchestrator):** [Worker node hosts, section 4](worker_node.md#4-capability-reporting-worker-obligations) and [Orchestrator backend hosts, section 5](orchestrator.md#5-inference-variant-and-placement-orchestrator-policy).

## 4 Troubleshooting Orientation

- Confirm **`rocm-smi`** sees the GPU on the host before debugging CyNodeAI services.
- Confirm the inference container receives device nodes and compatible userspace libraries for the ROCm version in use.
- If performance is poor, distinguish **ROCm configuration** from **model size and memory** limits.
- For **AI agents and developers** diagnosing timeouts, wrong Ollama images, or E2E inference failures,
  see [`ai_files/inference_stack_troubleshooting.md`](../../ai_files/inference_stack_troubleshooting.md)
  (checklist includes **`--print-gpu-detect`** on node-manager).

**Requirement traceability:** [Worker node hosts, section 6](worker_node.md#6-traceability).
