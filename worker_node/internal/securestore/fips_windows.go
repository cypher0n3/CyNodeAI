//go:build windows

package securestore

import (
	"golang.org/x/sys/windows/registry"
)

// FIPS algorithm policy key: Enabled = 1 means FIPS mode is on.
const (
	fipsPolicyKeyPath = `SYSTEM\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`
	fipsPolicyValue   = "Enabled"
)

func init() {
	getFIPSStatusPlatform = getFIPSStatusWindows
}

func getFIPSStatusWindows() fipsStatus {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, fipsPolicyKeyPath, registry.QUERY_VALUE)
	if err != nil {
		// Key missing or access denied: unknown → fail closed.
		return fipsUnknown
	}
	defer k.Close()
	val, _, err := k.GetIntegerValue(fipsPolicyValue)
	if err != nil {
		// Value missing or error: unknown → fail closed.
		return fipsUnknown
	}
	if val == 1 {
		return fipsOn
	}
	return fipsOff
}
