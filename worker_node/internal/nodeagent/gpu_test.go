package nodeagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestDetectGPU_NoToolsReturnsNil(t *testing.T) {
	// On CI/test machines without rocm-smi or nvidia-smi, detectGPU must return nil
	// rather than panic or error.
	t.Setenv("PATH", t.TempDir())
	got := detectGPU(context.Background())
	if got != nil {
		t.Errorf("expected nil when no GPU tools present, got %+v", got)
	}
}

func TestRunGPUDiagnostic_ReturnsReport(t *testing.T) {
	ctx := context.Background()
	rep := RunGPUDiagnostic(ctx)
	if rep == nil {
		t.Fatal("RunGPUDiagnostic returned nil")
	}
	// Merged may be nil on machines without GPUs/tools; report must still marshal.
	if rep.Merged == nil && rep.ROCmSMI.LookupError == "" && rep.ROCmSMI.Path != "" {
		t.Log("rocm-smi present but merged nil: check rocm_smi stdout/json parse")
	}
}

func TestDetectROCmGPU_MissingBinaryReturnsNil(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got := detectROCmGPU(context.Background())
	if got != nil {
		t.Errorf("expected nil when rocm-smi is absent, got %+v", got)
	}
}

func TestDetectNVIDIAGPU_MissingBinaryReturnsNil(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got := detectNVIDIAGPU(context.Background())
	if got != nil {
		t.Errorf("expected nil when nvidia-smi is absent, got %+v", got)
	}
}

func TestDetectGPU_BothToolsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got := detectGPU(context.Background())
	if got != nil {
		t.Errorf("detectGPU with no tools: want nil, got %+v", got)
	}
}

func TestCachedGPUInfo_SkipEnvReturnsNil(t *testing.T) {
	t.Setenv("NODE_MANAGER_TEST_NO_GPU_DETECT", "1")
	got := cachedGPUInfo(context.Background())
	if got != nil {
		t.Errorf("expected nil when skip env set, got %+v", got)
	}
}

func TestParseROCmSMIOutput_Valid(t *testing.T) {
	// Simplified rocm-smi JSON: one card entry.
	input := []byte(`{
		"card0": {
			"Card series": "Radeon RX 7900 XTX",
			"VRAM Total Memory (B)": "21458059264"
		}
	}`)
	got := parseROCmSMIOutput(input)
	if got == nil {
		t.Fatal("expected non-nil GPUInfo")
	}
	if !got.Present {
		t.Error("expected Present=true")
	}
	if len(got.Devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(got.Devices))
	}
	d := got.Devices[0]
	if d.Vendor != "AMD" {
		t.Errorf("vendor = %q, want AMD", d.Vendor)
	}
	if d.Model != "Radeon RX 7900 XTX" {
		t.Errorf("model = %q", d.Model)
	}
	// 21458059264 bytes / 1024 / 1024 = 20464 MB (rocm-smi reports slightly under 20 GiB)
	if d.VRAMMB != 20464 {
		t.Errorf("VRAMMB = %d, want 20464", d.VRAMMB)
	}
}

func TestParseROCmSMIOutput_InvalidJSON(t *testing.T) {
	got := parseROCmSMIOutput([]byte("not json"))
	if got != nil {
		t.Errorf("expected nil for bad JSON, got %+v", got)
	}
}

func TestParseROCmSMIOutput_EmptyDevices(t *testing.T) {
	got := parseROCmSMIOutput([]byte(`{}`))
	if got != nil {
		t.Errorf("expected nil for empty device map, got %+v", got)
	}
}

func TestParseNvidiaSMIOutput_Valid(t *testing.T) {
	input := []byte("NVIDIA GeForce RTX 4090, 24576\n")
	got := parseNvidiaSMIOutput(input)
	if got == nil {
		t.Fatal("expected non-nil GPUInfo")
	}
	if !got.Present || len(got.Devices) != 1 {
		t.Fatalf("expected 1 device; got %+v", got)
	}
	d := got.Devices[0]
	if d.Vendor != "NVIDIA" {
		t.Errorf("vendor = %q, want NVIDIA", d.Vendor)
	}
	if d.Model != "NVIDIA GeForce RTX 4090" {
		t.Errorf("model = %q", d.Model)
	}
	if d.VRAMMB != 24576 {
		t.Errorf("VRAMMB = %d, want 24576", d.VRAMMB)
	}
}

func TestParseNvidiaSMIOutput_MultiGPU(t *testing.T) {
	input := []byte("Tesla A100, 81920\nTesla A100, 81920\n")
	got := parseNvidiaSMIOutput(input)
	if got == nil || len(got.Devices) != 2 {
		t.Fatalf("expected 2 devices; got %+v", got)
	}
}

