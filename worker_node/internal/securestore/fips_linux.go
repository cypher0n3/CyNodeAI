//go:build linux

package securestore

import (
	"os"
	"strings"
)

const fipsEnabledPath = "/proc/sys/crypto/fips_enabled"

// fipsEnabledPathOverride is set by tests to inject a path (so getFIPSStatusLinux can be covered without reading /proc).
var fipsEnabledPathOverride string

func init() {
	getFIPSStatusPlatform = getFIPSStatusLinux
}

func getFIPSStatusLinux() fipsStatus {
	path := fipsEnabledPath
	if fipsEnabledPathOverride != "" {
		path = fipsEnabledPathOverride
	}
	b, err := os.ReadFile(path)
	if err != nil {
		// File missing (old kernel): not FIPS. Any other error: unknown → fail closed.
		if os.IsNotExist(err) {
			return fipsOff
		}
		return fipsUnknown
	}
	if strings.TrimSpace(string(b)) == "1" {
		return fipsOn
	}
	return fipsOff
}
