package nodeagent

import (
	"context"
	"os"
	"testing"
	"time"
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
