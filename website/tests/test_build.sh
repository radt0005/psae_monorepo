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

echo "=== Build Tests ==="

# Clean and rebuild
rm -rf public/
zola build > /dev/null 2>&1
check "zola build exits successfully" test $? -eq 0

check "public/ directory exists" test -d public
check "public/index.html exists and is non-empty" test -s public/index.html
check "public/style.css exists" test -f public/style.css
check "public/features/index.html exists" test -f public/features/index.html
check "public/use-cases/index.html exists" test -f public/use-cases/index.html
check "public/404.html exists" test -f public/404.html
check "public/robots.txt exists" test -f public/robots.txt
check "public/sitemap.xml exists" test -f public/sitemap.xml

echo ""
echo "Build tests: $PASS passed, $FAIL failed"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
