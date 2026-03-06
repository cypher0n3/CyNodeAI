// Package securestore: FIPS mode detection. When status is unknown we fail closed (treat as FIPS on).
package securestore

import (
	"os"
	"strings"
)

// FIPS status: on, off, or unknown. Unknown is treated as on (fail closed) for env fallback.
type fipsStatus int

const (
	fipsOff fipsStatus = iota
	fipsOn
	fipsUnknown
)

// testFIPSPath overrides the path read for FIPS mode in tests (Linux simulation).
var testFIPSPath string

// testFIPSModeKnownOff when true forces FIPS off so tests that use env fallback succeed without platform detection.
var testFIPSModeKnownOff bool

// envFIPSMode is the explicit override. "1"/"true"/"yes" = FIPS on, "0"/"false"/"no" = FIPS off.
const envFIPSMode = "CYNODE_FIPS_MODE"

// isFIPSMode reports whether the host is in FIPS mode or status is unknown (fail closed).
// When true, Open rejects env-based master key (CYNAI.WORKER.NodeLocalSecureStore).
func isFIPSMode() bool {
	return getFIPSStatus() != fipsOff
}

// getFIPSStatus returns FIPS status. Unknown is returned when we cannot determine (fail closed).
func getFIPSStatus() fipsStatus {
	if testFIPSModeKnownOff {
		return fipsOff
	}
	if testFIPSPath != "" {
		// Test-controlled file (e.g. content "1" or "0").
		b, err := os.ReadFile(testFIPSPath)
		if err != nil {
			return fipsUnknown
		}
		if strings.TrimSpace(string(b)) == "1" {
			return fipsOn
		}
		return fipsOff
	}
	if v := strings.TrimSpace(os.Getenv(envFIPSMode)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes":
			return fipsOn
		case "0", "false", "no":
			return fipsOff
		}
		// Invalid value: treat as unknown (fail closed).
		return fipsUnknown
	}
	return getFIPSStatusPlatform()
}

// getFIPSStatusPlatform is set by build-tagged init (fips_linux.go, fips_windows.go). Default: unknown (fail closed).
var getFIPSStatusPlatform = func() fipsStatus { return fipsUnknown }
