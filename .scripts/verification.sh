#!/usr/bin/env bash
# Run all quality gates: lint, tests (race + coverage), build, GPG signatures,
# docs completeness, and CHANGES.md. Exits 1 if any gate fails.
# Use this as the final step before merging a feature branch.
set -uo pipefail   # intentionally no -e: we handle errors gate-by-gate

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

PASS=0
FAIL=0
ISSUES=""

# ── helpers ───────────────────────────────────────────────────────────────────

gate_pass() { PASS=$((PASS+1)); printf '\033[32m  PASS\033[0m  %s\n' "$1"; }
gate_fail() { FAIL=$((FAIL+1)); printf '\033[31m  FAIL\033[0m  %s\n' "$1"; ISSUES="${ISSUES}  - $1\n"; }
header()    { printf '\n\033[1m\033[36m=== %s ===\033[0m\n' "$1"; }

# Resolve GOBIN so tools installed via 'go install' are always found.
GOBIN=$(go env GOBIN)
[ -z "$GOBIN" ] && GOBIN=$(go env GOPATH)/bin
export PATH="$GOBIN:$PATH"

# ── 1. Build ──────────────────────────────────────────────────────────────────

header "BUILD"
if make build 2>&1; then
  gate_pass "make build"
else
  gate_fail "make build - fix compilation errors above"
fi

# ── 2. Lint ───────────────────────────────────────────────────────────────────

header "LINT"
if make lint 2>&1; then
  gate_pass "make lint"
else
  gate_fail "make lint - fix lint errors above (run 'make lint' for details)"
fi

# ── 3. Tests with race detector ───────────────────────────────────────────────

header "TESTS (race detector)"
if ./.scripts/test-race.sh 2>&1; then
  gate_pass "test-race"
else
  gate_fail "test-race - fix race conditions or test failures above"
fi

# ── 4. Coverage ───────────────────────────────────────────────────────────────

header "COVERAGE"
COVERAGE_OUT=$(./.scripts/test-coverage.sh 2>&1) || true
printf '%s\n' "$COVERAGE_OUT"

# Parse only the summary table rows produced by test-coverage.sh.
# Those lines look like (with leading 4 spaces):
#     .                   62.0%
#     ./plugins/gopass    97.6%
#
# Skip the root "." row (bundles cmd/ packages with 0% that skew the total).
# Flag any non-root module whose coverage is between 1% and 89%.
BELOW=""
while IFS= read -r line; do
  # Match only summary table rows: 4-space indent followed by a path starting with "."
  echo "$line" | grep -qE '^    \.' || continue
  # Skip the root-module aggregate line (exactly "    .")
  echo "$line" | grep -qE '^    \. ' || echo "$line" | grep -qE '^    \.$' && continue
  # Extract percentage (last token ending in %)
  pct=$(echo "$line" | grep -oE '[0-9]+\.[0-9]+%' | tail -1 | tr -d '%')
  [ -z "$pct" ] && continue
  pkg=$(echo "$line" | awk '{print $1}')
  int_pct=$(echo "$pct" | awk '{printf "%d", $1}')
  if [ "$int_pct" -gt 0 ] && [ "$int_pct" -lt 90 ]; then
    BELOW="${BELOW}    ${pkg}: ${pct}%\n"
  fi
done <<< "$COVERAGE_OUT"

if [ -z "$BELOW" ]; then
  gate_pass "coverage >= 90% in all tested packages"
else
  gate_fail "coverage below 90% in the following packages (add tests):"
  printf '\033[33m%b\033[0m' "$BELOW"
fi

# ── 5. GPG signatures ─────────────────────────────────────────────────────────

header "GPG SIGNATURES"
# Check commits on this branch that haven't been pushed to origin/main yet.
REMOTE_BASE=$(git for-each-ref --format='%(refname:short)' refs/remotes/origin/HEAD 2>/dev/null | head -1)
if [ -n "$REMOTE_BASE" ]; then
  COMMITS=$(git log --format="%G? %h %s" "${REMOTE_BASE}..HEAD" 2>/dev/null || true)
else
  COMMITS=$(git log --format="%G? %h %s" | head -20 || true)
fi

UNSIGNED=$(printf '%s\n' "$COMMITS" | { grep -v "^G " || true; })
if [ -z "$UNSIGNED" ]; then
  gate_pass "all commits on this branch are GPG-signed"
else
  gate_fail "unsigned or badly-signed commits (re-sign before merging):"
  printf '\033[33m%s\033[0m\n' "$UNSIGNED"
fi

# ── 6. CHANGES.md ─────────────────────────────────────────────────────────────

header "CHANGES.md"
if ! [ -f CHANGES.md ]; then
  gate_fail "CHANGES.md not found - create it and add entries for all user-visible changes"
elif grep -q "^## Development" CHANGES.md; then
  # Count bullet lines in the Development section only (stop at next ## heading)
  DEV_SECTION=$(sed -n '/^## Development/,/^## [^D]/{ /^## [^D]/d; p; }' CHANGES.md)
  ENTRIES=$(printf '%s\n' "$DEV_SECTION" | { grep -c "^- " || true; })
  if [ "$ENTRIES" -gt 0 ]; then
    WORD=$([ "$ENTRIES" -eq 1 ] && echo "entry" || echo "entries")
    gate_pass "CHANGES.md has $ENTRIES $WORD under ## Development"
  else
    gate_fail "CHANGES.md has a '## Development' section but no bullet entries - add at least one entry describing what changed"
  fi
else
  gate_fail "CHANGES.md is missing a '## Development' section - add entries for all user-visible changes in this branch"
fi

# ── 7. Docs completeness (new / changed functionality) ────────────────────────

header "DOCS COMPLETENESS"

# Compute merge base against origin/main (or main if no remote).
BASE=$(git merge-base HEAD origin/main 2>/dev/null \
    || git merge-base HEAD main 2>/dev/null \
    || echo "HEAD~1")

CHANGED=$(git diff --name-only "$BASE" HEAD 2>/dev/null || true)

# Non-test Go files outside gen/ and docs/ indicate functional changes.
FUNC_CHANGED=$(printf '%s\n' "$CHANGED" \
  | { grep -E '\.go$' || true; } \
  | { grep -v '_test\.go' || true; } \
  | { grep -Ev '^(gen/|docs/)' || true; })

if [ -z "$FUNC_CHANGED" ]; then
  gate_pass "no functional Go files changed - no doc update required"
else
  DOC_CHANGED=$(printf '%s\n' "$CHANGED" | { grep -E '^(docs/|README\.md)' || true; })
  if [ -n "$DOC_CHANGED" ]; then
    gate_pass "docs updated alongside functional changes"
    printf '%s\n' "$DOC_CHANGED" | sed 's/^/    /'
  else
    gate_fail "functional Go files changed but docs not updated - update README.md or docs/ to document new/changed behaviour"
    printf '\033[33m    functional files changed (first 10):\n'
    printf '%s\n' "$FUNC_CHANGED" | head -10 | sed 's/^/      /'
    printf '\033[0m'
  fi
fi

# ── Summary ───────────────────────────────────────────────────────────────────

TOTAL=$((PASS+FAIL))
printf '\n'
if [ "$FAIL" -eq 0 ]; then
  printf '\033[1m\033[32m=== ALL %d GATES PASSED ===\033[0m\n' "$TOTAL"
  exit 0
else
  printf '\033[1m\033[31m=== %d/%d GATES FAILED ===\033[0m\n' "$FAIL" "$TOTAL"
  printf '\n\033[1mIssues to fix before merging:\033[0m\n'
  printf '%b' "$ISSUES"
  exit 1
fi
