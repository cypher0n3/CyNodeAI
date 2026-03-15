//go:build !goexperiment.runtimesecret

// Package secretutil provides shared utilities for handling secret-bearing data.
// This file is the fallback when runtime/secret is not available; callers must rely
// on best-effort secure erasure (e.g. zeroing buffers) per REQ-STANDS-0133.
package secretutil

// RunWithSecret runs f without runtime/secret; zeroBytes and similar remain the fallback.
func RunWithSecret(f func()) {
	f()
}
