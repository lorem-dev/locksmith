# Locksmith Configuration Reference

## Configuration file

Default location: `~/.config/locksmith/config.yaml`

Override with `--config <path>`.

## Top-level structure

```yaml
defaults:
  session_ttl: 3h           # default session TTL (e.g. 1h, 30m)
  socket_path: ~/.config/locksmith/locksmith.sock

logging:
  level: info               # debug | info | warn | error
  format: text              # text | json
  file: ~/.config/locksmith/logs/daemon.log  # optional log file

vaults:
  <name>:
    type: <plugin-type>
    # ... plugin-specific fields

keys:
  <alias>:
    vault: <vault-name>
    path: <secret-path>
```

## Logging configuration

### logging.level

**Optional.** Log level for daemon output. Default: `info`.

- `debug` - verbose output; session IDs are logged in plaintext (see security note below)
- `info` - standard operational messages
- `warn` - warnings and errors only
- `error` - errors only

### logging.format

**Optional.** Output format. Default: `text`.

- `text` - human-readable plaintext logs
- `json` - structured JSON for log aggregators

### logging.file

**Optional.** Path to the log file. If set, all log output is written to this
file instead of stdout. Supports `~` expansion. The parent directory is created
automatically with mode `0700` if it does not exist.

The file is rotated when it reaches 50 MB and files older than 3 days are
deleted automatically.

Recommended when running as a background daemon via the shell hook.

> **Security note:** If `logging.level` is `debug`, session IDs are written to
> the log in plaintext. See [Debug Logging Security Notice](security/debug-logging.md).

## Direct access (without alias)

```bash
locksmith get --vault keychain --path my-account
locksmith get --vault my-gopass --path dev/key
```

---

## Vault plugins

### keychain (macOS only)

Retrieves secrets from the macOS Keychain using the Security framework.
Authorization (Touch ID or password) is triggered by the OS on each access.

> Plugin-specific setup, examples, and troubleshooting:
> [`plugins/keychain/README.md`](../plugins/keychain/README.md).

**Configuration:**

```yaml
vaults:
  keychain:
    type: keychain
    service: com.example.myapp  # optional: default Keychain service name
```

**Key path format:**

```yaml
keys:
  # Plain account - uses vault-level service (or "locksmith" if unset)
  notion-token:
    vault: keychain
    path: notion

  # service/account - overrides vault-level service for this key only
  github-token:
    vault: keychain
    path: github/mytoken

  # No service configured - falls back to "locksmith" for backward compatibility
  legacy-key:
    vault: keychain
    path: my-old-account
```

**Service resolution order:** path prefix `service/account` > vault `service:` > `"locksmith"` (backward-compatible default).

**Full example:**

```yaml
vaults:
  work:
    type: keychain
    service: com.acme.work    # default service for all keys in this vault

keys:
  slack:
    vault: work
    path: slack               # service="com.acme.work", account="slack"
  github:
    vault: work
    path: github/token        # service="github", account="token" (overrides vault-level)
  legacy:
    vault: work
    path: legacy-tool         # service="locksmith" if vault has no service: set
```

**Notes:**
- Only available on macOS (darwin/amd64 and darwin/arm64).
- Passwords are stored and retrieved using `SecItemCopyMatching` via CGo.
- Error messages come directly from `SecCopyErrorMessageString` for readability.
- A path with more than one `/` is rejected at startup (e.g. `a/b/c` is invalid;
  use `a/b` for service=`a`, account=`b`).

---

### gopass

