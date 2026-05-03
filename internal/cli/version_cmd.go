// internal/cli/version_cmd.go
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	sdkversion "github.com/lorem-dev/locksmith/sdk/version"
)

// newVersionCmd builds the `locksmith version` command. It prints the
// embedded version (from sdk/version/VERSION via //go:embed) followed
// by a newline.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the locksmith version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), sdkversion.Current)
			if err != nil {
				return fmt.Errorf("writing version: %w", err)
			}
			return nil
		},
	}
}
