# Locksmith Integration for Gemini CLI

This workspace uses [Locksmith](https://github.com/lorem-dev/locksmith) for
secure secret management.

## Session Protocol

At the start of any session where secrets may be needed:

1. Check `LOCKSMITH_SESSION` in environment. If set and non-empty, use it.
2. If not set:
   ```bash
   export LOCKSMITH_SESSION=$(locksmith session ensure --quiet)
   ```
3. Access secrets:
   ```bash
   locksmith get --key <alias>
   ```

## Sub-agent Session Passing

When spawning sub-agents, export `LOCKSMITH_SESSION` into their environment.
This is the default behavior per `agent.pass_session_to_subagents: true` in
`~/.config/locksmith/config.yaml`.

## Non-blocking Behavior

If `locksmith session ensure` fails (daemon not running), do not block - work
continues without secrets.
