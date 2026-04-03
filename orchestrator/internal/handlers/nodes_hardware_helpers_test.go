package handlers

import (
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestSumVRAMByVendor(t *testing.T) {
	t.Parallel()
	nv, amd := sumVRAMByVendor([]nodepayloads.GPUDevice{
		{Vendor: "NVIDIA", VRAMMB: 1000, Features: map[string]any{"cuda_capability": "8.0"}},
		{Vendor: "AMD", VRAMMB: 2000, Features: map[string]any{"rocm_version": "6.0"}},
		{Vendor: "NVIDIA", VRAMMB: -10},
	})
	if nv != 1000 || amd != 2000 {
		t.Fatalf("nv=%d amd=%d", nv, amd)
	}
}

func TestVariantFromFirstRecognizableDevice(t *testing.T) {
	t.Parallel()
	if v := variantFromFirstRecognizableDevice([]nodepayloads.GPUDevice{
		{Features: map[string]any{"rocm_version": "6"}},
	}); v != ollamaVariantROCm {
		t.Fatalf("got %q", v)
	}
	if v := variantFromFirstRecognizableDevice([]nodepayloads.GPUDevice{
		{Features: map[string]any{"cuda_capability": "8"}},
	}); v != ollamaVariantCUDA {
		t.Fatalf("got %q", v)
	}
	if v := variantFromFirstRecognizableDevice([]nodepayloads.GPUDevice{{Vendor: "AMD"}}); v != ollamaVariantROCm {
		t.Fatalf("got %q", v)
	}
	if v := variantFromFirstRecognizableDevice([]nodepayloads.GPUDevice{{Vendor: "NVIDIA"}}); v != ollamaVariantCUDA {
		t.Fatalf("got %q", v)
	}
	if v := variantFromFirstRecognizableDevice(nil); v != ollamaVariantCPU {
		t.Fatalf("got %q", v)
	}
}

func TestVariantAndVRAM(t *testing.T) {
	t.Parallel()
	v, mb := variantAndVRAM(&nodepayloads.CapabilityReport{
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{
				{Vendor: "NVIDIA", VRAMMB: 8000},
				{Vendor: "AMD", VRAMMB: 4000},
			},
		},
	})
	if v != ollamaVariantCUDA || mb != 8000 {
		t.Fatalf("want cuda/8000 got %s/%d", v, mb)
	}
	v2, mb2 := variantAndVRAM(&nodepayloads.CapabilityReport{GPU: &nodepayloads.GPUInfo{Present: false}})
	if v2 != ollamaVariantCPU || mb2 != 0 {
		t.Fatalf("empty gpu: %s %d", v2, mb2)
	}
	// AMD wins when strictly greater VRAM.
	v3, mb3 := variantAndVRAM(&nodepayloads.CapabilityReport{
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{
				{Vendor: "NVIDIA", VRAMMB: 1000},
				{Vendor: "AMD", VRAMMB: 8000},
			},
		},
	})
	if v3 != ollamaVariantROCm || mb3 != 8000 {
		t.Fatalf("amd win: got %s %d", v3, mb3)
	}
	// Tie on VRAM: prefer CUDA (REQ tie-break).
	v4, mb4 := variantAndVRAM(&nodepayloads.CapabilityReport{
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{
				{Vendor: "NVIDIA", VRAMMB: 4000},
				{Vendor: "AMD", VRAMMB: 4000},
			},
		},
	})
	if v4 != ollamaVariantCUDA || mb4 != 4000 {
		t.Fatalf("tie cuda: got %s %d", v4, mb4)
	}
}
