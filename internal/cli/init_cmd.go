package cli

import "github.com/spf13/cobra"

// newInitCmd returns a stub init command; full implementation is in Task 12.
func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactive setup for locksmith",
	}
}