func TestParseNvidiaSMIOutput_MalformedLine(t *testing.T) {
	// Lines with no comma are skipped; valid lines are still parsed.
	input := []byte("no-comma-line\nRTX 3080, 10240\n")
	got := parseNvidiaSMIOutput(input)
	if got == nil || len(got.Devices) != 1 {
		t.Fatalf("expected 1 device from mixed input; got %+v", got)
	}
}

func TestParseNvidiaSMIOutput_Empty(t *testing.T) {
	got := parseNvidiaSMIOutput([]byte(""))
	if got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
}

func TestDetectGPU_ReportsAllDevicesWhenBothVendorsPresent(t *testing.T) {
	// Worker reports all GPUs from all vendors so orchestrator can sum VRAM per vendor.
	binDir := t.TempDir()
	nvidiaOut := "NVIDIA GeForce RTX 3080, 10240\n"
	rocmOut := `{"card0":{"Card series":"Radeon Vega","VRAM Total Memory (B)":"8589934592"}}`
	if err := os.WriteFile(filepath.Join(binDir, "nvidia-smi"), []byte("#!/bin/sh\necho '"+nvidiaOut+"'"), 0o755); err != nil {
		t.Fatalf("write nvidia-smi: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "rocm-smi"), []byte("#!/bin/sh\necho '"+rocmOut+"'"), 0o755); err != nil {
		t.Fatalf("write rocm-smi: %v", err)
	}
	t.Setenv("PATH", binDir+string(filepath.ListSeparator)+os.Getenv("PATH"))
	gpuCacheMu.Lock()
	gpuCacheExpiry = time.Time{}
	gpuCacheResult = nil
	gpuCacheMu.Unlock()
	got := detectGPU(context.Background())
	if got == nil {
		t.Fatal("expected non-nil GPUInfo when both tools present")
	}
	if len(got.Devices) != 2 {
		t.Fatalf("expected 2 devices (1 NVIDIA + 1 AMD), got %d", len(got.Devices))
	}
	var nvidia, amd int
	for _, d := range got.Devices {
		switch d.Vendor {
		case "NVIDIA":
			nvidia++
		case "AMD":
			amd++
		}
	}
	if nvidia != 1 || amd != 1 {
		t.Errorf("expected 1 NVIDIA and 1 AMD device, got %d NVIDIA and %d AMD", nvidia, amd)
	}
}

func TestDetectGPU_SingleVendorReturnsAllDevices(t *testing.T) {
	// Multiple GPUs of same vendor are all reported.
	binDir := t.TempDir()
	nvidiaOut := "Tesla A100, 81920\nTesla A100, 81920\n"
	if err := os.WriteFile(filepath.Join(binDir, "nvidia-smi"), []byte("#!/bin/sh\necho '"+nvidiaOut+"'"), 0o755); err != nil {
		t.Fatalf("write nvidia-smi: %v", err)
	}
	t.Setenv("PATH", binDir+string(filepath.ListSeparator)+os.Getenv("PATH"))
	gpuCacheMu.Lock()
	gpuCacheExpiry = time.Time{}
	gpuCacheResult = nil
	gpuCacheMu.Unlock()
	got := detectGPU(context.Background())
	if got == nil {
		t.Fatal("expected non-nil GPUInfo")
	}
	if len(got.Devices) != 2 {
		t.Fatalf("expected 2 NVIDIA devices, got %d", len(got.Devices))
	}
	if totalVRAM(got) != 163840 {
		t.Errorf("totalVRAM = %d, want 163840 (2x 81920)", totalVRAM(got))
	}
}

func TestTotalVRAM(t *testing.T) {
	if totalVRAM(nil) != 0 {
		t.Error("totalVRAM(nil) should be 0")
	}
	info := &nodepayloads.GPUInfo{
		Devices: []nodepayloads.GPUDevice{{VRAMMB: 8192}, {VRAMMB: 8192}},
	}
	if got := totalVRAM(info); got != 16384 {
		t.Errorf("totalVRAM = %d, want 16384", got)
	}
}

func TestCachedGPUInfo_UsesCache(t *testing.T) {
	// Ensure skip env is not set so the cache path is exercised.
	_ = os.Unsetenv("NODE_MANAGER_TEST_NO_GPU_DETECT")
	defer t.Setenv("NODE_MANAGER_TEST_NO_GPU_DETECT", "1")

	// Force an empty PATH so GPU tools are absent; result should be nil but cached.
	t.Setenv("PATH", t.TempDir())
	// Reset cache state to ensure a fresh detection.
	gpuCacheMu.Lock()
	gpuCacheExpiry = time.Time{}
	gpuCacheResult = nil
	gpuCacheMu.Unlock()

	got1 := cachedGPUInfo(context.Background())
	// Second call should use cache (no external commands).
	got2 := cachedGPUInfo(context.Background())
	if got1 != got2 {
		t.Error("cachedGPUInfo should return the same pointer from cache")
	}
}
