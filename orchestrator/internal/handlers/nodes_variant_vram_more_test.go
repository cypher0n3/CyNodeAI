package handlers

import (
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestVariantAndVRAM_SumVRAMAndTieBreak(t *testing.T) {
	// Sum VRAM per vendor; tie-break prefers cuda. Table covers mixed-GPU and equal-VRAM cases.
	tests := []struct {
		name        string
		devices     []nodepayloads.GPUDevice
		wantVariant string
		wantVRAM    int
	}{
		{
			name: "mixed_nvidia_dominant",
			devices: []nodepayloads.GPUDevice{
				{VRAMMB: 8192, Features: map[string]interface{}{"cuda_capability": "8.6"}},
				{VRAMMB: 2048, Features: map[string]interface{}{"rocm_version": "6.0"}},
			},
			wantVariant: ollamaVariantCUDA,
			wantVRAM:    8192,
		},
		{
			name: "tie_prefers_cuda",
			devices: []nodepayloads.GPUDevice{
				{VRAMMB: 8192, Features: map[string]interface{}{"cuda_capability": "8.6"}},
				{VRAMMB: 8192, Features: map[string]interface{}{"rocm_version": "6.0"}},
			},
			wantVariant: ollamaVariantCUDA,
			wantVRAM:    8192,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := &nodepayloads.CapabilityReport{
				GPU: &nodepayloads.GPUInfo{Present: true, Devices: tc.devices},
			}
			variant, vramMB := variantAndVRAM(report)
			if variant != tc.wantVariant {
				t.Errorf("variant = %q, want %s", variant, tc.wantVariant)
			}
			if vramMB != tc.wantVRAM {
				t.Errorf("vramMB = %d, want %d", vramMB, tc.wantVRAM)
			}
		})
	}
}

func TestVariantAndVRAM_MultiGPUSameVendorSumsTotal(t *testing.T) {
	// Multiple GPUs of same vendor: vramMB is sum of all devices.
	report := &nodepayloads.CapabilityReport{
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{
				{VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
				{VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
				{VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
			},
		},
	}
	variant, vramMB := variantAndVRAM(report)
	if variant != ollamaVariantCUDA {
		t.Errorf("variant = %q, want %s", variant, ollamaVariantCUDA)
	}
	if vramMB != 36864 {
		t.Errorf("vramMB = %d, want 36864 (3x 12 GB)", vramMB)
	}
}

func TestVariantAndVRAM_MixedVendorsNVIDIADominant(t *testing.T) {
	// 1 AMD 20 GB vs 3 NVIDIA 12 GB each -> cuda (36 > 20).
	report := &nodepayloads.CapabilityReport{
		GPU: &nodepayloads.GPUInfo{
			Present: true,
			Devices: []nodepayloads.GPUDevice{
				{VRAMMB: 20480, Features: map[string]interface{}{"rocm_version": "6.0"}},
				{VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
				{VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
				{VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
			},
		},
	}
	variant, vramMB := variantAndVRAM(report)
	if variant != ollamaVariantCUDA {
		t.Errorf("variant = %q, want %s (NVIDIA 36 GB > AMD 20 GB)", variant, ollamaVariantCUDA)
	}
	if vramMB != 36864 {
		t.Errorf("vramMB = %d, want 36864", vramMB)
	}
}
