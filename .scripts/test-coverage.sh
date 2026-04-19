#!/usr/bin/env bash
# Run 'go test' with coverage for every module in the Go workspace.
# Saves per-module HTML reports to .reports/ and prints a summary table.
set -e

ROOT=$(cd "$(dirname "$0")/.." && pwd)
REPORTS="$ROOT/.reports"
mkdir -p "$REPORTS"

PASS=0
FAIL=0
RESULTS=""

# Filter known harmless macOS linker warnings from stderr while preserving
# the exit code of the go test invocation itself.
go_test() { "$@" 2> >(grep -Ev "malformed LC_DYSYMTAB" >&2); }

while IFS= read -r mod; do
  dir="$ROOT/$mod"
  [ "$mod" = "." ] && dir="$ROOT"

  # Profile filename: strip leading "./" and replace "/" with "-"
  name=$(printf '%s' "$mod" | sed 's|^\./||; s|/|-|g')
  [ "$name" = "." ] && name="root"

  profile="$REPORTS/coverage-${name}.out"
  html="$REPORTS/coverage-${name}.html"

  printf '\033[90m--- %s\033[0m\n' "$mod"
  if (cd "$dir" && go_test go test -timeout "${TEST_TIMEOUT:-3m}" -coverprofile="$profile" -covermode=atomic ./...); then
    printf '    \033[32mPASS\033[0m\n\n'
    PASS=$((PASS + 1))
    go tool cover -html="$profile" -o "$html" 2>/dev/null || true
    pct=$(go tool cover -func="$profile" 2>/dev/null | awk '/^total:/{print $3}' | tr -d '%')
    RESULTS="${RESULTS}${mod}:${pct:-0}\n"
  else
    printf '    \033[31mFAIL\033[0m\n\n'
    FAIL=$((FAIL + 1))
    RESULTS="${RESULTS}${mod}:FAIL\n"
  fi
done < <(cd "$ROOT" && go run ./.scripts/workspace-modules)

# Summary table
printf '\033[1m\033[36m=== coverage summary ===\033[0m\n'
printf '%b' "$RESULTS" | while IFS=: read -r mod pct; do
  [ -z "$mod" ] && continue
  if [ "$pct" = "FAIL" ]; then
    printf '    \033[31m%-30s  FAIL\033[0m\n' "$mod"
  else
    int_pct=$(echo "$pct" | awk '{printf "%d", $1}' 2>/dev/null || echo 0)
    if [ "$int_pct" -ge 90 ]; then
      color='\033[32m'
    elif [ "$int_pct" -gt 0 ]; then
      color='\033[33m'
    else
      color='\033[90m'
    fi
    printf "    %-30s  ${color}%s%%\033[0m\n" "$mod" "$pct"
  fi
done

printf '\033[90mHTML reports written to .reports/\033[0m\n\n'

TOTAL=$((PASS + FAIL))
if [ "$FAIL" -eq 0 ]; then
  printf '\033[1m\033[32m=== %d/%d modules passed ===\033[0m\n' "$PASS" "$TOTAL"
else
  printf '\033[1m\033[31m=== %d/%d modules passed ===\033[0m\n' "$PASS" "$TOTAL"
  exit 1
fi
