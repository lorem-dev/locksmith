---
name: release-prep
description: Prepare a release on the develop branch. Verifies that user-visible changes in ## Development have matching docs updates since the last tag, then drives the version-bump skill. Use before opening the release PR from develop into main.
---

## Instructions

**Announce at start:** "I'm using the release-prep skill."

This skill is the docs-aware gate before cutting a release. It does
NOT replace `verification` (`make verify`) - it focuses on whether
each user-visible bullet in `## Development` is documented in the
right place. Run it on `develop` after all feature branches have
merged.

### Step 0 - Sanity checks

1. Confirm the current branch is `develop`:

   ```sh
   git rev-parse --abbrev-ref HEAD
   ```

   If not `develop`, ask the user whether to switch or abort.

2. Confirm the working tree is clean:

   ```sh
   git status --short
   ```

   If there are uncommitted changes, list them and ask whether to
   continue.

3. Find the last release tag:

   ```sh
   LAST_TAG=$(git describe --tags --abbrev=0 --match='v*' 2>/dev/null)
   ```

   If there is no prior tag (first release), set `LAST_TAG` to the
   first commit. Tell the user which baseline is being used.

### Step 1 - Read `## Development`

Extract every bullet under `## Development` in `CHANGES.md` into a
list `entries`.

If `entries` is empty: tell the user "no development changes recorded
since `$LAST_TAG`; release would be empty. Abort?" and stop unless
the user explicitly wants to proceed.

If `entries` has fewer than 2 bullets: warn "release contains only
N change(s); confirm before proceeding."

### Step 2 - Classify each entry by area

For each bullet, infer the area it touches by keyword matching
against the bullet text (case-insensitive). Use the table below;
the first match wins.

| Keywords in bullet | Area | Expected docs |
|---|---|---|
| `mcp`, `proxy`, `transport`, `jsonrpc`, `sse`, `streamable`, `grpcfetcher`, `lazy fetch`, `session refresh` | mcp | `docs/architecture.md`, `docs/configuration.md` |
| `init`, `wizard`, `auto`, `detect`, `vault selection` | init | `docs/configuration.md`, `README.md` |
| `plugin`, `gopass`, `keychain`, `pinentry` | plugins | `plugins/<name>/README.md`, `docs/plugins/*.md`, `PLUGINS.md` |
| `daemon`, `session`, `socket`, `grpc`, `reload`, `sighup` | daemon | `docs/architecture.md`, `docs/configuration.md` |
| `cli`, `flag`, `subcommand`, `command`, `argument` | cli | `README.md`, `docs/configuration.md` |
| `sdk`, `provider`, `proto/` | sdk | `docs/plugins/authoring.md`, `docs/plugins/compatibility.md` |
| `agent`, `claude code`, `cursor`, `gemini`, `codex`, `hook` | agent | `docs/agent-integration.md` |
| `config`, `yaml`, `mcp.servers`, `logging.` | config | `docs/configuration.md` |
| `install`, `release`, `tag`, `version`, `binary` | install | `README.md`, `docs/install.md`, `docs/release.md`, `docs/verification.md` |
| `debug logging`, `mask`, `redact`, `security`, `gpg verify` | security | `docs/security/debug-logging.md`, `docs/verification.md` |
| `lint`, `test`, `coverage`, `refactor` (internal) | internal | (none expected) |

When a bullet matches keywords from multiple rows (e.g. an MCP
config change), the first match in the table wins. The `mcp` row is
listed first so MCP-flavoured session/grpc bullets do not get
misrouted to the `daemon` row.

If a bullet matches no area, ask the user to classify it before
proceeding.

### Step 3 - Check docs were updated for each area

For each non-internal area, run:

```sh
git diff --name-only "$LAST_TAG"..HEAD -- <expected docs path(s)>
```

Record the result per entry: GREEN if at least one expected doc was
touched in the range, RED if none were touched.

Print the table:

```
Area      | Entry                                    | Docs touched | Verdict
----------|------------------------------------------|--------------|--------
mcp       | locksmith mcp run --url retries on ...   | docs/arch... | GREEN
init      | locksmith init no longer offers ...      | (none)       | RED
plugins   | gopass: handle GPG agent timeout         | plug/gop...  | GREEN
internal  | refactor: extract helper                 | n/a          | SKIP
```

### Step 4 - Resolve RED rows

For every RED row, prompt the user with the bullet and three
options:

```
RED: <bullet text>
Expected docs (untouched since $LAST_TAG): <paths>
Choose:
  [d] add/update docs now (you will edit before continuing)
  [i] mark as internal (rewrite the bullet or drop it during changelog compression)
  [s] skip - I will handle docs separately
```

- `d`: pause and let the user edit; when they say `done`, re-check
  the diff for that area and update the verdict.
- `i`: note the entry as internal; it will be classified as **drop**
  during `changelog` compression.
- `s`: record the override and proceed. Note: this is escape-hatch;
  the release will still flag the gap in the final summary.

Do not proceed past Step 4 while any RED row is unresolved.

### Step 5 - Drive the release

When all rows are GREEN, SKIP, or internal:

1. Print the final docs verdict table for the record.
2. Invoke the `version-bump` skill via the `Skill` tool. It will
   bump `sdk/version/VERSION`, optionally invoke `check-licenses`,
   sync the README install pin, and invoke `changelog` to compress
   `## Development`.
3. `version-bump` prints its own "next steps" block when it
   finishes; do not duplicate it.

### Step 6 - Reminder

> **HARD RULE.** This skill never runs `git push`, `git tag`, or
> opens a PR. The commands `version-bump` printed in Step 5 are for
> the user to execute. Stop after printing the reminder below.

End with:

```
Release prepared on develop. Verification (lint/race/coverage/docs)
is NOT part of this skill; run `make verify` or the `verification`
skill before opening the PR.

Direct push to main is forbidden - the release lands via PR review.
Push develop, open the PR, merge it, and tag the merge commit
yourself using the commands version-bump printed above.
```

### Notes

- This skill is for the integration step on `develop`, not for
  hotfixes targeting `main` directly. For hotfix flow see
  `CONTRIBUTING.md`.
- The keyword classification is heuristic; ambiguous bullets always
  fall through to user prompts.
- The `internal` bucket here mirrors the `drop` bucket in the
  `changelog` skill - both indicate "do not surface to release
  notes."
