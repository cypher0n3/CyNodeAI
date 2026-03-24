# NVIDIA GPU Systems

- [1 Summary](#1-summary)
- [2 Software Requirements](#2-software-requirements)
- [3 Inference Backend Variant](#3-inference-backend-variant)
- [4 Troubleshooting Orientation](#4-troubleshooting-orientation)

## 1 Summary

NVIDIA GPUs are a **supported MVP** path for accelerated local inference on worker nodes.
The system maps capable NVIDIA hardware to a **CUDA**-oriented inference backend variant when the orchestrator configures the node to run that variant.

This page is host-oriented guidance.
Exact container images and startup behavior are defined in requirements and tech specs, not duplicated here.

**Role:** [Worker node hosts](worker_node.md) (inference on the worker).
**Containers:** [Common host requirements](common.md#2-container-runtime).

## 2 Software Requirements

**Container runtime:** [Common host requirements, section 2](common.md#2-container-runtime) (Podman, Compose, distribution packages).

GPU access inside containers requires the **NVIDIA driver** on the host, **`nvidia-smi`** on the host **`PATH`**, **NVIDIA Container Toolkit**, and runtime configuration per [NVIDIA Container Toolkit install guide](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html) (**Podman** typically uses **CDI**).

The following sections cover **`nvidia-smi`**, the **proprietary driver**, the **NVIDIA Container Toolkit**, and **distribution examples** for mainline Linux.

### 2.1 NVIDIA Management CLI (`nvidia-smi`)

**`nvidia-smi`** is **required** host software alongside the proprietary NVIDIA driver and CUDA-capable userspace.
Use it to confirm the driver, GPU visibility, and memory before you trust Compose or worker inference containers.

- **Arch Linux:** **`nvidia-utils`** provides **`/usr/bin/nvidia-smi`**.
  Install it with **`nvidia`** or **`nvidia-open`** (both depend on **`nvidia-utils`**).
- **Debian / Ubuntu:** **`nvidia-smi`** ships with the NVIDIA userspace tied to your driver metapackage; it is commonly in **`nvidia-utils-<version>`** or a similarly named **`nvidia-utils`** package.
  Run **`dpkg -S "$(command -v nvidia-smi)"`** after install to record the owning package on your release.
- **Fedora (RPM Fusion stack):** **`nvidia-utils`** provides **`nvidia-smi`**.

### 2.2 NVIDIA Proprietary Driver (All Distributions)

You need a **proprietary NVIDIA kernel driver** and matching **userspace** so **`nvidia-smi`** works on the host before you debug Compose or worker containers.

- Driver major versions must support your GPU generation; pick the package your distribution recommends for that hardware.
- Keep **kernel updates** and **driver packages** in sync with your distribution's supported combinations.

### 2.3 NVIDIA Container Toolkit (All Distributions)

The **NVIDIA Container Toolkit** (packages such as `nvidia-container-toolkit`, `nvidia-container-toolkit-base`, `libnvidia-container-tools`, `libnvidia-container1` on Debian-family systems) lets the container runtime inject NVIDIA devices and userspace libraries into GPU-backed containers.

- **Canonical install steps** (apt, dnf, zypper, and post-install **`nvidia-ctk`** configuration): [NVIDIA Container Toolkit install guide](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html).
- **Arch Linux** ships the toolkit in the main repos as **`nvidia-container-toolkit`**; configure **Podman** (CDI) or **Docker** per that guide.

### 2.4 Distribution Examples (Mainline Linux)

These are **examples** of common package names.
Verify versions against your GPU, kernel, and distribution release notes.

#### 2.4.1 Arch Linux

- **Driver and userspace:** **`nvidia`** and **`nvidia-utils`**, or **`nvidia-open`** and **`nvidia-utils`** if you use NVIDIA's open kernel modules (supported GPU list is narrower; see Arch Wiki **NVIDIA** article).
- **`nvidia-smi`:** **`nvidia-utils`** (see [section 2.1](#21-nvidia-management-cli-nvidia-smi)).
- **32-bit compatibility (optional, desktop):** enable **`multilib`** and install **`lib32-nvidia-utils`** when needed.
- **NVIDIA Container Toolkit:** **`nvidia-container-toolkit`** (official package name in the **`extra`** repository).
- **Configure the runtime** after install (for example **`nvidia-ctk`** for **Docker** or **Podman CDI** per NVIDIA docs).

#### 2.4.2 Debian and Ubuntu

Use **`apt`** with Ubuntu **restricted** / multiverse or Debian **`contrib`/`non-free`** as required by your release.

##### 2.4.2.1 Proprietary Driver Packages

- **Ubuntu:** install a metapackage such as **`nvidia-driver-550`** or **`nvidia-driver-580`** (the number changes with releases), or install **`ubuntu-drivers-common`** and run **`ubuntu-drivers autoinstall`** after enabling **restricted / proprietary drivers** where applicable.
- **Debian:** enable **`contrib` and `non-free`** (and **`non-free-firmware`** on releases that split firmware), then install **`nvidia-driver`** or a release-specific metapackage from **`non-free`** (exact names depend on the Debian version; use **`apt-cache search nvidia-driver`**).
- **`nvidia-smi`:** installed with the driver userspace; confirm with **`dpkg -S "$(command -v nvidia-smi)"`** (see [section 2.1](#21-nvidia-management-cli-nvidia-smi)).

##### 2.4.2.2 NVIDIA Container Toolkit Packages (Apt)

- Add NVIDIA's **`libnvidia-container`** repository and install **`nvidia-container-toolkit`** per [NVIDIA Container Toolkit install guide](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html) (apt section).
- That guide lists the related packages **`nvidia-container-toolkit-base`**, **`libnvidia-container-tools`**, and **`libnvidia-container1`** when you pin or install the full dependency set.

#### 2.4.3 Fedora Packages

Fedora typically uses **RPM Fusion** for the proprietary driver and NVIDIA's **dnf** repo for the container toolkit.

##### 2.4.3.1 Proprietary Driver Packages (RPM Fusion Path)

- Enable **RPM Fusion** **`nonfree`** (and **`free`** as a dependency), then install **`akmod-nvidia`** and **`xorg-x11-drv-nvidia-cuda`** so the kernel module tracks kernel updates.
- **`nvidia-smi`:** install **`nvidia-utils`** (see [section 2.1](#21-nvidia-management-cli-nvidia-smi)).
- Alternative third-party NVIDIA repositories exist; pick one consistent stack and follow its documentation.

##### 2.4.3.2 NVIDIA Container Toolkit Packages (Dnf)

- Add NVIDIA's **dnf** repository from [NVIDIA Container Toolkit install guide](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html) (dnf section) and install **`nvidia-container-toolkit`** (and matching **`nvidia-container-toolkit-base`**, **`libnvidia-container-tools`**, **`libnvidia-container1`** if you install the full set).

### 2.5 Quick Validation

- On the host: **`nvidia-smi`** must list the GPU and driver version.
- After toolkit configuration: run NVIDIA's **sample workload** from the same Container Toolkit documentation, or a minimal **`podman run`** / **`docker run`** with GPU access (syntax depends on engine and CDI setup).

## 3 Inference Backend Variant

Requirements describe an orchestrator-supplied **backend variant** for local inference (for Ollama, **CUDA** maps to the standard Ollama image family rather than a separate `cuda` tag; see worker requirements for image selection rules).

Workers **MUST** start the backend only after registration and orchestrator-supplied configuration that authorizes the inference backend, including variant when applicable.

**Multi-vendor reporting (worker) and placement (orchestrator):** [Worker node hosts, section 4](worker_node.md#4-capability-reporting-worker-obligations) and [Orchestrator backend hosts, section 5](orchestrator.md#5-inference-variant-and-placement-orchestrator-policy).

## 4 Troubleshooting Orientation

- Confirm **`nvidia-smi`** works on the host before testing inside containers.
- Confirm the container runtime sees GPU devices when running a diagnostic container with GPU flags.
- Separate **driver install** issues from **Orchestrator or worker configuration** issues by reproducing GPU access outside CyNodeAI first.

**Requirement traceability:** [Worker node hosts, section 6](worker_node.md#6-traceability).
