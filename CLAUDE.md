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
- `cmd/locksmith-pinentry/` - pinentry helper binary (main.go only; logic lives in internal/pinentry)
- `internal/` - daemon internals (not importable externally)
- `internal/pinentry/` - Assuan/pinentry protocol implementation
- `sdk/` - public SDK for vault plugin authors (split into subpackages)
  - `sdk/vault` - Provider interface, Serve(), plugin wiring
  - `sdk/errors` - VaultError type and constructors
  - `sdk/log` - LogConfig, NewLogWriter, IsDebug
  - `sdk/session` - HideSessionId, MaskSessionId
  - `sdk/platform` - Platform string constants (Darwin, Linux)
- `plugins/` - default vault plugins (each is a standalone Go module)
- `proto/` - protobuf definitions
- `gen/proto/` - generated code (do not edit manually, gitignored)
- `docs/` - documentation (architecture, configuration, plugins)
- `docs/superpowers/plans/` - superpowers implementation plans (gitignored, default plan location)
- `docs/superpowers/specs/` - superpowers design specs (gitignored, default spec location)
- `.worktrees/` - git worktrees (gitignored, default worktree location)

## Project skills (`.claude/skills/`)

Locksmith ships project-specific skills that drive recurring
workflows. Invoke them via the `Skill` tool. Each skill is
self-contained; consult its `SKILL.md` for the exact step list.

| Skill | When to use |
|---|---|
| `verification` | Mandatory final step of every superpowers plan. Runs lint, race, coverage, build, GPG, docs, and CHANGES.md checks via `.scripts/verification.sh`. |
| `check-licenses` | After adding or removing any direct Go dependency. Audits license compatibility with Apache 2.0 and updates `LICENSE`. |
| `release-prep` | On `develop` before cutting a release. Verifies each accumulated `## Development` bullet has matching docs updates since the last tag, then drives `version-bump`. Read-only; never pushes or tags. |
| `version-bump` | Driven by `release-prep` (rarely invoked directly). Bumps `sdk/version/VERSION`, syncs the README install pin, and invokes `changelog`. Prints the git commands; does not execute them. |
| `changelog` | Compresses `## Development` into a versioned `## Version vX.Y.Z` section. Triages bullets into BREAKING / Features / Fixes; drops internal noise. |

**Skills that print commands never run them.** `release-prep` and
`version-bump` are read-only: they print the `git add`, `git push`,
`gh pr create`, and `git tag` commands for the maintainer to copy.
The maintainer is responsible for pushing branches, opening PRs,
merging them, and creating the release tag.

## Superpowers Conventions
- **Plans and specs MUST be saved under `docs/superpowers/`. Never save them
  under `.claude/`, the user's home directory, `/tmp`, or anywhere else.**
  This applies to every agent and skill that writes superpowers artefacts,
  regardless of any built-in default the agent may have.
- Plans: save to `docs/superpowers/plans/YYYY-MM-DD-<feature>.md`
- Specs: save to `docs/superpowers/specs/YYYY-MM-DD-<feature>-design.md`
- Both directories are gitignored and MUST NOT be committed to git.
  Never use `git add -f` on these files. They are local working artifacts only.

## Final Verification (mandatory last step in every superpowers plan)

Every superpowers plan MUST end with a task that runs the `verification`
skill. When writing a plan, the last numbered task must be:

```
### Task N: Final Verification
- [ ] Run the `verification` skill
```

Do not mark a feature branch done until this task passes with all
gates green. Consult the skill's `SKILL.md` for the gate list.

## Branching

- **Do not commit directly to `main`.** The only exception is hotfixes
  targeting a released version - and even those should go via a PR
  unless the situation is genuinely urgent. See the "Hotfix flow"
  section in `CONTRIBUTING.md` for the branch-from-main pattern and
  the `## Development` cleanup that follows.
- Day-to-day work lands on `develop`, either via PRs from feature
  branches or directly by maintainers.
- Releases are cut by opening a PR from `develop` into `main`. The
  merge commit on `main` is the release commit; the tag is created
  from that commit. See `CONTRIBUTING.md` for the full release flow.
- When opening a PR, the default base branch is `develop` unless the
  change is an explicit release PR or hotfix to `main`.

## Commits
Follow Conventional Commits (see CONTRIBUTING.md).
- No mentions of AI tools, agents, or assistants anywhere in commits.
- No `Co-Authored-By: Claude ...` or any other AI co-author lines.
- No "generated by", "with AI assistance", or similar phrases in any part of a commit.

## GPG Signing
Commits in this repository are expected to be GPG-signed.

