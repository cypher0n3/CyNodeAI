//go:build !goexperiment.runtimesecret

// Package securestore fallback when runtime/secret is not available: run f normally;
// callers must rely on best-effort secure erasure (e.g. zeroBytes) per spec.
package securestore

// runWithSecret runs f without runtime/secret; zeroBytes and similar remain the fallback.
func runWithSecret(f func()) {
	f()
}
