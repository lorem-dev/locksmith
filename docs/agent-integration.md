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

```bash
locksmith get --key <alias>
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
| Claude Code | Hook (automatic) | Via instructions | See hook setup below |
| Gemini CLI | Via instructions | Via instructions | See `docs/hooks/GEMINI.md` |
| Cursor | Via instructions | Via instructions | See `docs/hooks/AGENTS.md` |
| Copilot CLI | Via instructions | Via instructions | See `docs/hooks/AGENTS.md` |
| Codex | Via instructions | Via instructions | See `docs/hooks/AGENTS.md` |

## Claude Code Hook Setup

For Claude Code, a `UserPromptSubmit` hook automatically ensures a session
exists before each prompt - no manual session management needed.

**1. Copy the hook script to a stable location:**

```bash
cp docs/hooks/locksmith-session.sh ~/.config/locksmith/agent-hook.sh
chmod +x ~/.config/locksmith/agent-hook.sh
```

**2. Register it in `~/.claude/settings.json`:**

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "~/.config/locksmith/agent-hook.sh"
          }
        ]
      }
    ]
  }
}
```

The hook injects `LOCKSMITH_SESSION` into the Claude Code environment before
each prompt. If the daemon is not running, the hook exits silently and work
continues without Locksmith.

## Locksmith Config Reference

```yaml
agent:
  pass_session_to_subagents: true   # default: true
```

See [Configuration Reference](configuration.md) for the full reference.
