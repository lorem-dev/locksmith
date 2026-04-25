# Agent Integration

Locksmith gives AI agents secure access to secrets with per-session caching.
This document describes the integration protocol for all supported agent
platforms.

## Protocol

Every agent that may need secrets follows three steps:

**Step 1 - Check for an existing session**

Before doing any work, check whether `LOCKSMITH_SESSION` is already set in the
environment. If it is, use it - no action needed.

**Step 2 - Ensure a session exists**

If `LOCKSMITH_SESSION` is not set (or is empty), run:

```bash
export LOCKSMITH_SESSION=$(locksmith session ensure --quiet)
```

`locksmith session ensure` reuses an existing valid session from the environment
or creates a new one. It exits non-zero if the daemon is not running - handle
this gracefully (secrets will be unavailable, but work should continue).

**Step 3 - Retrieve secrets**

Use `--key` when the alias is configured in `~/.config/locksmith/config.yaml`:

```bash
locksmith get --key openai_api_key
locksmith get --key github_token
```

Use `--path` + `--vault` to access a secret directly by its path in the vault
(no alias needed):

```bash
locksmith get --vault gopass --path work/aws/access-key-id
locksmith get --vault keychain --path "My API Token"
```

The session is reused automatically for subsequent calls within the same session
TTL (default: 3h).

## Sub-agent Delegation

When spawning sub-agents, pass `LOCKSMITH_SESSION` in their environment:

```bash
# Sub-agent reuses the parent session - no re-authorization needed.
LOCKSMITH_SESSION=$LOCKSMITH_SESSION some-agent-tool ...
```

This behavior is controlled by `agent.pass_session_to_subagents` in
`~/.config/locksmith/config.yaml` (default: `true`). When set to `false`,
each sub-agent must obtain its own session independently.

## Platform Support

| Platform | Session automation | Sub-agent passing | Notes |
|----------|--------------------|-------------------|-------|
| Claude Code | Hook (auto-installed by `locksmith init`) | Via instructions | Restart Claude Code after `init` |
| Gemini CLI | Via instructions | Via instructions | |
| Cursor | Via instructions | Via instructions | |
| Copilot CLI | Via instructions | Via instructions | |
| Codex | Via instructions | Via instructions | |

## Claude Code Hook Setup

Run `locksmith init` - the hook is installed automatically when Claude Code is
detected. It registers a `UserPromptSubmit` hook in `~/.claude/settings.json`
that injects `LOCKSMITH_SESSION` before each prompt.

After installation, restart Claude Code for the hook to take effect.

### Manual setup

If you prefer to install the hook without using `init`:

1. Run `locksmith init --agent claude` to write the hook script to
   `~/.config/locksmith/agent-hook.sh` without going through the full wizard.

2. Add to `~/.claude/settings.json` (merge with existing content, do not
   overwrite):

   ```json
   {
     "hooks": {
       "UserPromptSubmit": [
         {
           "matcher": "",
           "hooks": [
             {
               "type": "command",
               "command": "/Users/<you>/.config/locksmith/agent-hook.sh"
             }
           ]
         }
       ]
     }
   }
   ```

The hook exits silently if the Locksmith daemon is not running, so it never
blocks agent work.

## Locksmith Config Reference

```yaml
agent:
  pass_session_to_subagents: true   # default: true
```

See [Configuration Reference](configuration.md) for the full reference.
