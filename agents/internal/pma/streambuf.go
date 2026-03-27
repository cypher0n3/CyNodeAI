package pma

import "github.com/cypher0n3/cynodeai/go_shared_libs/secretutil"

// appendStreamBufferSecure appends chunk to dst inside secretutil.RunWithSecret so transient
// secret-bearing buffers follow REQ-STANDS-0133 best-effort handling.
func appendStreamBufferSecure(dst *[]byte, chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	secretutil.RunWithSecret(func() {
		*dst = append(*dst, chunk...)
	})
}
