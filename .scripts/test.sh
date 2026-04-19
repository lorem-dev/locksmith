#!/usr/bin/env bash
# Run 'go test ./...' for every module in the Go workspace.
set -e

ROOT=$(cd "$(dirname "$0")/.." && pwd)
PASS=0
FAIL=0

# Filter known harmless macOS linker warnings from stderr while preserving
# the exit code of the go test invocation itself.
go_test() { "$@" 2> >(grep -Ev "malformed LC_DYSYMTAB" >&2); }

while IFS= read -r mod; do
  dir="$ROOT/$mod"
  [ "$mod" = "." ] && dir="$ROOT"

  printf '\033[90m--- %s\033[0m\n' "$mod"
  if (cd "$dir" && go_test go test -timeout "${TEST_TIMEOUT:-3m}" ./...); then
    printf '    \033[32mPASS\033[0m\n\n'
    PASS=$((PASS + 1))
  else
    printf '    \033[31mFAIL\033[0m\n\n'
    FAIL=$((FAIL + 1))
  fi
done < <(cd "$ROOT" && go run ./.scripts/workspace-modules)

TOTAL=$((PASS + FAIL))
if [ "$FAIL" -eq 0 ]; then
  printf '\033[1m\033[32m=== %d/%d modules passed ===\033[0m\n' "$PASS" "$TOTAL"
else
  printf '\033[1m\033[31m=== %d/%d modules passed ===\033[0m\n' "$PASS" "$TOTAL"
  exit 1
fi
