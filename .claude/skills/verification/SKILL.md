---
name: verification
description: Run all quality gates (lint, race tests, coverage >= 90%, build, GPG signatures, docs completeness, CHANGES.md) as the mandatory final step of every superpowers plan. Interprets failures and tells Claude exactly what to fix.
---

**Announce at start:** "I'm using the verification skill."

## Purpose

This skill is the mandatory last task in every superpowers plan. It runs `.scripts/verification.sh`,
interprets each failed gate, and - if CHANGES.md needs compressing - offers to invoke the
`changelog` skill.

---

## Step 1 - Run verification script

```bash
bash .scripts/verification.sh
```

Capture the full output. Note the exit code.

If exit code is 0 and the output ends with `=== ALL N GATES PASSED ===`, skip to Step 3.

---

## Step 2 - Fix each failed gate

For every `FAIL` line in the output, apply the fix described below, then re-run
`bash .scripts/verification.sh` after each batch. Repeat until all gates pass.

| Gate | What to do |
|------|-----------|
| `make build` | Fix every compilation error shown above the gate line. Check imports, types, missing fields. |
| `make lint` | Run `make lint` to see current errors. Fix each one - do not add `//nolint:` without a comment explaining why. Re-run until clean. |
| `test-race` | Find the failing test(s) in the output above the gate line. Fix the root cause. Do NOT suppress the race detector with `runtime.GOMAXPROCS(1)` or similar hacks. |
| coverage below 90% | Open `.reports/coverage-<module>.html` for the listed package. Add tests for uncovered lines. Aim for lines that test real behaviour, not just coverage points. |
| GPG signatures | Do NOT re-sign automatically. Report the unsigned commits to the user, show the exact commits, and ask for explicit confirmation before running the re-sign command from CLAUDE.md. |
| CHANGES.md missing `## Development` | Add the section at the top of CHANGES.md with one bullet per user-visible change introduced on this branch. |
| CHANGES.md has no bullet entries | Add bullets under `## Development` - one per logical change: what changed and why, in plain English. |
| docs not updated | Identify which new or changed behaviour is undocumented. Add a section to `README.md` (user-facing changes) or the relevant file under `docs/` (architecture/config changes). |

---

## Step 3 - Changelog offer

After all gates pass, count items under `## Development` in CHANGES.md:

```bash
sed -n '/^## Development/,/^## [^D]/{ /^## [^D]/d; p; }' CHANGES.md | grep -c "^- " || true
```

If the count is **5 or more**, offer:

> "There are N entries under ## Development in CHANGES.md. Would you like me to run the
> changelog skill to compress them into a versioned release entry?"

Wait for the user's answer. Do NOT invoke the changelog skill automatically.

If the count is fewer than 5, skip this step silently.

---

## Step 4 - Final GPG check

After all gates pass (and changelog is handled if applicable), run:

```bash
git log --format="%G? %h %s" | head -20
```

- Every line starts with `G` - report "All commits signed. Verification complete."
- Any line starts with `N` or `B` - list those commits and ask the user for confirmation
  before re-signing. Use the re-sign command from CLAUDE.md.
