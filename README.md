# Locksmith

Secure middleware that gives AI agents access to secrets stored in vault providers
(macOS Keychain, gopass), with per-session caching and
vault-delegated authorization (Touch ID, GPG passphrase).

**Repository:** https://github.com/lorem-dev/locksmith

## Installation

```bash
go install github.com/lorem-dev/locksmith/cmd/locksmith@latest
```

Or build from source:

```bash
git clone https://github.com/lorem-dev/locksmith
cd locksmith
make init        # install tools and generate protobuf code (required after clone)
make build-all
```

After install, `locksmith init` extracts the default vault plugins and
`locksmith-pinentry` from the embedded bundle into `~/.config/locksmith/`.
See [PLUGINS.md](PLUGINS.md).

## Quick Start

```bash
# Print the installed version (embedded from sdk/version/VERSION)
locksmith version

# 1. Initialize (interactive TUI)
locksmith init

# 2. Start the daemon
locksmith serve &

# 3. Retrieve a secret - a session starts automatically on first use
locksmith get --key my-api-key

# Optional: start a session explicitly to reuse it across calls
# (avoids repeated vault authorization for the same task)
export LOCKSMITH_SESSION=$(locksmith session start | jq -r .session_id)
# Sessions expire automatically by TTL - no need to end them manually
```

The version is read from `sdk/version/VERSION` and embedded into the
binary at build time via `//go:embed`. See [CONTRIBUTING.md](CONTRIBUTING.md#version-bumps)
for the release process.

## Agent Usage

Locksmith provides a session-aware CLI for AI agents. See
[Agent Integration](docs/agent-integration.md) for the full protocol and
platform-specific setup (Claude Code hooks, Gemini CLI, Cursor, Copilot,
Codex).

For Claude Code, run `locksmith init` - the `UserPromptSubmit` hook is
installed automatically and injects `LOCKSMITH_SESSION` before each prompt.
Restart Claude Code after running `init`.

**Quick start:**

```bash
# Ensure a valid session (reuses existing or creates new)
export LOCKSMITH_SESSION=$(locksmith session ensure --quiet)

# Retrieve a secret by alias (configured in config.yaml)
locksmith get --key my-api-key

# Retrieve a secret directly by vault path (no alias needed)
locksmith get --vault gopass --path work/aws/access-key-id

# Sub-agents: pass the session in their environment
LOCKSMITH_SESSION=$LOCKSMITH_SESSION some-subagent-tool
```

Session delegation to sub-agents is controlled by `agent.pass_session_to_subagents`
in `~/.config/locksmith/config.yaml` (default: `true`).

## MCP Integration

In your MCP server config:

```json
{
  "mcpServers": {
    "my-server": {
      "url": "https://api.example.com",
      "headers": {
        "Authorization": "Bearer $(locksmith get --key my-api-key)"
      }
    }
  }
}
```

### Reloading configuration

Apply config changes (new vaults, keys, defaults) without restarting the daemon:

```bash
# Via CLI command
locksmith reload

# Via UNIX signal
kill -HUP $(pgrep locksmith)
```

The daemon also watches `~/.config/locksmith/config.yaml` automatically and reloads
within one second of a detected change - no manual action needed.

Active sessions and their secret caches are preserved across reloads. If the new
config file is invalid, the daemon keeps the previous configuration and logs an error.

## Supported Vaults

| Vault | Platform | Auth | Status |
|-------|----------|------|--------|
| macOS Keychain | macOS | Touch ID / password | Supported |
| gopass | macOS, Linux | GPG passphrase / Touch ID | Supported |
| 1Password | macOS, Linux | Touch ID / master password | Planned |
| GNOME Keyring | Linux | Keyring password | Planned |

### Plugin Setup Guides

Per-plugin installation, configuration examples, and troubleshooting:

| Plugin   | Platform     | Setup guide                                              |
|----------|--------------|----------------------------------------------------------|
| gopass   | Linux, macOS | [`plugins/gopass/README.md`](plugins/gopass/README.md)   |
| keychain | macOS only   | [`plugins/keychain/README.md`](plugins/keychain/README.md) |

## Documentation

- [Architecture](docs/architecture.md)
- [Configuration Reference](docs/configuration.md)
- [Agent Integration](docs/agent-integration.md)
- [Plugins](docs/plugins/README.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
