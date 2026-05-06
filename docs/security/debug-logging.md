# Debug Logging Security Notice

## What debug mode does

When `logging.level: debug` is set, the locksmith daemon logs session IDs in
plaintext. Session IDs are bearer tokens - anyone with a session ID can use it
to retrieve secrets from any vault the session has access to.

In debug mode, plaintext session IDs appear in:

- The log file (if `logging.file` is set)
- stdout (if no log file is configured)
- Any log aggregator or monitoring system that collects daemon output

The daemon prints a warning to stderr when it starts in debug mode:

```
WARNING: debug logging is enabled - session IDs will be written to logs in
plaintext. Do not use debug level in production.
```

## When to use debug mode

Debug mode is intended for:

- Local development on a personal machine
- Troubleshooting authentication or vault connectivity issues
- Short-duration diagnostics (disable as soon as the issue is identified)

Do not use debug mode in:

- Shared development environments
- Staging or production systems
- Any environment where log files are accessible to multiple users or systems

## Enabling and disabling

In `~/.config/locksmith/config.yaml`:

```yaml
logging:
  level: debug   # enable debug logging
```

To disable, change to `info` (the default):

```yaml
logging:
  level: info
```

Restart the daemon after changing the level:

```bash
# Stop the daemon (send SIGTERM to the locksmith serve process)
pkill -TERM -f 'locksmith serve'
# Let the shell hook restart it on next terminal open, or run manually:
locksmith serve
```

## Log file permissions

If `logging.file` is configured, restrict access to the log directory and
file to the owning user:

```bash
chmod 700 ~/.config/locksmith/logs
chmod 600 ~/.config/locksmith/logs/daemon.log
```

locksmith creates the log directory with mode `0700` automatically, but
existing directories retain their original permissions.
