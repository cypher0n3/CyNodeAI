//go:build goexperiment.runtimesecret

// Package securestore uses Go 1.26 runtime/secret when built with GOEXPERIMENT=runtimesecret
// so that temporaries for master key and decrypted plaintext are erased (primary path per spec).
package securestore

import "runtime/secret"

// runWithSecret runs f with runtime/secret protection so that temporaries used by f
// (stack, registers, and eventually heap from f's call tree) are erased before return.
// Use for any code that handles the master key or decrypted plaintext.
func runWithSecret(f func()) {
	secret.Do(f)
}
