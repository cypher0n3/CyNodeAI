//go:build goexperiment.runtimesecret

// Package secretutil provides shared utilities for handling secret-bearing data.
// This file uses Go 1.26 runtime/secret when built with GOEXPERIMENT=runtimesecret.
package secretutil

import "runtime/secret"

// RunWithSecret runs f with runtime/secret protection so that temporaries used by f
// (stack, registers, and eventually heap from f's call tree) are erased before return.
// Use for any code that handles secrets, credentials, or decrypted plaintext per REQ-STANDS-0133.
func RunWithSecret(f func()) {
	secret.Do(f)
}
