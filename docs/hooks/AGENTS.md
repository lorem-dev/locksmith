# Locksmith Integration

This project uses [Locksmith](https://github.com/lorem-dev/locksmith) for
secure secret management. Follow this protocol when accessing secrets.

## Session Protocol

Before accessing any secret, ensure a Locksmith session is active:

1. Check if `LOCKSMITH_SESSION` is set in the environment. If yes, use it.
2. If not set, run:
   ```bash
   export LOCKSMITH_SESSION=$(locksmith session ensure --quiet)
   ```
   If this fails (daemon not running), continue without secrets.
3. Retrieve secrets with:
   ```bash
   locksmith get --key <alias>
   ```

## Sub-agent Delegation

When spawning sub-agents or tools, pass `LOCKSMITH_SESSION` in their
environment so they can reuse the parent session:

```bash
LOCKSMITH_SESSION=$LOCKSMITH_SESSION <sub-agent-command>
```

Check `agent.pass_session_to_subagents` in `~/.config/locksmith/config.yaml`
(default: `true`). If `false`, skip sub-agent delegation.

## Error Handling

If `locksmith session ensure` exits non-zero, the daemon is not running.
Continue work without secrets rather than blocking.