Retrieves secrets from a [gopass](https://github.com/gopasspw/gopass) password store.

> Plugin-specific setup, examples, and troubleshooting:
> [`plugins/gopass/README.md`](../plugins/gopass/README.md).

**Configuration:**

```yaml
vaults:
  secrets:
    type: gopass
    store: work              # optional: gopass mount name (default: root store)

keys:
  notion-token:
    vault: secrets
    path: personal/notion    # gopass path within the store
```

**Full example:**

```yaml
vaults:
  personal:
    type: gopass             # uses root store
  work:
    type: gopass
    store: work              # uses "work" gopass mount

keys:
  github-token:
    vault: work
    path: dev/github-api
  anthropic-key:
    vault: personal
    path: ai/anthropic
```

**Notes:**
- Requires `gopass` installed and configured (`gopass ls` must succeed).
- `store:` is passed as the gopass mount name; omit to use the default root store.
- GPG passphrase prompts in background daemons require `locksmith-pinentry` -
  see "GPG passphrase and background daemons" below.

---

## Agent integration

```yaml
agent:
  pass_session_to_subagents: true
```

### agent.pass_session_to_subagents

**Optional.** Controls whether agents should pass `LOCKSMITH_SESSION` to
sub-agents they spawn. Default: `true`.

When `true`, the agent passes `LOCKSMITH_SESSION` in the environment when
spawning child agents or tools, allowing them to reuse the parent session
without re-authorization.

When `false`, each agent obtains its own independent session.

See [Agent Integration](agent-integration.md) for the full protocol.

---

## MCP servers

Configure named MCP servers under `mcp.servers`. Use
`locksmith mcp run --server <name>` to start them with secrets injected.

```yaml
mcp:
  servers:
    github:
      command: ["npx", "-y", "@github/mcp"]
      env:
        GITHUB_TOKEN: github-token        # key alias from keys:

    my-api:
      url: https://api.example.com
      transport: auto                     # sse | http | auto (default: auto)
      headers:
        Authorization: "Bearer {key:openai-key}"
        X-Org-ID: "{vault:keychain path:org/id}"

    ad-hoc:
      command: ["my-tool"]
      env:
        API_KEY:
          vault: gopass
          path: work/api/key
```

### Lazy secret resolution

The first `GetSecret` call for any `mcp.servers.<name>` entry fires
on the first MCP request from the AI client, not at `locksmith mcp
run` startup. If a configured MCP server is never invoked, its
secrets are never fetched and no vault prompt is shown. Locksmith
will also transparently start a fresh session and retry once if the
session held by `mcp run` expires between startup and the first
fetch.

Within proxy mode, locksmith also defers individual auth headers
(those whose value template references the vault via `{key:...}` or
`{vault:... path:...}`) until the remote server demands them. Static
header values - those without any `{` token - are sent from the very
first request. The auth-deferral is automatic and has no config
knob; servers that do not require auth on the MCP handshake therefore
never trigger a vault prompt for that connection.

### mcp.servers.\<name\>.command

**Required (local mode).** List of strings: executable followed by
arguments. Mutually exclusive with `url`.

### mcp.servers.\<name\>.env

**Optional (local mode).** Map of environment variable names to secret
references.

Each value is either:
- A string: key alias from `keys:` in this config file.
- A struct with `vault:` and `path:` fields for direct vault access.

### mcp.servers.\<name\>.url

**Required (proxy mode).** Remote MCP server URL. Mutually exclusive
with `command`.

### mcp.servers.\<name\>.transport

**Optional (proxy mode).** HTTP transport to use. Default: `auto`.

- `auto` - try Streamable HTTP (MCP spec 2025-03-26); fall back to SSE on `404`/`405`
- `http` - Streamable HTTP only
- `sse` - SSE only (legacy)

### mcp.servers.\<name\>.headers

**Optional (proxy mode).** Map of HTTP header names to value templates.
Template tokens:

| Token | Description |
|-------|-------------|
| `{key:alias}` | Key alias from `keys:` |
| `{vault:name path:value}` | Direct vault + path |

---

## Vault types

| type | Description |
|------|-------------|
| `keychain` | macOS Keychain (CGo, Touch ID) |
| `gopass` | gopass password manager (shells out to `gopass` CLI) |

Default plugins are placed in `~/.config/locksmith/plugins/` automatically
by `locksmith init` from the embedded bundle. See
[Plugins](plugins/README.md) and [PLUGINS.md](../PLUGINS.md).

---

## Re-running locksmith init

Running `locksmith init` on a machine that already has a config file at the chosen
path will detect the file and validate it. You will be offered three options:

- **Continue with existing config** - skip rewriting the config file; proceed with
  agent and sandbox setup only.
- **Overwrite with new config** - run the full wizard and replace the file.
- **Exit setup** - cancel without any changes.

In `--auto` mode the choice is made automatically: a valid config is kept as-is;
an invalid config is silently replaced.

---

## GPG passphrase and background daemons

When running as a background daemon, GPG passphrase prompts require
`locksmith-pinentry`. See [GPG Passphrase and Pinentry](pinentry.md).

---

## Shell autostart

To start the locksmith daemon automatically when you open a terminal, add a
shell hook. The `locksmith init` wizard offers to do this for you.

To add it manually, append the following to your shell config file:

**bash / zsh / ash** (`~/.bashrc`, `~/.zshrc`, or `~/.profile`):

```sh
# locksmith daemon autostart
if command -v locksmith >/dev/null 2>&1; then locksmith _autostart 2>/dev/null; fi
```

**fish** (`~/.config/fish/config.fish`):

```fish
# locksmith daemon autostart
if command -v locksmith >/dev/null 2>&1; locksmith _autostart 2>/dev/null; end
```

The hook is idempotent: if the daemon is already running, `_autostart` exits
immediately without spawning a second process.
