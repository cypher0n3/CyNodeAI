package nodeagent

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

// gpuDetectTTL controls how long the GPU detection result is cached.
// GPU hardware does not change at runtime; a long TTL avoids repeated
// external-process invocations on every capability report cycle.
const gpuDetectTTL = 5 * time.Minute

var (
	gpuCacheMu     sync.Mutex
	gpuCacheResult *nodepayloads.GPUInfo
	gpuCacheExpiry time.Time
)

// cachedGPUInfo returns a cached GPU detection result, refreshing it at most
// once per gpuDetectTTL.  Detection is bounded by a 5-second context so slow
// or missing tools do not stall the capability report loop.
// When NODE_MANAGER_TEST_NO_GPU_DETECT is set (unit tests), returns nil immediately.
func cachedGPUInfo(ctx context.Context) *nodepayloads.GPUInfo {
	if getEnv("NODE_MANAGER_TEST_NO_GPU_DETECT", "") != "" {
		return nil
	}
	gpuCacheMu.Lock()
	defer gpuCacheMu.Unlock()
	if time.Now().Before(gpuCacheExpiry) {
		return gpuCacheResult
	}
	detectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	gpuCacheResult = detectGPU(detectCtx)
	gpuCacheExpiry = time.Now().Add(gpuDetectTTL)
	return gpuCacheResult
}

// detectGPU probes available GPU hardware and returns a populated GPUInfo, or nil
// if no GPU is detected or the required tools are unavailable.
// Reports all GPUs from all supported vendors (AMD and NVIDIA) so the orchestrator
// can sum VRAM per vendor and select the variant for the vendor with greatest total
// (REQ-WORKER-0265, orchestrator_inference_container_decision.md).
func detectGPU(ctx context.Context) *nodepayloads.GPUInfo {
	nvidia := detectNVIDIAGPU(ctx)
	rocm := detectROCmGPU(ctx)
	if nvidia == nil && rocm == nil {
		return nil
	}
	var devices []nodepayloads.GPUDevice
	if nvidia != nil {
		devices = append(devices, nvidia.Devices...)
	}
	if rocm != nil {
		devices = append(devices, rocm.Devices...)
	}
	if len(devices) == 0 {
		return nil
	}
	return &nodepayloads.GPUInfo{Present: true, Devices: devices}
}

// totalVRAM returns the sum of VRAMMB across all devices (used for GPU preference).
func totalVRAM(info *nodepayloads.GPUInfo) int {
	if info == nil {
		return 0
	}
	var sum int
	for _, d := range info.Devices {
		sum += d.VRAMMB
	}
	return sum
}

// detectROCmGPU queries rocm-smi for AMD GPU information.
func detectROCmGPU(ctx context.Context) *nodepayloads.GPUInfo {
	out, err := exec.CommandContext(ctx, "rocm-smi", "--showproductname", "--showmeminfo", "vram", "--json").Output()
	if err != nil {
		return nil
	}
	return parseROCmSMIOutput(out)
}

// parseROCmSMIOutput parses the JSON emitted by `rocm-smi --json`.
// rocm-smi emits a map keyed by "card0", "card1", etc.
func parseROCmSMIOutput(out []byte) *nodepayloads.GPUInfo {
	var raw map[string]map[string]interface{}
	if json.Unmarshal(out, &raw) != nil {
		return nil
	}
	var devices []nodepayloads.GPUDevice
	for _, props := range raw {
		dev := nodepayloads.GPUDevice{
			Vendor:   "AMD",
			Features: map[string]interface{}{"rocm_version": "unknown"},
		}
		if name, ok := props["Card series"].(string); ok {
			dev.Model = strings.TrimSpace(name)
		}
		if name, ok := props["Card model"].(string); ok && dev.Model == "" {
			dev.Model = strings.TrimSpace(name)
		}
		if v, ok := props["VRAM Total Memory (B)"].(string); ok {
			if bytes, e := strconv.ParseInt(strings.TrimSpace(v), 10, 64); e == nil {
				dev.VRAMMB = int(bytes / 1024 / 1024)
			}
		}
		devices = append(devices, dev)
	}
	if len(devices) == 0 {
		return nil
	}
	return &nodepayloads.GPUInfo{Present: true, Devices: devices}
}

// gpuDiagTruncate is the max runes retained for raw tool stdout/stderr in GPUDiagnosticReport.
const gpuDiagTruncate = 8192

func truncateGPUdiag(s string) string {
	r := []rune(s)
	if len(r) <= gpuDiagTruncate {
		return s
	}
	return string(r[:gpuDiagTruncate]) + "\n... [truncated]"
}

