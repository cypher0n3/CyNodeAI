// Package pma provides configuration and runtime for the cynode-pma agent binary.
// See docs/tech_specs/cynode_pma.md.
package pma

import (
	"os"
	"path/filepath"
)

// Role is the agent role mode.
type Role string

const (
	RoleProjectManager Role = "project_manager"
	RoleProjectAnalyst Role = "project_analyst"
)

// Config holds cynode-pma configuration.
// Precedence: flag > config file > environment variable.
type Config struct {
	Role Role

	// InstructionsRoot is the root directory for role-specific instruction bundles.
	// Default: "instructions"
	InstructionsRoot string

	// InstructionsProjectManager overrides the default path for project_manager bundle.
	// Default: InstructionsRoot + "/project_manager"
	InstructionsProjectManager string

	// InstructionsProjectAnalyst overrides the default path for project_analyst bundle.
	// Default: InstructionsRoot + "/project_analyst"
	InstructionsProjectAnalyst string

	// ListenAddr is the HTTP listen address for the agent server.
	ListenAddr string
}

// DefaultInstructionsRoot is the default instructions root directory name.
const DefaultInstructionsRoot = "instructions"

// InstructionsPath returns the absolute path to the instructions bundle for the current role.
// Per CYNAI.PMAGNT.InstructionsLoading: project_manager -> instructions/project_manager,
// project_analyst -> instructions/project_analyst; overrides via config apply.
func (c *Config) InstructionsPath() string {
	root := c.InstructionsRoot
	if root == "" {
		root = DefaultInstructionsRoot
	}
	switch c.Role {
	case RoleProjectAnalyst:
		if c.InstructionsProjectAnalyst != "" {
			return absPath(c.InstructionsProjectAnalyst)
		}
		return absPath(filepath.Join(root, "project_analyst"))
	default:
		// RoleProjectManager or any other value
		if c.InstructionsProjectManager != "" {
			return absPath(c.InstructionsProjectManager)
		}
		return absPath(filepath.Join(root, "project_manager"))
	}
}

func absPath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, p)
}
