#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

PASS=0
FAIL=0

check() {
  local desc="$1"
  shift
  if "$@" > /dev/null 2>&1; then
    echo "  PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $desc"
    FAIL=$((FAIL + 1))
  fi
}

echo "=== Accessibility Tests ==="

# Check that img tags have alt attributes (if any img tags exist)
img_count="$(grep -c '<img ' public/index.html 2>/dev/null || true)"
img_count="${img_count:-0}"
if [ "$img_count" -gt 0 ]; then
  img_no_alt="$(grep '<img ' public/index.html | grep -cv 'alt=' || true)"
  img_no_alt="${img_no_alt:-0}"
  check "all img tags have alt attributes" test "$img_no_alt" -eq 0
else
  echo "  SKIP: no img tags found in index.html"
fi

# Check that h1 exists
check "index.html has h1 heading" grep -q '<h1' public/index.html

# Check nav has aria-label on toggle
check "nav toggle has aria-label" grep -q 'aria-label' public/index.html

# Check html has lang attribute
check "html tag has lang attribute" grep -q 'lang="en"' public/index.html

echo ""
echo "Accessibility tests: $PASS passed, $FAIL failed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
