# Locksmith

Secure middleware that gives AI agents access to secrets stored in vault providers
(macOS Keychain, gopass, 1Password, GNOME Keyring), with per-session caching and
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
make proto       # generate protobuf code (required after clone)
make build-all
```

## Quick Start

```bash
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

## Agent Usage

Agents call `locksmith get --key <alias>` directly. If `LOCKSMITH_SESSION` is not
set, a session is started automatically. To share a session with sub-agents, export
the session ID printed to stderr:

```bash
export LOCKSMITH_SESSION=$(locksmith session start | jq -r .session_id)
# pass LOCKSMITH_SESSION to sub-agents via environment
```

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

## Supported Vaults

| Vault | Platform | Auth |
|-------|----------|------|
| macOS Keychain | macOS | Touch ID / password |
| gopass | macOS, Linux | GPG passphrase / Touch ID |
| 1Password | macOS, Linux | Touch ID / master password |
| GNOME Keyring | Linux | Keyring password |

## Documentation

- [Architecture](docs/architecture.md)
- [Configuration Reference](docs/configuration.md)
- [Writing Vault Plugins](docs/plugins.md)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
