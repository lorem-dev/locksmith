---
name: changelog
description: Compress development changes into a versioned CHANGES.md entry before cutting a release
---

## Instructions

Read CHANGES.md and identify all items listed under `## Development`. Call this list `entries`.

If `entries` is empty, skip to "Ask for version".

---

### Pass 1: Auto-merge (apply silently, no user prompt)

Scan `entries` for these patterns. For each match, replace the involved entries with the merged result. Track: `pass1_count` (groups merged), `dropped_count` (net-zero pairs removed).

**Pattern 1 - Rename + subsequent change:**
If entry A describes a rename (`X renamed to Y`) and entry B mentions `Y` as the subject of a further change, merge A and B into one entry describing the final state.

Example:
- `SDK: HideSession renamed to HideSessionId (breaking change)` + `SDK: new MaskSessionId function for log call sites`
- Becomes: `SDK: HideSession renamed to HideSessionId (breaking change); added MaskSessionId for log call sites.`

**Pattern 2 - Add + Remove (net-zero):**
If entry A adds feature/symbol X and entry B explicitly removes, reverts, or undoes X, drop both entries entirely. Increment `dropped_count` by the number of removed entries.

**Pattern 3 - Add + Extend:**
If entry A introduces feature X and entry B extends, refines, or documents the same feature (same component, high overlap in key terms), merge into one entry describing the complete final state.

If no matches are found in Pass 1, proceed to Pass 2 without any output. Do not tell the user "no conflicts found" - just proceed silently.

---

### Pass 2: Ambiguous groups (confirm with user)

Scan remaining `entries` for groups where 2 or more entries share the same component, module, config key, or named feature but do not match a clear Pass 1 pattern. Track: `pass2_count` (groups confirmed or edited by user, not kept-as-is).

For each ambiguous group, show:

```
Found overlap:
  BEFORE:
    - [original entry 1]
    - [original entry 2]
    ...

  PROPOSED:
    - [single merged entry describing the final observable state]

Accept? [yes / edit / keep as-is]
```

Wait for the user's response before moving to the next group:
- `yes` - use the proposed merged entry; increment `pass2_count`
- `edit` - user provides replacement text; use it; increment `pass2_count`
- `keep as-is` - all original entries remain unchanged; do not increment `pass2_count`

If no ambiguous groups are found, skip Pass 2 silently.

---

### Ask for version

Ask the user for the version number (e.g. v0.1.0) and release date.

---

### Summarize

Summarize the final `entries` into a concise bullet list - group any remaining related changes, remove remaining redundancy, keep each bullet to one line.

---

### Diff summary

Before writing, show:

```
Summary:
  Auto-merged (Pass 1):  <pass1_count> groups
  Confirmed by you:      <pass2_count> groups
  Dropped (net-zero):    <dropped_count> entries
  Entries: <original_count> -> <final_count>

Final list for <version> - <date>:
  - [bullet 1]
  - [bullet 2]
  ...

Write CHANGES.md? [yes / edit]
```

If the user says `edit`, ask what to change and apply it before writing.

---

### Write

Replace the `## Development` section with:
1. A new empty `## Development` section at the top
2. A new `## Version <version> - <date>` section below it containing the compressed bullet list

Write the updated CHANGES.md.

Do not include AI tool names in the changelog.
