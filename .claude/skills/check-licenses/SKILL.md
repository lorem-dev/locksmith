---
name: check-licenses
description: Audit direct third-party Go dependencies for license compatibility with Apache 2.0, present them for user verification, and maintain the Third-Party Notices section in LICENSE
---

## Instructions

**Announce at start:** "I'm using the check-licenses skill."

### Step 1 - Determine mode

Read `LICENSE` and check for a `## Third-Party Notices` heading.

- **Absent** - full-scan mode: collect all direct dependencies from every
  `go.mod` in the workspace.
- **Present** - incremental mode: run
  `git diff HEAD~1 -- go.mod sdk/go.mod plugins/gopass/go.mod plugins/keychain/go.mod`
  to find added direct dependencies only. If `HEAD~1` does not exist (initial
  commit), fall back to full-scan mode.

In incremental mode, if the diff shows no new direct dependencies, report
"No new dependencies found" and stop.

### Step 2 - Collect direct dependencies

Parse every `go.mod` in the workspace:
- `go.mod`
- `sdk/go.mod`
- `plugins/gopass/go.mod`
- `plugins/keychain/go.mod`

A line is a **direct dependency** if:
- It is inside a `require (...)` block or a single-line `require`
- It does NOT end with `// indirect`
- The module path does NOT start with `github.com/lorem-dev/locksmith`
  (those are internal `replace` targets, not third-party)

Deduplicate across files: if the same module path appears in multiple
`go.mod` files, keep the entry once at the highest version seen.

### Step 3 - Look up licenses and build the verification table

For each dependency, determine:

1. **License SPDX identifier** - look at `https://pkg.go.dev/<module>` (the
   "Licenses" field on the overview page).
2. **License URL** - direct link to the `LICENSE` file in the upstream
   repository at the tagged version. For `github.com/<org>/<repo>` modules
   this is `https://github.com/<org>/<repo>/blob/<version>/LICENSE`.
   For `golang.org/x/*` modules the GitHub mirror is
   `https://github.com/golang/<name>` (e.g. `golang.org/x/term` ->
   `github.com/golang/term`). For `google.golang.org/grpc` use
   `https://github.com/grpc/grpc-go`. For `google.golang.org/protobuf` use
   `https://github.com/protocolbuffers/protobuf-go`.
3. **NOTICE URL** - check whether a `NOTICE` file exists at the same tagged
   version. Include only if it exists.

Display a Markdown table:

```
| Library                         | Version | License    | License URL                                                              |
|---------------------------------|---------|------------|--------------------------------------------------------------------------|
| github.com/charmbracelet/huh    | v1.0.0  | MIT        | https://github.com/charmbracelet/huh/blob/v1.0.0/LICENSE                 |
| ...                             | ...     | ...        | ...                                                                      |
```

Then ask:
> "Please check each license via the links above. Reply `yes` to continue
> or `no` to abort."

Wait for an explicit `yes` or `no`. Do **not** proceed until the user
responds. If the user replies `no`, stop immediately and make no changes.

### Step 4 - Block-list check

After the user confirms, check each SPDX identifier against this list.
If any match, **stop with an error** naming the offending library and
suggesting it be removed from `go.mod`. Do not modify any file.

```
GPL-2.0  GPL-2.0-only  GPL-2.0-or-later
GPL-3.0  GPL-3.0-only  GPL-3.0-or-later
AGPL-3.0  AGPL-3.0-only  AGPL-3.0-or-later
LGPL-2.1  LGPL-2.1-only
CC-BY-NC-4.0  (any -NC- variant)
SSPL-1.0
BSL-1.1
Commons Clause  (as a modifier on any license)
```

If a license is **unknown or ambiguous**, stop and ask the user to verify
it manually before continuing.

Licenses that are always acceptable: MIT, Apache-2.0, BSD-2-Clause,
BSD-3-Clause, ISC, MPL-2.0, Unlicense, CC0-1.0, BlueOak-1.0.0.

### Step 5 - Update `LICENSE`

Append or update the `## Third-Party Notices` section at the end of
`LICENSE`.

The section header:

```
---

## Third-Party Notices

This project uses the following third-party libraries. Each library
retains its original copyright and is distributed under its respective
license.
```

For each dependency, add an entry sorted **alphabetically by module path**:

```
### <module-path>
- Version: <version>
- License: <SPDX identifier>
- Source: https://<module-path>
- License text: <URL to LICENSE file at tagged version>
```

If a NOTICE file was found in Step 3, add a line:
```
- NOTICE: <URL to NOTICE file at tagged version>
```

In incremental mode, insert new entries in alphabetical order within the
existing list. Do not duplicate entries that are already present.

### Step 6 - Update `CONTRIBUTING.md`

Check whether `CONTRIBUTING.md` already contains a `## License Compliance`
heading. If it does, skip this step.

If it is absent, insert the following section **before** `## GPG Signing`:

```markdown
## License Compliance

All direct third-party dependencies must carry a license compatible with
Apache 2.0 before being merged.

**When adding a new library:**
1. Run the `check-licenses` skill after editing `go.mod`.
2. Review the license table the skill displays and confirm each entry.
3. The skill updates `LICENSE` automatically.

**Licenses that are NOT acceptable** (prohibit commercial use or impose
copyleft incompatible with Apache 2.0):
- GPL-2.0 / GPL-3.0 / AGPL-3.0
- LGPL-2.1 (Go links statically, so LGPL's dynamic-linking exception does
  not apply)
- SSPL-1.0 / BSL-1.1
- Any Creative Commons -NC- variant
- Any license containing a "Commons Clause" addendum

If the library you need carries one of these licenses, look for a
permissive alternative or raise the issue with the maintainers before
adding it.
```

### Step 7 - Commit

Stage and commit changed files:

```bash
git add LICENSE CONTRIBUTING.md
git commit -S -m "chore: update third-party notices and license compliance docs"
```

If `CONTRIBUTING.md` was not modified (section already existed), commit
only `LICENSE`:

```bash
git add LICENSE
git commit -S -m "chore: update third-party notices"
```