**At the end of any session** that produces commits (superpowers workflows, standalone fixes,
or any other work), before wrapping up, run:

```bash
git log --format="%G? %h %s" | head -20
```

- `G` = good signature - nothing to do.
- `N` or `B` = unsigned or bad signature.

If any commits lack a valid signature, offer to re-sign them. Do NOT do this automatically -
ask first and only proceed if the user confirms. The re-sign command is:

```bash
LAST_SIGNED=$(git log --format="%G? %H" | awk '$1=="G"{print $2; exit}')
git rebase --onto "$LAST_SIGNED" "$LAST_SIGNED" HEAD \
  --exec "git commit --amend --no-edit -S && git restore go.work.sum 2>/dev/null; true"
```

Do not mention GPG during the session - only check at the very end, after all other work is done.

## Testing
- Coverage must be ≥ 90% per package
- All tests must pass under the race detector
- Run `make test-coverage` and `make test-race` before committing

## Makefile

`make` is the entry-point for all build, lint, test, and code-generation tasks.

| Command                 | Purpose                                              |
|-------------------------|------------------------------------------------------|
| `make init`             | First-time setup: install tools, generate protobuf   |
| `make build`            | Build locksmith and locksmith-pinentry               |
| `make build-all`        | Build locksmith + all vault plugins                  |
| `make lint`             | Run golangci-lint and buf linter                     |
| `make test`             | Unit tests across all workspace modules              |
| `make test-race`        | Unit tests with race detector                        |
| `make test-coverage`    | Coverage report (HTML + summary table in .reports/)  |
| `make test-integration` | Integration tests (require running daemon + plugins) |
| `make verify`           | Run all quality gates (lint, race, coverage, build, GPG, docs) |
| `make proto`            | Regenerate protobuf Go code from .proto files        |
| `make tidy`             | Run `go mod tidy` across all workspace modules       |
| `make install-tools`    | Install pinned tool versions into $GOPATH/bin        |

## Dependency Changes
**Whenever a dependency is added or removed, run the `check-licenses` skill.**
This audits third-party Go dependencies for license compatibility with Apache 2.0.
Run it before committing any change to `go.mod` / `go.sum`.

## Code Conventions

### CLI command files (`internal/cli/`)
Each Cobra subcommand lives in its own file named `<command>_cmd.go` (e.g. `get_cmd.go`,
`serve_cmd.go`). Files that do not define a command (e.g. `root.go`, `client.go`,
`color.go`, `errors.go`) are exempt. Tests for a command go in `<command>_cmd_test.go`.

## Logging
Use `internal/log` (zerolog). Never use `fmt.Print*` in daemon/plugin code.

## Documentation
All documentation lives in `docs/`. Keep `README.md` focused on install + quickstart.

## Plugin Documentation

Each built-in plugin under `plugins/<name>/` ships its own `README.md` covering
installation, configuration examples, and troubleshooting. The canonical YAML
schema lives in `docs/configuration.md`; the plugin README links to it rather
than duplicating it.

Whenever a plugin's behavior, configuration fields, or external requirements
change, update `plugins/<name>/README.md` in the same commit.

## Bundled Plugins

Default plugins (`gopass`, `keychain`) and `locksmith-pinentry` ship
embedded inside the `locksmith` binary as a per-platform zip. `locksmith
init` extracts the plugins for the chosen vaults to
`~/.config/locksmith/plugins/` and pinentry to
`~/.config/locksmith/bin/locksmith-pinentry`. Plugin version is locked to
the host `locksmith` version - no network resolution. See
[`PLUGINS.md`](PLUGINS.md) for the full overview.

## Changelog (CHANGES.md)

Every user-visible change must be recorded in `CHANGES.md` under
`## Development` in the **same commit** as the code change - never
as a follow-up. One bullet per logical change. Breaking changes
start with `BREAKING:` (see the `changelog` skill for the full
rules).

When writing a superpowers plan, include an explicit task for
updating `CHANGES.md` before the "commit" step:

```
- [ ] Update CHANGES.md under ## Development with a summary of the change
```

For releases, run the `release-prep` skill on `develop`: it
verifies docs are updated for each accumulated change, then drives
`version-bump` (which itself invokes `changelog` for compression).
See `CONTRIBUTING.md` for the release-via-PR flow.

## Typography
All documentation and comments must use ASCII punctuation only:
- Hyphens (`-`) instead of em dashes (`-`) or en dashes
- Straight double quotes (`"`) instead of curly quotes (" ")
- Straight single quotes (`'`) instead of curly apostrophes (' ')
