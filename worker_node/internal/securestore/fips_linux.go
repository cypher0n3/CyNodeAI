//go:build linux

package securestore

import (
	"os"
	"strings"
)

const fipsEnabledPath = "/proc/sys/crypto/fips_enabled"

func init() {
	getFIPSStatusPlatform = getFIPSStatusLinux
}

func getFIPSStatusLinux() fipsStatus {
	path := fipsEnabledPath
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
