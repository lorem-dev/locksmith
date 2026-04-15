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
make build-all
```

## Quick Start

```bash
# 1. Initialize (interactive TUI)
locksmith init

# 2. Start the daemon
locksmith serve &

# 3. Start a session
export LOCKSMITH_SESSION=$(locksmith session start | jq -r .session_id)

# 4. Use in MCP config headers
# "Authorization": "Bearer $(locksmith get --key my-api-key)"

# 5. End session when done
locksmith session end
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
