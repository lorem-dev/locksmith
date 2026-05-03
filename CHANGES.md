# Changelog

## Development

- The gopass plugin `Info()` response now declares `MinLocksmithVersion` ("0.1.0") and `MaxLocksmithVersion` (from `sdk/version.Current`), enabling the daemon's CompatValidator to enforce version range checks for this plugin.

- The daemon gRPC server now surfaces plugin compatibility warnings in `VaultHealth`
  responses (`compat_warnings` field) and uses the cached `InfoResponse` from
  plugin launch in `VaultList` (live RPC fallback when no cache is available).

- Added `internal/semver` package: minimal major.minor.patch semver parser used by CompatValidator.

- Added `sdk/version` package exposing `version.Current` - the authoritative locksmith version string, overridable at link time via `-ldflags`.
- Added `verification` skill and `.scripts/verification.sh`: a single `make verify`
  command now runs all quality gates (lint, race tests, coverage >= 90% per package,
  build, GPG signature check on branch commits, docs-completeness check, CHANGES.md
  check) and produces actionable output for any failures. The `verification` skill
  wraps the script, interprets failures gate-by-gate, and offers to run the
  `changelog` skill when five or more Development entries have accumulated. CLAUDE.md
  updated to make verification the mandatory last task in every superpowers plan.

- Hot-reload config: the daemon now picks up changes to `config.yaml` without
  restarting. Changes are applied via SIGHUP, `locksmith reload` CLI command, or
  automatically when the file is saved (1-second debounce via fsnotify). Active
  sessions and their secret caches are preserved; an invalid config is rejected and
  the previous config remains active. Plugin processes are delta-synced on reload:
  new vault types are launched, removed ones are killed cleanly with no zombie
  processes. Config is stored in `atomic.Pointer[config.Config]` for lock-free reads
  on the hot path.

- Updated linting to golangci-lint v2 (v2.11.4) with stricter settings: `errcheck`
  with `check-blank` and `check-type-assertions`, `wrapcheck`, `unparam`, `mnd`,
  `embeddedstructfieldcheck`, `gocritic`, `errorlint`, and others. Fixed all
  resulting issues across the codebase: proper error wrapping, named return
  values for defer-captured errors, removed unused nolint directives, renamed
  variables to avoid shadowing, and extracted helper functions to reduce
  cognitive complexity.

- `init` now automatically installs the Claude Code `UserPromptSubmit` hook
  into `~/.claude/settings.json` when Claude Code is detected; prompts for
  confirmation (skipped in `--auto` mode), merges idempotently without
  touching existing settings, and reminds to restart Claude Code after
  installation.
- Hook script and agent instruction templates moved from `docs/hooks/` into
  the embedded template set (`internal/initflow/templates/`); `docs/hooks/`
  removed - the binary is now the single source of truth.
- All agent templates updated with `--path/--vault` get syntax and two
  examples per form, covering both alias-based and direct-path access.
- Refactored `sdk/` into subpackages: `sdk/vault`, `sdk/errors`, `sdk/log`,
  `sdk/session`, `sdk/platform`. The old flat `sdk` package is removed; all
  consumers updated to use the new import paths.
- Added `sdk/platform` constants (`platform.Darwin`, `platform.Linux`) used by
  plugins and initflow instead of bare string literals.
- Added `internal/config` vault-type constants (`config.VaultKeychain`, etc.)
  and updated initflow and config validation to use them.
- Moved pinentry logic (`assuan.go`, `ui*.go`) from `cmd/locksmith-pinentry/` to
  `internal/pinentry/`; `cmd/locksmith-pinentry/main.go` is now a one-liner.
- Renamed all CLI command files in `internal/cli/` to `<command>_cmd.go` and
  split `coverage_test.go` into per-command `_cmd_test.go` files.
- Added `TestAutostart_ZombieReaping` to verify that short-lived child processes
  spawned by `_autostart` are reaped and do not become zombies.
- Added `make tidy` to run `go mod tidy` across all workspace modules via `.scripts/tidy/main.go`.
- Fixed zombie process leak in `_autostart`: added `go c.Wait()` goroutine
  so the spawned `serve` child is reaped if it exits before the parent.
  Also isolated `HOME` in autostart tests to prevent long-lived daemon
  processes from accumulating across test runs.

- Added `locksmith session ensure` command: reuses an existing valid session
  from `LOCKSMITH_SESSION` or starts a new one; `--quiet` flag for use in hook
  scripts without extra output.
- Added `agent.pass_session_to_subagents` config field (default: `true`) to
  control whether agents pass their session to child agents they spawn.
- Added `docs/agent-integration.md`: universal session management protocol for
  all supported agent platforms (Claude Code, Gemini CLI, Cursor, Copilot,
  Codex).
- Added `docs/hooks/locksmith-session.sh`: Claude Code `UserPromptSubmit` hook
  that automatically injects `LOCKSMITH_SESSION` before each prompt.
- Added platform adapter templates: `docs/hooks/AGENTS.md` (Cursor/Copilot/
  Codex) and `docs/hooks/GEMINI.md` (Gemini CLI).
- LICENSE: added third-party notices for buf and golangci-lint (development
  tools only, not distributed with Locksmith).
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
