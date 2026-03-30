// Package containerps interprets container runtime `ps` output (e.g. podman ps --format {{.Names}}).
package containerps

import "strings"

// NameListed reports whether psOutput lists name as an exact line. One container name is emitted
// per line; substring/prefix matches are rejected so "cynodeai-managed-pma" does not match
// "cynodeai-managed-pma-test".
func NameListed(psOutput, name string) bool {
	for _, line := range strings.Split(psOutput, "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}
