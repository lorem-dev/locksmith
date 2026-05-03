// internal/cli/plugins_cmd.go
package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/bundled"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/log"
)

// newPluginsCmd builds the `locksmith plugins` command group.
func newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Manage extracted vault plugins and pinentry",
	}
	cmd.AddCommand(newPluginsUpdateCmd())
	return cmd
}

func newPluginsUpdateCmd() *cobra.Command {
	var dryRun, force bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Re-extract bundled plugins and pinentry into ~/.config/locksmith/",
		Long: `Re-extracts the default plugins matching the vaults in your config.yaml,
plus locksmith-pinentry, from the bundle embedded in the locksmith binary.

By default, files that already match the bundled sha256 are silently skipped;
files with different content prompt for confirmation. --force overwrites
without prompting; --dry-run prints what would change without modifying disk.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginsUpdate(dryRun, force)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would change; do not modify disk")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files without prompting")
	return cmd
}

func runPluginsUpdate(dryRun, force bool) error {
	cfgPath := config.DefaultConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w (run `locksmith init` first)", err)
	}
	if len(cfg.Vaults) == 0 {
		fmt.Println("No vaults configured. Run `locksmith init` first.")
		return nil
	}
	bundle, err := bundled.OpenBundle()
	if err != nil {
		if errors.Is(err, bundled.ErrEmptyBundle) {
			fmt.Println("This build has no bundled plugins; nothing to update.")
			return nil
		}
		return fmt.Errorf("opening bundle: %w", err)
	}
	pluginsDir, err := bundled.PluginsDir()
	if err != nil {
		return err
	}
	pinentryPath, err := bundled.PinentryPath()
	if err != nil {
		return err
	}
	names := []string{"locksmith-pinentry"}
	for _, v := range cfg.Vaults {
		names = append(names, "locksmith-plugin-"+v.Type)
	}
	if dryRun {
		return printDiff(bundle, names, pluginsDir, pinentryPath)
	}
	return bundled.Extract(bundle, bundled.ExtractOptions{
		Names:          names,
		PluginsDir:     pluginsDir,
		PinentryPath:   pinentryPath,
		Prompter:       cliPromptOrNil(force),
		ForceOverwrite: force,
		OnKept: func(name string, withWarning bool) {
			if withWarning {
				log.Warn().Str("entry", name).
					Msg("kept; bundled version differs - functionality may not work as expected")
			}
		},
		OnExtracted: func(name string) {
			fmt.Printf("updated: %s\n", name)
		},
	})
}

func cliPromptOrNil(force bool) bundled.ExtractPrompter {
	if force {
		return nil // ForceOverwrite path skips prompter anyway
	}
	return &cliPrompter{}
}

// cliPrompter is a minimal stdin/stdout prompter used by `plugins update`.
// It does not depend on huh; the command is intended for non-interactive
// terminals as well.
type cliPrompter struct{}

func (cliPrompter) BundleExtractPrompt(name, existingSHA, newSHA string) (bundled.ConflictResolution, error) {
	fmt.Printf("Existing %s differs from bundled (disk %s vs bundled %s). [y]es/[n]o/[a]ll/[s]kip-all: ",
		name, bundled.ShortSHA(existingSHA), bundled.ShortSHA(newSHA))
	var ans string
	fmt.Scanln(&ans) //nolint:errcheck
	switch ans {
	case "y", "Y", "yes":
		return bundled.Overwrite, nil
	case "n", "N", "no":
		return bundled.Keep, nil
	case "a", "A", "all":
		return bundled.OverwriteAll, nil
	case "s", "S", "skip":
		return bundled.KeepAll, nil
	default:
		return bundled.Keep, nil
	}
}

// printDiff lists what would be updated without modifying the filesystem.
func printDiff(bundle *bundled.Bundle, names []string, pluginsDir, pinentryPath string) error {
	for _, name := range names {
		entry, ok := bundle.FindEntry(name)
		if !ok {
			continue // not in bundle (e.g. config requests a custom vault)
		}
		dest := pluginsDir + "/" + entry.Name
		if entry.Kind == bundled.KindPinentry {
			dest = pinentryPath
		}
		switch action, err := diffAction(dest, entry.SHA256); {
		case err != nil:
			return err
		case action == "missing":
			fmt.Printf("would install: %s\n", name)
		case action == "differ":
			fmt.Printf("would update:  %s\n", name)
		}
	}
	return nil
}

func diffAction(path, wantSHA string) (string, error) {
	got, exists, err := readSHA(path)
	if err != nil {
		return "", err
	}
	if !exists {
		return "missing", nil
	}
	if got == wantSHA {
		return "match", nil
	}
	return "differ", nil
}

func readSHA(path string) (string, bool, error) {
	return bundled.FileSHA256(path)
}
