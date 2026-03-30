#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==============================="
echo " Spade Website Test Suite"
echo "==============================="
echo ""

TOTAL_FAIL=0

run_test() {
  local script="$1"
  if bash "$script"; then
    echo ""
  else
    TOTAL_FAIL=$((TOTAL_FAIL + 1))
    echo ""
  fi
}

run_test "$SCRIPT_DIR/test_build.sh"
run_test "$SCRIPT_DIR/test_content.sh"
run_test "$SCRIPT_DIR/test_accessibility.sh"

echo "==============================="
if [ "$TOTAL_FAIL" -gt 0 ]; then
  echo " RESULT: $TOTAL_FAIL test suite(s) FAILED"
  exit 1
else
  echo " RESULT: All test suites PASSED"
  exit 0
fi
