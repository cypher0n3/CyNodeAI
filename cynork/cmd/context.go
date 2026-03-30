package cmd

import (
	"context"

	"github.com/spf13/cobra"
)

// cmdContext returns cmd.Context() when cmd is non-nil; otherwise context.Background().
// Unit tests invoke RunE handlers with a nil *cobra.Command.
func cmdContext(cmd *cobra.Command) context.Context {
	if cmd != nil {
		return cmd.Context()
	}
	return context.Background()
}
