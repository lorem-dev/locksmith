# Locksmith - Project Rules

- Overall rules: **CLAUDE.md** (must read)
- Contributing rules: **CONTRIBUTING.md** (must read)
- Makefile commands: **Makefile** (must read)
- Changelog: **CHANGES.md**

## Project skills (`.claude/skills/`)

Invoke these via the agent's skill mechanism. Each skill's
`SKILL.md` carries the full step list.

| Skill | When to use |
|---|---|
| `verification` | Final gate on every plan: lint, race, coverage, build, GPG, docs, CHANGES.md. Runs `.scripts/verification.sh`. |
| `check-licenses` | After any change to `go.mod` / `go.sum`. Audits Apache-2.0 compatibility and updates `LICENSE`. |
| `release-prep` | On `develop` before cutting a release. Verifies docs reflect each accumulated `## Development` bullet, then drives `version-bump`. Read-only. |
| `version-bump` | Driven by `release-prep`. Bumps `sdk/version/VERSION`, syncs README install pin, invokes `changelog`. Prints git commands; never executes push/tag. |
| `changelog` | Compresses `## Development` into a versioned section; triages into BREAKING / Features / Fixes. |

`release-prep` and `version-bump` never run `git push`, `git tag`,
or `gh pr create` themselves - they print the commands for the
human maintainer to run.
