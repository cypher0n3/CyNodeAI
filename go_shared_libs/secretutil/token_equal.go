package secretutil

import "crypto/subtle"

// TokenEquals compares two secret strings for equality using constant-time comparison of UTF-8 bytes.
func TokenEquals(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
