# Changelog

## Development

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
