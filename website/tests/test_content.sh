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

contains() {
  grep -q "$1" "$2"
}

echo "=== Content Tests ==="

# Check index.html contains expected text
check "index.html contains 'Spade'" contains "Spade" public/index.html
check "index.html contains tagline" contains "Data processing at scale" public/index.html
check "index.html contains 'Features'" contains "Features" public/index.html
check "index.html contains 'How It Works'" contains "How It Works" public/index.html
check "index.html contains 'Get Started'" contains "Get Started" public/index.html
check "index.html contains 'What is Spade'" contains "What is Spade" public/index.html
check "index.html contains 'Ready to Play'" contains "Ready to Play" public/index.html

# Check feature pages exist
check "multi-language feature page exists" test -f public/features/multi-language/index.html
check "pipelines feature page exists" test -f public/features/pipelines/index.html
check "geospatial feature page exists" test -f public/features/geospatial/index.html
check "map-reduce feature page exists" test -f public/features/map-reduce/index.html
check "security feature page exists" test -f public/features/security/index.html
check "cli feature page exists" test -f public/features/cli/index.html

# Check use case pages exist
check "remote-sensing use case exists" test -f public/use-cases/remote-sensing/index.html
check "geospatial-etl use case exists" test -f public/use-cases/geospatial-etl/index.html
check "scientific-pipelines use case exists" test -f public/use-cases/scientific-pipelines/index.html

echo ""
echo "Content tests: $PASS passed, $FAIL failed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
