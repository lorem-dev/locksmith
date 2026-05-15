---
name: changelog
description: Compress development changes into a versioned CHANGES.md entry before cutting a release. Keeps only BREAKING changes, user-visible features, and fixes; drops internal noise.
---

## Instructions

**Announce at start:** "I'm using the changelog skill."

Read `CHANGES.md` and collect every bullet under `## Development`
into a list `entries`. Record `original_count = len(entries)`. If
`entries` is empty, skip to "Ask for version".

The output of this skill is a `## Version vX.Y.Z` section with three
optional subsections, in this fixed order:

```
## Version vX.Y.Z - YYYY-MM-DD

### BREAKING CHANGES
- BREAKING: <verbatim>

### Features
- <one-line user-visible feature>

### Fixes
- <one-line fix>
```

The goal is signal, not exhaustiveness. A reader scanning the
release entry should see what they need to migrate (BREAKING), what
they can now do (Features), and what stopped misbehaving (Fixes) -
nothing else.

### BREAKING entries (immutable contract)

Bullets that begin with `BREAKING:` (uppercase, colon, then space)
are treated specially in every pass below and in the final write:

1. **One bullet per change.** Do NOT merge two `BREAKING:` bullets
   in any pass. Each remains independently addressable.
2. **Grouped at the top.** All `BREAKING:` bullets land under the
   `### BREAKING CHANGES` heading, in their original chronological
   order. They appear above `### Features` and `### Fixes`.
3. **Verbatim across compressions.** When folding `## Development`
   into `## Version vX.Y.Z`, every `BREAKING:` bullet is carried
   forward as written. Non-BREAKING bullets MAY be merged, rephrased,
   or dropped per the rules below.
4. **Immutable after release.** Once a `## Version vX.Y.Z` heading
   is tagged and pushed, its `BREAKING:` bullets must not be
   reworded or removed (typo fixes that do not change meaning are
   allowed).
5. **Pre-release deletion only with logic revert.** A `BREAKING:`
   bullet under `## Development` may be removed only if the same
   change reverts the breaking logic. "Decided not to mention" is
   not a valid reason.

### Step 1 - Auto-merge pass (silent)

Scan `entries` for these patterns. Apply matches silently; track
`pass1_count` (groups merged) and `dropped_count` (entries removed
in net-zero pairs).

**Pattern 1 - Rename + subsequent change:** entry A renames `X` to
`Y`, entry B describes a change to `Y`. Merge into one entry
describing the final state.

**Pattern 2 - Add + Remove (net-zero):** entry A adds feature `X`,
entry B explicitly removes or reverts `X`. Drop both.

**Pattern 3 - Add + Extend:** entry A introduces feature `X`,
entry B extends or refines `X` (same component, high overlap in key
terms). Merge into one entry describing the final state.

Never apply these to `BREAKING:` entries (see rule 1 above).

### Step 2 - Triage into buckets

Classify every surviving entry into exactly one bucket:

| Bucket | What goes there | Examples |
|---|---|---|
| **drop** | Internal-only changes invisible to users. Refactors that do not change behaviour. Test-only changes. CI-only changes. Doc-only changes that do not announce new capabilities. Code-style fixes. Comment polish. Dependency bumps without user impact. | "refactor: extract helper", "test: add table cases", "ci: bump golangci-lint", "chore: tidy go.mod", "docs: fix typo" |
| **break** | Already-tagged `BREAKING:` bullets. | "BREAKING: rename `--vault` to `--vault-name`" |
| **feat** | New user-visible capability: new command, new flag, new config field, new behaviour, new platform support. One-liner per feature; if the feature is complex, the bullet points at `docs/` rather than describing it in full. | "add `locksmith mcp run` with proxy + local modes (see `docs/configuration.md`)" |
| **fix** | Behavioural change that corrects a bug, regression, security issue, or visibly wrong output. | "fix(keychain): handle errSecUserCanceled from Touch ID prompt" |

Rules for triage:

- A `fix:` Conventional-Commits type is a strong signal for **fix**,
  but classify by user impact, not by the commit verb. A "fix"
  that is actually an internal cleanup is **drop**.
- A `feat:` commit that is wholly internal (e.g. "feat(internal):
  add helper") is **drop**.
- Doc-only entries are **drop** unless they announce a new
  capability that did not exist before (rare; usually paired with
  code).
- When in doubt, lean towards **drop**. The git log is the
  exhaustive record; the changelog is the highlight reel.

For each entry that is ambiguous (e.g. could be either **drop** or
**fix** depending on user impact), ask the user before deciding.
Show:

```
Entry: <original bullet>
  Suggested: <bucket>
  Choose: [drop / feat / fix]
```

Accept `drop`, `feat`, `fix`, or their single-letter prefixes
(`d`, `feat`, `fix`). Track `pass2_count` = number of ambiguous
entries the user resolved.

### Step 3 - Compress within buckets

Within **feat** and **fix**:

- One bullet per logical change. If multiple bullets describe
  pieces of the same feature, merge into one bullet pointing at the
  final state.
- One line per bullet. No wrapped paragraphs.
- If a feature is complex enough to need explanation, the bullet
  reads like: `add foo (see docs/configuration.md#foo)`. Do not
  inline the explanation - the changelog is not the docs.
- Use plain English, present tense, imperative voice ("add", "fix",
  not "added", "fixes").

Within **break**: no compression. Each `BREAKING:` bullet stays as
authored.

### Step 4 - Ask for version

Ask the user for the version number (e.g. `v0.3.0`) and release
date (default: today, `YYYY-MM-DD`).

### Step 5 - Preview

Show a final preview with two parts: what's being written, and
what's being dropped.

```
Final entry for vX.Y.Z - YYYY-MM-DD:

  ## Version vX.Y.Z - YYYY-MM-DD

  ### BREAKING CHANGES
  - BREAKING: ...

  ### Features
  - ...

  ### Fixes
  - ...

Dropped from changelog (still in git log):
  - <original bullet>
  - <original bullet>

Counts:
  Auto-merged:   <pass1_count> groups
  Resolved:      <pass2_count> ambiguous entries
  Net-zero:      <dropped_count> bullets
  Triaged:       <drop_count> drops, <feat_count> feats, <fix_count> fixes, <break_count> breaks
  Entries: <original_count> -> <final_count>

Write CHANGES.md? [yes / edit]
```

If `edit`, ask what to change and apply before writing.

### Step 6 - Write

Replace the `## Development` section with:

1. A new empty `## Development` section at the top.
2. A new `## Version vX.Y.Z - YYYY-MM-DD` section below it
   containing the three subsections (omit any subsection that has
   no entries).

Write `CHANGES.md`.

### Notes

- Subsections are omitted if empty. A release with only fixes
  prints only `### Fixes`. A release with no BREAKING changes does
  not print `### BREAKING CHANGES`.
- Never include AI-tool names in any bullet.
- If the user disputes a triage decision after the preview,
  re-classify and reshow the preview. After three dispute rounds,
  ask the user to authoritatively pick a bucket per entry to
  prevent loops.
