# Locksmith

Secure middleware that gives AI agents access to secrets stored in vault providers
(macOS Keychain, gopass), with per-session caching and
vault-delegated authorization (Touch ID, GPG passphrase).

## Installation

```sh
curl -fsSL https://github.com/lorem-dev/locksmith/releases/latest/download/install.sh | sh
```

Pin a specific version:

```sh
LOCKSMITH_VERSION=v0.2.0 curl -fsSL https://github.com/lorem-dev/locksmith/releases/download/v0.2.0/install.sh | sh
```

Custom install dir (default `~/.local/bin`):

```sh
LOCKSMITH_INSTALL_DIR=/usr/local/bin curl -fsSL https://github.com/lorem-dev/locksmith/releases/latest/download/install.sh | sudo sh
```

Re-running the same command updates an existing install in place and
refreshes bundled plugins / `locksmith-pinentry`. Supported platforms:
linux/amd64, linux/arm64, darwin/arm64. On Intel Macs (darwin/amd64)
the install script prints `go install` instructions instead of
fetching a prebuilt binary. See
[docs/install.md](docs/install.md) for manual download, GPG signature
verification, build-from-source, the `go install` fallback, and the
full list of install-script flags. Plugin and pinentry extraction
happens on first `locksmith init`; see [PLUGINS.md](PLUGINS.md).

## Quick Start

```bash
locksmith init
```

`locksmith init` is an interactive wizard: it detects your available vaults
(Keychain, gopass), writes a starter config, and installs hooks for the AI
agents you use (Claude Code, Cursor, Copilot, Codex, Gemini CLI) so they get
a `LOCKSMITH_SESSION` automatically. The daemon is started by the installed
shell hook on the next shell session.

## Configuration

After `init`, your config lives at `~/.config/locksmith/config.yaml`. A
minimal example with two vaults (gopass for work, macOS Keychain for
personal) and three named keys:

```yaml
defaults:
  session_ttl: 3h

vaults:
  work:
    type: gopass
    store: work-secrets
  personal:
    type: keychain
    service: locksmith    # macOS only; Keychain service name

keys:
  github-token:
    vault: work
    path: github/personal-access-token
  openai-key:
    vault: work
    path: ai/openai-api-key
  slack-webhook:
    vault: personal
    path: slack-incoming-webhook

mcp:
  servers:
    github:
      command: ["npx", "-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: github-token        # alias from keys:
    my-api:
      url: https://api.example.com
      transport: auto                     # sse | http | auto (default: auto)
      headers:
        Authorization: "Bearer {key:openai-key}"
```

Retrieve a secret by alias:

```bash
locksmith get --key github-token
```

Reload after editing the file (the daemon also auto-detects changes within
~1 second):

```bash
locksmith reload
```

See the [Configuration Reference](docs/configuration.md) for all fields,
vault types, defaults, logging, and security notes.

## MCP Integration

`locksmith mcp run` injects secrets into MCP servers at startup. No shell
substitution required - all values in `mcp.json` are static strings.

**Local MCP server** (secret injected as environment variable):

```json
{
  "mcpServers": {
    "github": {
      "command": "locksmith",
      "args": ["mcp", "run", "--env", "GITHUB_TOKEN=github-token", "--", "npx", "-y", "@modelcontextprotocol/server-github"]
    }
  }
}
```

**Remote MCP server** (secret injected as HTTP header, stdio<->HTTP proxy):

```json
{
  "mcpServers": {
    "my-api": {
      "command": "locksmith",
      "args": ["mcp", "run", "--url", "https://api.example.com", "--header", "Authorization=Bearer {key:openai-key}"]
    }
  }
}
```

**Named server from config** (secrets configured in `~/.config/locksmith/config.yaml`):

```json
{
  "mcpServers": {
    "github": {
      "command": "locksmith",
      "args": ["mcp", "run", "--server", "github"]
    }
  }
}
```

### Secret reference syntax

| Format | Description |
|--------|-------------|
| `alias` | Key alias from `keys:` in `config.yaml` (for `--env` only) |
| `{key:alias}` | Key alias (template form, works in `--header` and `--env`) |
| `{vault:vault-name path:some/path}` | Direct vault + path, no alias required |

### Supported transports

`locksmith mcp run` supports both SSE (legacy) and Streamable HTTP (MCP spec
2025-03-26). Use `--transport sse|http|auto` (default: `auto`). In `auto` mode,
Streamable HTTP is tried first; on `404`/`405` the proxy falls back to SSE.

### Lazy secret resolution

`locksmith mcp run` does not contact the vault at startup. The first
vault prompt (Touch ID dialog, GPG passphrase) for a given MCP server
fires only when the AI client sends its first MCP request to that
server. MCP servers configured but never used in a session never
trigger a prompt.

In local mode (`-- command`) locksmith stays resident as the parent
of the MCP server child. The process tree becomes:

```
AI client -> locksmith mcp run -> <MCP server binary>
```

This adds roughly 5-10 MB RAM per server. Signals (SIGTERM, SIGINT)
sent by the AI client to locksmith are forwarded to the child so
graceful shutdown works as expected.

In proxy mode, locksmith goes one step further: templated `Authorization`
(or other vault-referencing) headers are not sent on the first request.
If the remote MCP server accepts the handshake unauthenticated (a
common pattern - `initialize` and capability negotiation usually do
not require auth), no vault prompt fires. Locksmith only resolves and
sends the auth headers after the server responds `401` or `403`,
after which it remembers the resolved value for the rest of the
proxy session.

The optional long-lived SSE GET stream that some servers use for
notifications is opened with the headers available at that time.
If a server happens to authorise the GET without credentials but
later demands them on POST, the GET is NOT reopened with auth;
that server should be configured with `--transport http` (Streamable
HTTP) instead, which has no long-lived GET.

See the [Configuration Reference](docs/configuration.md#mcp-servers) for
configuring named servers in `config.yaml`. For client-specific notes
(Claude Code, Cursor, Copilot, Codex, Gemini CLI), see
[Agent Integration](docs/agent-integration.md).

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
