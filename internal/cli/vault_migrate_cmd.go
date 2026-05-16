package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/config"
)

// newVaultMigrateCmd returns the `locksmith vault migrate` command. It
// re-stores existing macOS Keychain items with biometric access
// control by reading them via the current ACL (password prompt one
// last time) and writing them back with kSecAccessControlUserPresence.
// Keychain-only; gopass aliases get a friendly "not relevant" error.
func newVaultMigrateCmd(cfgFile *string) *cobra.Command {
	var (
		all    bool
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "migrate [<alias>]",
		Short: "Re-store macOS Keychain items with Touch ID ACL",
		Long: `Migrate existing keychain items so subsequent reads prompt for Touch ID
instead of a password. Reads each item via the current access control
(one last password prompt), then re-stores it with biometry ACL.

Use --all to migrate every keychain-typed key in config.yaml.
Use --dry-run to list what would be migrated without writing.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVaultMigrate(cmd, args, cfgFile, all, dryRun)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "migrate every keychain-typed key in config")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list what would be migrated without writing")
	return cmd
}

// runVaultMigrate is the RunE body for `vault migrate`, factored out
// to keep cognitive complexity manageable.
func runVaultMigrate(cmd *cobra.Command, args []string, cfgFile *string, all, dryRun bool) error {
	if !all && len(args) == 0 {
		return fmt.Errorf("migrate requires an alias or --all")
	}

	cfg, err := loadMigrateConfig(cfgFile)
	if err != nil {
		return err
	}

	targets, err := resolveMigrateTargets(cfg, args, all)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no keychain entries to migrate") //nolint:errcheck // writing to stdout
		return nil
	}

	if dryRun {
		for _, alias := range targets {
			fmt.Fprintf(cmd.OutOrStdout(), "would migrate %s\n", alias) //nolint:errcheck // writing to stdout
		}
		return nil
	}

	return performMigrate(cmd, targets)
}

// loadMigrateConfig loads config.yaml from --config or the default
// path.
func loadMigrateConfig(cfgFile *string) (*config.Config, error) {
	cfgPath := ""
	if cfgFile != nil {
		cfgPath = *cfgFile
	}
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

// resolveMigrateTargets returns the list of aliases to migrate based
// on --all or a single positional alias. Non-keychain entries are
// rejected for the single-alias form and skipped for --all.
func resolveMigrateTargets(cfg *config.Config, args []string, all bool) ([]string, error) {
	if all {
		var targets []string
		for alias, key := range cfg.Keys {
			v, ok := cfg.Vaults[key.Vault]
			if !ok || v.Type != config.VaultKeychain {
				continue
			}
			targets = append(targets, alias)
		}
		return targets, nil
	}
	alias := args[0]
	key, ok := cfg.Keys[alias]
	if !ok {
		return nil, fmt.Errorf("unknown alias: %s", alias)
	}
	v, ok := cfg.Vaults[key.Vault]
	if !ok {
		return nil, fmt.Errorf("alias %s references unknown vault %s", alias, key.Vault)
	}
	if v.Type != config.VaultKeychain {
		return nil, fmt.Errorf("migrate is only relevant for keychain vaults; %s uses %s", alias, v.Type)
	}
	return []string{alias}, nil
}

// performMigrate dials the daemon and re-stores each target alias.
func performMigrate(cmd *cobra.Command, targets []string) error {
	client, conn, err := dialDaemon()
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // gRPC connection close; error not actionable in defer

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	sess, err := ensureSession(ctx, client)
	if err != nil {
		return err
	}

	var done int
	var failures []string
	for _, alias := range targets {
		if ferr := migrateOne(ctx, cmd, client, sess, alias); ferr != "" {
			failures = append(failures, ferr)
			continue
		}
		done++
		fmt.Fprintf(cmd.OutOrStdout(), "migrated %s\n", alias) //nolint:errcheck // writing to stdout
	}

	summary := fmt.Sprintf("migrated %d/%d keychain entries", done, len(targets))
	if len(failures) > 0 {
		summary += fmt.Sprintf(" (%d failed)", len(failures))
	}
	fmt.Fprintln(cmd.OutOrStdout(), summary) //nolint:errcheck // writing to stdout
	for _, f := range failures {
		fmt.Fprintln(cmd.OutOrStdout(), "  "+f) //nolint:errcheck // writing to stdout
	}
	if len(failures) > 0 && done == 0 {
		// Total failure: return non-nil so exit code is non-zero.
		return fmt.Errorf("migrate failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

// migrateOne reads then re-stores a single alias. Returns "" on
// success or a human-readable failure string.
func migrateOne(
	ctx context.Context,
	cmd *cobra.Command,
	client locksmithv1.LocksmithServiceClient,
	sess, alias string,
) string {
	_ = cmd // reserved for future progress reporting
	getResp, gerr := client.GetSecret(ctx, &locksmithv1.GetSecretRequest{
		SessionId: sess,
		KeyAlias:  alias,
	})
	if gerr != nil {
		return fmt.Sprintf("%s: read failed: %v", alias, gerr)
	}
	if _, serr := client.SetSecret(ctx, &locksmithv1.SetSecretRequest{
		SessionId: sess,
		KeyAlias:  alias,
		Secret:    getResp.Secret,
		Force:     true,
	}); serr != nil {
		return fmt.Sprintf("%s: write failed: %v", alias, serr)
	}
	return ""
}
