# Changelog

## Development

- Release tooling: canonical version moved to `sdk/version/VERSION`
  (with a `VERSION` symlink in the repo root) and embedded into
  `sdk/version.Current` via `//go:embed`; new `locksmith version`
  command prints the embedded value; new `make check-version`
  (`.scripts/check-version`) verifies tag-VERSION-CHANGES.md
  alignment on CI tag builds (GitHub Actions and GitLab CI);
  `CONTRIBUTING.md` documents the bump procedure and CI YAML
  snippets; new committed `version-bump` skill orchestrates VERSION
  update plus a `changelog`-skill invocation, optionally invoking
  `check-licenses` first.

- Bundled default plugins (`gopass`, `keychain`) and `locksmith-pinentry`
  inside the `locksmith` binary as a per-platform `//go:embed` zip;
  `locksmith init` extracts the plugins matching chosen vault types to
  `~/.config/locksmith/plugins/` and pinentry to
  `~/.config/locksmith/bin/locksmith-pinentry`. Conflict policy: sha256
  match -> silent skip; mismatch -> `y/n/all/skip` prompt; "keep" emits a
  warning. New `locksmith plugins update [--dry-run] [--force]` re-extracts
  for the vaults declared in `config.yaml`. Plugin version is locked to
  the host `locksmith` version (no network, no registry). Documentation
  decomposed: `docs/plugins.md` is split into `docs/plugins/{README,
  architecture,authoring,compatibility}.md`; new root `PLUGINS.md` is the
  canonical short overview; `CLAUDE.md` gains a "Bundled Plugins" rule.

- Added per-plugin `README.md` for `plugins/gopass` and `plugins/keychain`
  covering installation, configuration examples, and troubleshooting; the
  canonical YAML schema stays in `docs/configuration.md` and per-plugin
  READMEs are cross-linked from `docs/configuration.md`, `docs/plugins.md`,
  the root `README.md`, and `CLAUDE.md` (new "Plugin Documentation" rule).

- Plugin launches now run a soft compatibility check (platform support,
  locksmith min/max version range, `Info()` availability); warnings are
  surfaced via `locksmith vault health` (`compat_warnings` field) and never
  block a plugin from running. Added `internal/semver` (minimal
  major.minor.patch parser), `sdk/version` exposing `version.Current`
  (overridable via `-ldflags`), and the gopass plugin declares
  `MinLocksmithVersion`/`MaxLocksmithVersion` in `Info()`.

- Added `verification` skill and `.scripts/verification.sh`: a single
  `make verify` runs all quality gates (lint, race tests, coverage >= 90%
  per package, build, GPG signatures on branch commits, docs completeness,
  `CHANGES.md` entry); the skill interprets failures gate-by-gate and
  offers the `changelog` skill once five or more Development entries have
  accumulated. CLAUDE.md updated to make verification the mandatory final
  task in every superpowers plan.

- Hot-reload config: the daemon picks up `config.yaml` changes without
  restarting via SIGHUP, `locksmith reload`, or automatic file-watcher
  (1-second debounce via fsnotify). Active sessions and their secret
  caches are preserved; an invalid config is rejected and the previous
  config remains active. Plugin processes are delta-synced (new types
  launched, removed ones killed cleanly with no zombies). Config stored
  in `atomic.Pointer[config.Config]` for lock-free reads on the hot path.

- Build and lint stack: golangci-lint v2.11.4 with strict ruleset
  (`errcheck` with `check-blank`/`check-type-assertions`, `wrapcheck`,
  `unparam`, `mnd`, `embeddedstructfieldcheck`, `gocritic`, `errorlint`),
  buf linter for protobuf, all tool versions pinned via
  `make install-tools`, `make tidy` for workspace-wide `go mod tidy`,
  configurable per-module test timeout (`TEST_TIMEOUT`, default 3m), and
  third-party notices for buf and golangci-lint in LICENSE.

- Interactive `locksmith init` (charmbracelet/huh) with agent
  auto-detection, sandbox allowlist generation for restricted modes, and
  idempotent installation of the Claude Code `UserPromptSubmit` hook into
  `~/.claude/settings.json` (with `--auto` mode and post-install restart
  reminder). Embedded agent templates document both alias-based and
  direct-path (`--path/--vault`) syntax.

- Universal agent integration: session management protocol documented in
  `docs/agent-integration.md`; `locksmith session ensure --quiet` reuses
  or starts a session for hook scripts; `agent.pass_session_to_subagents`
  config (default `true`) controls session inheritance to child agents;
  embedded templates cover Claude Code (`UserPromptSubmit`),
  Cursor / Copilot / Codex (`AGENTS.md`), and Gemini CLI (`GEMINI.md`).

- Internal refactors: vault-type constants in `internal/config`, pinentry
  protocol/UI moved into `internal/pinentry/`, and all CLI command files
  renamed to `<command>_cmd.go` with per-command test files.

- Fixed zombie process leak in `_autostart` (a `go c.Wait()` goroutine
  reaps the spawned `serve` child if it exits before parent); added
  `TestAutostart_ZombieReaping` and isolated `HOME` in autostart tests
  to prevent long-lived daemons from accumulating across test runs.

- SDK for vault plugin authors at `github.com/lorem-dev/locksmith/sdk`,
  organized into subpackages: `sdk/vault` (Provider interface, `Serve()`),
  `sdk/errors` (typed `VaultError`), `sdk/log` (`LogConfig`,
  `NewLogWriter`, `IsDebug`), `sdk/session` (session-id helpers), and
  `sdk/platform` (`Darwin`/`Linux` constants used by plugins and
  initflow).

- Structured logging via zerolog (stdout or JSON, configurable level);
  optional `logging.file` config option enables file output with 50 MB
  rotation and 3-day retention via lumberjack.

- Session IDs are masked in daemon log output unless `logging.level: debug`
  is active (masking applies only to log call sites, not RPC responses or
  CLI output). SDK: `HideSession` renamed to `HideSessionId` (breaking
  change for plugin authors); added `MaskSessionId` for log call sites
  (returns full ID in debug mode). New `docs/security/debug-logging.md`
  documents the security implications of debug mode.

- Shell autostart hook: the locksmith daemon starts automatically when a
  terminal opens via a one-line snippet added to `~/.zshrc`, `~/.bashrc`,
  or Fish config; `locksmith init` offers to set this up and it can also
  be added manually.

- Core: Go workspace + Makefile + protobuf definitions (VaultProvider,
  LocksmithService); daemon gRPC server over Unix socket; CLI commands
  `serve`, `get`, `session`, `vault`, `config`, `init`; YAML config with
  validation and path expansion; session manager with TTL, per-session
  secret cache, and memory wipe on invalidation; plugin manager with
  process isolation via `hashicorp/go-plugin`; reference plugins for
  gopass and macOS Keychain (Touch ID via Security framework + CGo);
  integration test for the full session lifecycle.

- Documentation: README, architecture, configuration, plugins guide.
