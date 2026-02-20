// Command cynork is the CyNodeAI CLI management client.
// See docs/tech_specs/cli_management_app.md.
package main

import (
	"os"

	"github.com/cypher0n3/cynodeai/cynork/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
