// Command gen-nats-dev-jwt writes the local dev NATS JWT bundle (operator + accounts) to disk.
// Used by setup-dev; default output: XDG cache (see natsjwt.DefaultDevJWTBundleDir).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/natsjwt"
)

func main() {
	out := flag.String("dir", "", "output directory (default: XDG cache cynodeai/nats-dev-jwt)")
	flag.Parse()
	dir := *out
	if dir == "" {
		dir = natsjwt.DefaultDevJWTBundleDir()
	}
	if err := natsjwt.WriteDevJWTBundle(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(dir)
}
