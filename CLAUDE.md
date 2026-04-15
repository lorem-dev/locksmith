# Locksmith - Project Rules

## Repository: https://github.com/lorem-dev/locksmith

## Language
All code, comments, commit messages, and documentation must be in English.
When conversing with the user, always respond in the user's language.

## Module Names
- Main module: `github.com/lorem-dev/locksmith`
- SDK: `github.com/lorem-dev/locksmith/sdk`
- Gopass plugin: `github.com/lorem-dev/locksmith-plugin-gopass`
- Keychain plugin: `github.com/lorem-dev/locksmith-plugin-keychain`

## Project Structure
- `cmd/locksmith/` - CLI + daemon entry point
- `internal/` - daemon internals (not importable externally)
- `sdk/` - public SDK for vault plugin authors
- `plugins/` - default vault plugins (each is a standalone Go module)
- `proto/` - protobuf definitions
- `gen/proto/` - generated code (do not edit manually, gitignored)
- `docs/` - documentation (architecture, configuration, plugins)
- `docs/plans/` - superpowers implementation plans (gitignored, default plan location)
- `.worktrees/` - git worktrees (gitignored, default worktree location)

## Commits
Follow Conventional Commits (see CONTRIBUTING.md). No mentions of AI tools.

## Testing
- Coverage must be ≥ 90% per package
- All tests must pass under the race detector
- Run `make test-coverage` and `make test-race` before committing

## Logging
Use `internal/log` (zerolog). Never use `fmt.Print*` in daemon/plugin code.

## Documentation
All documentation lives in `docs/`. Keep `README.md` focused on install + quickstart.
Track changes in `CHANGES.md` - development changes go under `## Development`,
releases under `## Version vX.Y.Z`.

## Typography
All documentation and comments must use ASCII punctuation only:
- Hyphens (`-`) instead of em dashes (`-`) or en dashes
- Straight double quotes (`"`) instead of curly quotes (" ")
- Straight single quotes (`'`) instead of curly apostrophes (' ')

## Changelog Skill
Use the `changelog` skill (`.claude/skills/changelog/SKILL.md`) to compress changes before cutting a release.
