# Changelog

## Development

- Test scripts now enforce a per-module timeout (default 3 minutes, override
  with TEST_TIMEOUT env variable).
- Initial project scaffold: Go workspace, module structure, Makefile
- Protobuf definitions for VaultProvider and LocksmithService
- Logging package using zerolog (stdout/JSON, configurable level)
- Configuration package with YAML loading, validation, path expansion
- Session manager with TTL, per-session secret cache, memory wipe on invalidation
- SDK package for vault plugin authors
- Plugin manager with process-based isolation via hashicorp/go-plugin
- Daemon gRPC server (LocksmithService) over Unix socket
- CLI commands: serve, get, session, vault, config, init
- gopass vault plugin
- macOS Keychain vault plugin with Touch ID (CGo, Security framework)
- `locksmith init` with TUI forms (charmbracelet/huh), agent auto-detection
- Agent integration: Claude Code, Codex
- Sandbox allowlist generation for agent restricted modes
- Documentation: README, architecture, configuration, plugins guide
- Integration test for full session lifecycle (build tag: integration)
- SDK module renamed to `github.com/lorem-dev/locksmith/sdk`
- buf linter added; vault.proto naming warnings fixed (VaultProviderService, InfoResponse)
- Makefile: pinned tool versions (buf, protoc-gen-go, protoc-gen-go-grpc, golangci-lint) with install-tools target
- File-based logging: add `logging.file` config option; logs are rotated at
  50 MB and retained for 3 days via lumberjack; if unset, logs go to stdout.
- Session IDs are masked in daemon log output unless `logging.level: debug`
  is active; masking applies only to log call sites, not RPC responses or CLI output.
- SDK: `HideSession` renamed to `HideSessionId` (breaking change for plugin authors).
- SDK: new `MaskSessionId` function for log call sites; returns full ID in debug mode.
- Docs: new `docs/security/debug-logging.md` - security implications of debug mode.
- Shell autostart hook: the locksmith daemon starts automatically when a
  terminal opens via a one-line snippet added to ~/.zshrc, ~/.bashrc, or
  Fish config. `locksmith init` offers to set this up; it can also be added
  manually. See docs/configuration.md.
