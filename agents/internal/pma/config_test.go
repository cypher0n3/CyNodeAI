package pma

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_InstructionsPath(t *testing.T) {
	wd, _ := os.Getwd()
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name: "project_manager default",
			config: Config{
				Role:             RoleProjectManager,
				InstructionsRoot: "instructions",
			},
			want: filepath.Join(wd, "instructions", "project_manager"),
		},
		{
			name: "project_analyst default",
			config: Config{
				Role:             RoleProjectAnalyst,
				InstructionsRoot: "instructions",
			},
			want: filepath.Join(wd, "instructions", "project_analyst"),
		},
		{
			name: "project_manager override",
			config: Config{
				Role:                       RoleProjectManager,
				InstructionsRoot:           "instructions",
				InstructionsProjectManager: "/opt/pma/pm",
			},
			want: "/opt/pma/pm",
		},
		{
			name: "project_analyst override",
			config: Config{
				Role:                       RoleProjectAnalyst,
				InstructionsRoot:           "instructions",
				InstructionsProjectAnalyst: "/opt/pma/pa",
			},
			want: "/opt/pma/pa",
		},
		{
			name: "empty root uses default",
			config: Config{
				Role:             RoleProjectManager,
				InstructionsRoot: "",
			},
			want: filepath.Join(wd, DefaultInstructionsRoot, "project_manager"),
		},
		{
			name: "unknown role defaults to project_manager",
			config: Config{
				Role:             Role("other"),
				InstructionsRoot: "inst",
			},
			want: filepath.Join(wd, "inst", "project_manager"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.InstructionsPath()
			if got != tt.want {
				t.Errorf("InstructionsPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
