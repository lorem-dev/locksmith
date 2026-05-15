---
name: version-bump
description: Bump the locksmith version and compress the changelog into a versioned section. PRINTS the git commit/push/tag commands for the user to copy; never runs git add, commit, push, or tag itself.
---

## Instructions

**Announce at start:** "I'm using the version-bump skill."

This skill drives a release-time version bump. It does NOT auto-commit
or auto-tag - those steps stay manual to keep the blast radius
explicit.

### Step 1 - Read the current version

Read `sdk/version/VERSION` (the `VERSION` symlink at repo root resolves
to the same file). Trim whitespace. Print it.

### Step 2 - Suggest a bump kind

Heuristic based on `## Development` content in `CHANGES.md`:

- Contains `BREAKING` (case-sensitive) -> suggest MAJOR.
- Otherwise contains `feat(` or `feat:` -> suggest MINOR.
- Otherwise -> suggest PATCH.

This is advisory only; the user always confirms.

### Step 3 - Ask for the new version

Show multiple-choice options computed from the current version
`X.Y.Z`:

- `[suggested]` (the heuristic recommendation)
- `MAJOR`: `(X+1).0.0`
- `MINOR`: `X.(Y+1).0`
- `PATCH`: `X.Y.(Z+1)`
- `enter manually` (free-form)

Wait for the user's choice.

### Step 4 - Optionally run check-licenses

Ask: `Run check-licenses before bumping? Recommended if any go.mod has changed since the previous release. [y/n]`

If `y`, invoke the `check-licenses` skill via the `Skill` tool. After
it completes, return here.

If `n`, skip silently.

### Step 5 - Write VERSION

Write the chosen version to `sdk/version/VERSION` with a single
trailing newline. The `VERSION` symlink in repo root resolves to this
file automatically.

### Step 5b - Sync README.md install pin

The README's "Pin a specific version" example must match `VERSION`.
The pin lives in the Installation section as:

```
LOCKSMITH_VERSION=vX.Y.Z curl -fsSL https://github.com/lorem-dev/locksmith/releases/download/vX.Y.Z/install.sh | sh
```

Both occurrences of `vX.Y.Z` on that line must equal the new version
(with a leading `v`).

1. Find the current pin:

   ```bash
   grep -nE 'LOCKSMITH_VERSION=v[0-9]+\.[0-9]+\.[0-9]+' README.md
   ```

2. If the version on that line already equals the new `vX.Y.Z`, do
   nothing.

3. Otherwise, replace BOTH occurrences on that line with the new
   `vX.Y.Z`. Use the `Edit` tool with the full line as `old_string`
   so only the install pin is touched. Other version mentions in the
   README (e.g. "added in v0.1.0" notes) must NOT be rewritten.

4. Verify:

   ```bash
   grep -nE 'v[0-9]+\.[0-9]+\.[0-9]+' README.md
   ```

   Every match on the `LOCKSMITH_VERSION=` line and the
   `releases/download/` URL must show the new version. Historical
   references in changelog snippets or "since" notes are unaffected.

If a pre-existing drift is detected here (the previous release left
the README out of sync), fix it during this bump - do NOT carry the
drift forward.

### Step 6 - Compress the changelog

Invoke the `changelog` skill via the `Skill` tool. When the skill's
"Ask for version" step prompts, answer with the version chosen in
Step 3 and today's date (`YYYY-MM-DD`); do not re-prompt the user.

The `changelog` skill replaces the old `## Development` block with a
new empty `## Development` plus a `## Version vX.Y.Z - YYYY-MM-DD`
section containing the compressed bullets.

### Step 7 - Print the commands the user will run manually

> **HARD RULE.** From this point on the skill is read-only. You MUST
> NOT run `git add`, `git commit`, `git push`, or `git tag`. You
> MUST NOT open the PR yourself. The commands below are printed for
> the human maintainer to copy, review, and execute. The human is
> the one who pushes branches, opens the release PR, merges it, and
> creates the tag.

Releases ship via a PR from `develop` into `main`; the merge commit
on `main` is the release commit and the tag is created from it. See
"Release flow" in `CONTRIBUTING.md`.

Print verbatim (this is your last output - no further tool calls):

```
Version bumped to vX.Y.Z. Run the following yourself.

On develop, review the diff then commit:

  git add sdk/version/VERSION CHANGES.md LICENSE README.md
  git commit -S -m "release: vX.Y.Z"

Push develop and open a PR develop -> main with the new
## Version vX.Y.Z section as the description:

  git push origin develop
  # then open the PR in your browser or with `gh pr create --base main`

After the PR is merged, tag the merge commit on main:

  git checkout main && git pull
  git log -1 --format='%s'    # sanity-check HEAD is the release merge
  git tag -s vX.Y.Z -m "vX.Y.Z"
  git push origin vX.Y.Z       # push only the new tag, not all tags
```

### Notes

- The skill ends after Step 7's print. Do NOT continue with git
  operations; do NOT open the PR; do NOT push or tag.
- Do NOT include AI tool names in the commit message or tag
  message that the user will write.
- Pre-release suffixes (e.g. `-rc1`, `-alpha`) are out of scope for
  this skill; if needed, write the version manually in step 3.