// GPUDiagnosticReport captures raw rocm-smi / nvidia-smi output and parsed results.
// Use RunGPUDiagnostic to verify the host reports what node-manager sends in capability
// reports before debugging orchestrator or config delivery.
type GPUDiagnosticReport struct {
	ROCmSMI struct {
		LookupError string `json:"lookup_error,omitempty"`
		Path        string `json:"path,omitempty"`
		Args        string `json:"args,omitempty"`
		ExecError   string `json:"exec_error,omitempty"`
		Stdout      string `json:"stdout,omitempty"`
		Parsed      *nodepayloads.GPUInfo `json:"parsed,omitempty"`
	} `json:"rocm_smi"`
	NvidiaSMI struct {
		LookupError string `json:"lookup_error,omitempty"`
		Path        string `json:"path,omitempty"`
		Args        string `json:"args,omitempty"`
		ExecError   string `json:"exec_error,omitempty"`
		Stdout      string `json:"stdout,omitempty"`
		Parsed      *nodepayloads.GPUInfo `json:"parsed,omitempty"`
	} `json:"nvidia_smi"`
	// Merged matches detectGPU (capability report gpu field); same merge as cachedGPUInfo without cache.
	Merged *nodepayloads.GPUInfo `json:"merged_detect_gpu"`
}

// RunGPUDiagnostic runs rocm-smi and nvidia-smi with the same arguments as detectROCmGPU /
// detectNVIDIAGPU, records raw output, and sets Merged to detectGPU(ctx). It does not use the
// GPU detection cache. NODE_MANAGER_TEST_NO_GPU_DETECT does not apply here.
func RunGPUDiagnostic(ctx context.Context) *GPUDiagnosticReport {
	rep := &GPUDiagnosticReport{}
	rocmArgs := []string{"--showproductname", "--showmeminfo", "vram", "--json"}
	rep.ROCmSMI.Args = strings.Join(append([]string{"rocm-smi"}, rocmArgs...), " ")
	if p, err := exec.LookPath("rocm-smi"); err != nil {
		rep.ROCmSMI.LookupError = err.Error()
	} else {
		rep.ROCmSMI.Path = p
		cmd := exec.CommandContext(ctx, "rocm-smi", rocmArgs...)
		out, err := cmd.CombinedOutput()
		rep.ROCmSMI.Stdout = truncateGPUdiag(string(out))
		if err != nil {
			rep.ROCmSMI.ExecError = err.Error()
		}
		rep.ROCmSMI.Parsed = parseROCmSMIOutput(out)
	}

	nvArgs := []string{"--query-gpu=name,memory.total", "--format=csv,noheader,nounits"}
	rep.NvidiaSMI.Args = strings.Join(append([]string{"nvidia-smi"}, nvArgs...), " ")
	if p, err := exec.LookPath("nvidia-smi"); err != nil {
		rep.NvidiaSMI.LookupError = err.Error()
	} else {
		rep.NvidiaSMI.Path = p
		cmd := exec.CommandContext(ctx, "nvidia-smi", nvArgs...)
		out, err := cmd.CombinedOutput()
		rep.NvidiaSMI.Stdout = truncateGPUdiag(string(out))
		if err != nil {
			rep.NvidiaSMI.ExecError = err.Error()
		}
		rep.NvidiaSMI.Parsed = parseNvidiaSMIOutput(out)
	}

	rep.Merged = detectGPU(ctx)
	return rep
}

// detectNVIDIAGPU queries nvidia-smi for NVIDIA GPU information.
func detectNVIDIAGPU(ctx context.Context) *nodepayloads.GPUInfo {
	out, err := exec.CommandContext(ctx,
		"nvidia-smi",
		"--query-gpu=name,memory.total",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return nil
	}
	return parseNvidiaSMIOutput(out)
}

// parseNvidiaSMIOutput parses CSV output from `nvidia-smi --query-gpu=name,memory.total`.
func parseNvidiaSMIOutput(out []byte) *nodepayloads.GPUInfo {
	var devices []nodepayloads.GPUDevice
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			continue
		}
		dev := nodepayloads.GPUDevice{
			Vendor:   "NVIDIA",
			Model:    strings.TrimSpace(parts[0]),
			Features: map[string]interface{}{"cuda_capability": "unknown"},
		}
		if vram, e := strconv.Atoi(strings.TrimSpace(parts[1])); e == nil {
			dev.VRAMMB = vram
		}
		devices = append(devices, dev)
	}
	if len(devices) == 0 {
		return nil
	}
	return &nodepayloads.GPUInfo{Present: true, Devices: devices}
}
