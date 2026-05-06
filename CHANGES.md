# Changelog

## Development

## Version v0.1.0 - 2026-05-07

Locksmith v0.1.0 is the first public release. It ships the secret-management
daemon, CLI, two reference vault plugins, agent integrations for the major
AI clients, and the install/release pipeline.

### Daemon & CLI

- gRPC daemon over a local Unix socket; CLI subcommands `init`, `serve`,
  `get`, `session`, `vault`, `config`, `reload`, `version`, and
  `plugins update`.
- Session manager with TTL, per-session secret cache, and memory wipe on
  invalidation. Session IDs are masked in daemon log output unless
  `logging.level: debug` is set; see `docs/security/debug-logging.md` for
  the security implications of debug mode.
- Structured logging via zerolog (text or JSON, configurable level);
  optional `logging.file` enables file output with 50 MB rotation and
  3-day retention via lumberjack.

### Vault plugins

- Reference plugins for gopass and macOS Keychain (Touch ID via the
  Security framework + CGo). Plugins run as separate processes via
  `hashicorp/go-plugin`.
- Default plugins and `locksmith-pinentry` ship bundled inside the
  `locksmith` binary as a per-platform `//go:embed` zip and are extracted
  by `locksmith init` to `~/.config/locksmith/plugins/` and
  `~/.config/locksmith/bin/locksmith-pinentry`. Conflict policy: sha256
  match -> silent skip; mismatch -> interactive prompt
  (Overwrite / Keep / Overwrite all / Keep all). `locksmith plugins update
  [--dry-run] [--force]` re-extracts for the vaults declared in
  `config.yaml`.
- Plugin launches run a soft compatibility check (platform support,
  locksmith min/max version range); warnings surface via `locksmith vault
  health` (`compat_warnings`) and never block a plugin from running.
- Per-plugin documentation: `plugins/gopass/README.md`,
  `plugins/keychain/README.md`.

### Agent integration

- Universal session-management protocol documented in
  `docs/agent-integration.md`. `locksmith session ensure --quiet` reuses
  or starts a session for hook scripts.
- Embedded templates for Claude Code (`UserPromptSubmit` hook installed
  into `~/.claude/settings.json` by `locksmith init`),
  Cursor / Copilot / Codex (`AGENTS.md`), and Gemini CLI (`GEMINI.md`).
- `agent.pass_session_to_subagents` config (default `true`) controls
  session inheritance to child agents.

### Configuration & operations

- YAML config at `~/.config/locksmith/config.yaml` with validation,
  path expansion, and per-vault sections; full reference in
  `docs/configuration.md`.
- Hot-reload: the daemon picks up `config.yaml` changes via SIGHUP,
  `locksmith reload`, or an automatic file-watcher (1-second debounce
  via fsnotify). Active sessions and secret caches are preserved; an
  invalid config is rejected and the previous config remains active.
  Plugin processes are delta-synced (new types launched, removed ones
  killed cleanly with no zombies).
- Shell autostart hook for zsh, bash, and fish; `locksmith init` offers
  to install it. Spawned `serve` children are reaped to prevent
  zombies.

### Release & CI/CD

- GitHub Actions: `ci.yml` runs lint, race tests, and coverage on
  ubuntu-latest and macos-latest for every push and PR; `release.yml`
  cross-builds for linux/amd64, linux/arm64, and darwin/arm64 on tag
  push and publishes a GitHub release with per-platform zip archives,
  SHA-256 checksums, a GPG-signed `checksums.txt.asc`, and a generated
  POSIX `install.sh`. Intel Macs (darwin/amd64) are not in the prebuilt
  set for v0.1.0; on those systems the install script prints
  `go install` instructions so users can opt in to a from-source build.
- Install path: `curl -fsSL https://github.com/lorem-dev/locksmith/releases/latest/download/install.sh | sh`
  with `LOCKSMITH_VERSION` pinning and `LOCKSMITH_INSTALL_DIR` override.
  Long-form install, GPG verification, build-from-source, and the
  `go install` fallback in `docs/install.md`; maintainer release
  procedure in `docs/release.md`.
- Version is embedded from `sdk/version/VERSION` (with a `VERSION`
  symlink at the repo root) and surfaced by `locksmith version`.
  `make check-version` keeps the pushed tag, VERSION file, and
  `CHANGES.md` heading aligned at tag-build time.
- Build and lint stack pinned via `make install-tools`: golangci-lint
  v2.11.4 with a strict ruleset (errcheck, wrapcheck, gocritic,
  errorlint, mnd, etc.) and the buf protobuf linter.

### SDK for plugin authors

- Public SDK at `github.com/lorem-dev/locksmith/sdk`, organised into
  subpackages: `sdk/vault` (Provider interface, `Serve()`),
  `sdk/errors` (typed `VaultError`), `sdk/log` (`LogConfig`,
  `NewLogWriter`, `IsDebug`), `sdk/session` (session-id helpers,
  `MaskSessionId`), and `sdk/platform` (`Darwin` / `Linux` constants).
- The SDK surface is at v0.1.0; expect refinements before v1.0.
