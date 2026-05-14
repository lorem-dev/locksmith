# Changelog

## Development

- `locksmith init` interactive vault selection now hides planned (unimplemented)
  vault backends from the multi-select list and shows them as a description note
  instead. Only vaults with `Implemented: true` appear as selectable options.
- `locksmith init --auto` no longer selects vault backends that have no
  working plugin (1password, gnome-keyring). A new `Implemented bool` field
  on `DetectedVault` distinguishes detection from plugin availability; only
  vaults with both `Detected` and `Implemented` true are auto-selected.
- `locksmith mcp run` no longer contacts the vault at startup. The
  first `GetSecret` call for a given MCP server fires on the first
  MCP request from the AI client, so unused MCP servers never trigger
  a Touch ID or GPG prompt. Local mode now stays resident as the
  parent of the MCP server child (process tree:
  `client -> locksmith -> child`, +5-10 MB RAM per server) and
  forwards SIGTERM/SIGINT to the child for graceful shutdown.
- `GRPCFetcher` recovers from `LOCKSMITH_SESSION` expiry between
  `mcp run` startup and the first lazy fetch: on an `invalid session`
  error it calls a `RefreshSession` closure (backed by `SessionStart`
  on the daemon), updates the session ID under a mutex, and retries
  the request once. Non-expiry errors propagate unchanged.
- Proxy mode (`locksmith mcp run --url`) no longer sends vault-resolved
  HTTP headers on the very first request. Static headers (those whose
  value template contains no `{` token) are sent from the start;
  templated headers are resolved and attached only after the remote
  server responds `401` or `403`. Subsequent requests reuse the
  resolved headers for the lifetime of the proxy session. Servers
  that authorise the MCP `initialize` handshake without auth never
  trigger a vault prompt for that connection. Known limitation: when
  auth resolves late while a long-lived SSE GET stream is already
  open, the GET is not reopened - operators of such servers should
  set `--transport http`.

## Version v0.2.0 - 2026-05-13

Locksmith v0.2.0 introduces a first-class MCP server wrapper so AI
clients can inject secrets into Model Context Protocol servers without
relying on shell substitution. It also tightens diagnostics for the new
wrapper and refreshes the configuration documentation.

### BREAKING CHANGES

- **MCP integration format changed.** The previously documented
  `$(locksmith get --key X)` shell-substitution pattern inside
  `mcp.json` is removed - MCP clients read `mcp.json` as plain JSON and
  never evaluated the expression. Replace every such entry with a
  `locksmith mcp run` invocation:
  - Local MCP server: `locksmith mcp run --env VAR=alias -- <command>`
  - Remote MCP server: `locksmith mcp run --url <URL> --header "Name=Bearer {key:alias}"`
  - From `config.yaml`: `locksmith mcp run --server <name>`

  See the rewritten MCP Integration section in `README.md` and the new
  `MCP servers` section in `docs/configuration.md` for full examples.
  The agent-session bootstrap (`$(locksmith session ensure --quiet)`
  in shell hooks) is unchanged and still in use.

### MCP server wrapper

- New `locksmith mcp run` subcommand resolves secrets from the daemon
  at startup and dispatches to one of two modes:
  - **Local mode** (`-- command args...`): execs the subprocess with
    resolved secrets injected as environment variables; stdin, stdout,
    and stderr pass through unchanged.
  - **Proxy mode** (`--url URL`): stdio<->HTTP proxy. Each JSON-RPC
    line from stdin is forwarded as a request and server responses are
    written back to stdout. Secrets are injected into HTTP headers via
    `{key:alias}` and `{vault:name path:value}` templates.
- Streamable HTTP transport (per MCP spec 2025-03-26) plus the legacy
  SSE transport. `--transport auto` (default) tries Streamable HTTP
  first and falls back to SSE on `404`/`405` responses.
- New `mcp.servers` section in `config.yaml` defines named server
  entries consumable via `locksmith mcp run --server <name>`. Env
  values accept either a key alias string or a `{vault, path}` struct;
  headers accept the same template tokens as `--header`.
- Diagnostics emitted via the shared zerolog logger at the level
  configured by `logging.level`. Output is forced to stderr so it
  cannot corrupt the MCP JSON-RPC channel on stdout. Any `userinfo`
  segment in URLs is masked via `url.URL.Redacted()` so credentials
  accidentally inlined into `--url` do not appear in logs or error
  messages.

### Documentation

- `README.md` MCP section rewritten to use `locksmith mcp run`; the
  minimal configuration example now includes an `mcp.servers` block
  alongside vaults and keys.
- New `MCP servers` reference section in `docs/configuration.md`
  documents every `mcp.servers.<name>` field.
- `docs/architecture.md` adds an `MCP wrapper` component description
  covering local mode, proxy mode, transport auto-detect, and the
  stderr-only logging contract.
- Install and verification examples in `docs/install.md` and
  `docs/verification.md` pinned to `v0.2.0`.

### Dependencies

- Added `github.com/stretchr/testify v1.7.2` as a direct dependency
  (MIT). It was already pulled in transitively; the move makes the
  intent explicit. `LICENSE` `## Third-Party Notices` is updated
  accordingly.

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
