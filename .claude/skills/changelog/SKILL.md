---
name: changelog
description: Compress development changes into a versioned CHANGES.md entry before cutting a release
---

## Instructions

Read CHANGES.md and identify all items listed under `## Development`.

Summarize those items into a concise bullet list — group related changes,
remove redundancy, keep each bullet to one line.

Ask the user for the version number (e.g. v0.1.0) and release date.

Replace the `## Development` section with:
1. A new empty `## Development` section at the top
2. A new `## Version <version> — <date>` section below it containing the compressed bullet list

Write the updated CHANGES.md. Show a diff summary to the user before saving.

Do not include AI tool names in the changelog.
