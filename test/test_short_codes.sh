#!/usr/bin/bash
#
# test_short_codes.sh -- Integration tests for hand-authored pipelines
# that use `@<identifier>` short codes and the sibling lockfile.
#
# Templates live in ./short_code_pipelines/.  They reference fixtures
# via a `__FIXTURES_DIR__` placeholder which this script substitutes
# into working copies under ./short_code_pipelines/work/ before
# running.  Lockfiles end up alongside the working copies, never
# alongside the templates -- so reruns start from a known state.
#
# Usage:
#   ./test_short_codes.sh           Run all short-code tests
#   ./test_short_codes.sh --check   Only validate (spade check), don't run
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATES_DIR="$SCRIPT_DIR/short_code_pipelines"
WORK_DIR="$TEMPLATES_DIR/work"
FIXTURES_DIR="$SCRIPT_DIR/fixtures"
LOGS_DIR="$SCRIPT_DIR/logs/short_codes"

# Resolve spade binary -- prefer the repo-local build, fall back to PATH.
if [[ -x "$SCRIPT_DIR/../cli/spade" ]]; then
    SPADE="$SCRIPT_DIR/../cli/spade"
elif command -v spade &>/dev/null; then
    SPADE="$(command -v spade)"
else
    echo "ERROR: spade binary not found. Build it with: cd ../cli && go build -o spade ."
    exit 1
fi

MODE="run"
for arg in "$@"; do
    case "$arg" in
        --check) MODE="check" ;;
        -h|--help)
            echo "Usage: $0 [--check]"
            echo ""
            echo "  (default)  Validate (spade check) and execute (spade run) each pipeline"
            echo "  --check    Only validate pipelines; skip spade run"
            exit 0
            ;;
        *)
            echo "Unknown flag: $arg" >&2
            exit 1
            ;;
    esac
done

# ---------------------------------------------------------------------------
# Prepare working copies
# ---------------------------------------------------------------------------
echo "=== Preparing working copies in $WORK_DIR ==="
mkdir -p "$WORK_DIR" "$LOGS_DIR"
# Clean slate: any leftover yaml or lockfile from a previous run.
rm -f "$WORK_DIR"/*.yaml "$WORK_DIR"/*.lock.yaml

for template in "$TEMPLATES_DIR"/*.yaml; do
    name="$(basename "$template")"
    sed "s|__FIXTURES_DIR__|$FIXTURES_DIR|g" "$template" > "$WORK_DIR/$name"
done
echo "Prepared $(ls "$WORK_DIR"/*.yaml | wc -l) pipeline(s)."
echo ""

# Some block invocations (data.write* etc) run inside an isolate
# sandbox under a remapped uid; the fixtures dir must be world-readable.
chmod 0755 "$FIXTURES_DIR" 2>/dev/null || true

# ---------------------------------------------------------------------------
# Test harness helpers
# ---------------------------------------------------------------------------
PASS=0
FAIL=0
FAILED_TESTS=()

report() {
    local label="$1"
    local status="$2"
    printf "  %-60s %s\n" "$label" "$status"
    if [[ "$status" == "PASS" ]]; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
        FAILED_TESTS+=("$label")
    fi
}

# Count `@<ident>` occurrences in a file.  Used to confirm the
# walker substituted every short code in the source pipeline.
count_short_codes() {
    grep -oE '@[A-Za-z_][A-Za-z0-9_]*' "$1" 2>/dev/null | wc -l
}

# Count UUIDv7-shaped strings.  After resolution every short code
# should have become a UUID, so the resolved pipeline contains
# exactly as many UUIDs as the original had distinct short codes
# (plus any pre-existing UUIDs).
count_uuids() {
    grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}' "$1" 2>/dev/null | wc -l
}

# ---------------------------------------------------------------------------
# Test 1: spade check on a fresh short-code pipeline writes a lockfile
# ---------------------------------------------------------------------------
test_check_creates_lockfile() {
    local pipeline="$WORK_DIR/sc_base_filter.yaml"
    local lockfile="$WORK_DIR/sc_base_filter.lock.yaml"
    local log="$LOGS_DIR/01_check_creates_lockfile.log"

    rm -f "$lockfile"
    if "$SPADE" check "$pipeline" > "$log" 2>&1 \
        && [[ -f "$lockfile" ]] \
        && grep -q "Wrote .*sc_base_filter.lock.yaml" "$log" \
        && grep -q "is valid" "$log"; then
        report "spade check creates lockfile on first run" "PASS"
    else
        report "spade check creates lockfile on first run" "FAIL"
    fi
}

# ---------------------------------------------------------------------------
# Test 2: lockfile contains a binding for every short code in the source
# ---------------------------------------------------------------------------
test_lockfile_covers_all_codes() {
    local pipeline="$TEMPLATES_DIR/sc_base_filter.yaml"
    local lockfile="$WORK_DIR/sc_base_filter.lock.yaml"
    local log="$LOGS_DIR/02_lockfile_covers_all_codes.log"

    [[ -f "$lockfile" ]] || "$SPADE" check "$WORK_DIR/sc_base_filter.yaml" > "$log" 2>&1

    # Unique short codes used in the template
    local source_codes
    source_codes=$(grep -oE '@[A-Za-z_][A-Za-z0-9_]*' "$pipeline" | sort -u | wc -l)
    # Bindings recorded in the lockfile
    local locked_codes
    locked_codes=$(grep -cE '^\s*"?@[A-Za-z_]' "$lockfile" || true)

    if [[ "$source_codes" -eq "$locked_codes" ]] && [[ "$source_codes" -gt 0 ]]; then
        report "lockfile binds every short code in the source" "PASS"
    else
        report "lockfile binds every short code in the source (src=$source_codes lock=$locked_codes)" "FAIL"
    fi
}

# ---------------------------------------------------------------------------
# Test 3: second check leaves the lockfile byte-identical (stability)
# ---------------------------------------------------------------------------
test_lockfile_stable() {
    local pipeline="$WORK_DIR/sc_base_filter.yaml"
    local lockfile="$WORK_DIR/sc_base_filter.lock.yaml"
    local log="$LOGS_DIR/03_lockfile_stable.log"

    [[ -f "$lockfile" ]] || "$SPADE" check "$pipeline" > "$log" 2>&1
    local before
    before=$(sha256sum "$lockfile" | awk '{print $1}')

    "$SPADE" check "$pipeline" >> "$log" 2>&1
    local after
    after=$(sha256sum "$lockfile" | awk '{print $1}')

    if [[ "$before" == "$after" ]]; then
        report "lockfile content stable across reruns" "PASS"
    else
        report "lockfile content stable across reruns" "FAIL"
    fi
}

# ---------------------------------------------------------------------------
# Test 4: deleting the lockfile regenerates fresh UUIDs
# ---------------------------------------------------------------------------
test_lockfile_regen() {
    local pipeline="$WORK_DIR/sc_base_filter.yaml"
    local lockfile="$WORK_DIR/sc_base_filter.lock.yaml"
    local log="$LOGS_DIR/04_lockfile_regen.log"

    [[ -f "$lockfile" ]] || "$SPADE" check "$pipeline" > "$log" 2>&1
    local before
    before=$(sha256sum "$lockfile" | awk '{print $1}')

    rm -f "$lockfile"
    "$SPADE" check "$pipeline" >> "$log" 2>&1

    local after
    [[ -f "$lockfile" ]] && after=$(sha256sum "$lockfile" | awk '{print $1}') || after=""

    if [[ -n "$after" ]] && [[ "$before" != "$after" ]]; then
        report "deleting lockfile regenerates fresh UUIDs" "PASS"
    else
        report "deleting lockfile regenerates fresh UUIDs" "FAIL"
    fi
}

# ---------------------------------------------------------------------------
# Test 5: corrupt lockfile yields exit 1 plus a helpful hint to delete it
# ---------------------------------------------------------------------------
test_corrupt_lockfile() {
    local pipeline="$WORK_DIR/sc_base_filter.yaml"
    local lockfile="$WORK_DIR/sc_base_filter.lock.yaml"
    local log="$LOGS_DIR/05_corrupt_lockfile.log"

    cat > "$lockfile" <<'EOF'
pipeline: sc_base_filter
version: 1
bindings:
  "@read": not-a-valid-uuid
EOF

    local exit_code=0
    "$SPADE" check "$pipeline" > "$log" 2>&1 || exit_code=$?

    if [[ $exit_code -ne 0 ]] \
        && grep -qi "invalid lockfile" "$log" \
        && grep -qi "delete" "$log"; then
        report "corrupt lockfile yields exit 1 with delete hint" "PASS"
    else
        report "corrupt lockfile yields exit 1 with delete hint" "FAIL"
    fi
    # Restore a clean lockfile for later tests
    rm -f "$lockfile"
    "$SPADE" check "$pipeline" >> "$log" 2>&1 || true
}

# ---------------------------------------------------------------------------
# Test 6: pipeline-level `id` cannot be a short code (spec section 6.1)
# ---------------------------------------------------------------------------
test_top_level_short_code_rejected() {
    local bad="$WORK_DIR/sc_bad_toplevel.yaml"
    local log="$LOGS_DIR/06_top_level_short_code.log"

    cat > "$bad" <<EOF
id: "@pipeline_id"
name: sc_bad_toplevel
version: "1.0"
blocks:
  - id: "@read"
    name: data.read
    inputs: []
    args:
      uri: $FIXTURES_DIR/test_data.csv
      format: CSV
EOF

    local exit_code=0
    "$SPADE" check "$bad" > "$log" 2>&1 || exit_code=$?
    if [[ $exit_code -ne 0 ]] && grep -qi "pipeline-level" "$log"; then
        report "pipeline-level short code rejected" "PASS"
    else
        report "pipeline-level short code rejected" "FAIL"
    fi
    rm -f "$bad" "$WORK_DIR/sc_bad_toplevel.lock.yaml"
}

# ---------------------------------------------------------------------------
# Test 7: duplicate short codes surface as duplicate-id validation errors
# ---------------------------------------------------------------------------
test_duplicate_short_codes() {
    local bad="$WORK_DIR/sc_bad_duplicate.yaml"
    local log="$LOGS_DIR/07_duplicate_short_codes.log"

    cat > "$bad" <<EOF
name: sc_bad_duplicate
version: "1.0"
blocks:
  - id: "@blk"
    name: data.read
    inputs: []
    args:
      uri: $FIXTURES_DIR/test_data.csv
      format: CSV
  - id: "@blk"
    name: data.read
    inputs: []
    args:
      uri: $FIXTURES_DIR/test_data2.csv
      format: CSV
EOF

    local exit_code=0
    "$SPADE" check "$bad" > "$log" 2>&1 || exit_code=$?
    if [[ $exit_code -ne 0 ]] && grep -qi "duplicate block invocation id" "$log"; then
        report "duplicate short codes surfaced as duplicate-id error" "PASS"
    else
        report "duplicate short codes surfaced as duplicate-id error" "FAIL"
    fi
    rm -f "$bad" "$WORK_DIR/sc_bad_duplicate.lock.yaml"
}

# ---------------------------------------------------------------------------
# Test 8: spade run executes a linear short-code pipeline end-to-end
# ---------------------------------------------------------------------------
test_run_linear() {
    local pipeline="$WORK_DIR/sc_base_filter.yaml"
    local lockfile="$WORK_DIR/sc_base_filter.lock.yaml"
    local log="$LOGS_DIR/08_run_linear.log"

    local lock_before=""
    [[ -f "$lockfile" ]] && lock_before=$(sha256sum "$lockfile" | awk '{print $1}')

    if "$SPADE" run --no-ui --keep-work-dir "$pipeline" > "$log" 2>&1; then
        local lock_after=""
        [[ -f "$lockfile" ]] && lock_after=$(sha256sum "$lockfile" | awk '{print $1}')
        # Spade run should not rewrite an already-resolved lockfile.
        if [[ -z "$lock_before" ]] || [[ "$lock_before" == "$lock_after" ]]; then
            report "spade run executes a linear short-code pipeline" "PASS"
        else
            report "spade run executes a linear short-code pipeline (lockfile drift)" "FAIL"
        fi
    else
        report "spade run executes a linear short-code pipeline" "FAIL"
    fi
}

# ---------------------------------------------------------------------------
# Test 9: parallel/diamond DAG with short codes
# ---------------------------------------------------------------------------
test_run_parallel() {
    local pipeline="$WORK_DIR/sc_gdal_terrain.yaml"
    local log="$LOGS_DIR/09_run_parallel.log"

    if "$SPADE" run --no-ui --keep-work-dir "$pipeline" > "$log" 2>&1; then
        report "spade run executes parallel short-code DAG" "PASS"
    else
        report "spade run executes parallel short-code DAG" "FAIL"
    fi
}

# ---------------------------------------------------------------------------
# Test 10: explicit input references with short codes
# ---------------------------------------------------------------------------
test_run_explicit_refs() {
    local pipeline="$WORK_DIR/sc_gdal_color_relief.yaml"
    local log="$LOGS_DIR/10_run_explicit_refs.log"

    if "$SPADE" run --no-ui --keep-work-dir "$pipeline" > "$log" 2>&1; then
        report "spade run handles explicit refs with short codes" "PASS"
    else
        report "spade run handles explicit refs with short codes" "FAIL"
    fi
}

# ---------------------------------------------------------------------------
# Test 11: mixed UUID + short-code pipeline
# ---------------------------------------------------------------------------
test_run_mixed_format() {
    local pipeline="$WORK_DIR/sc_mixed_format.yaml"
    local log="$LOGS_DIR/11_run_mixed_format.log"

    if "$SPADE" run --no-ui --keep-work-dir "$pipeline" > "$log" 2>&1; then
        report "spade run handles mixed UUID + short-code pipeline" "PASS"
    else
        report "spade run handles mixed UUID + short-code pipeline" "FAIL"
    fi
}

# ---------------------------------------------------------------------------
# Run all tests
# ---------------------------------------------------------------------------
echo "=== Short-code feature tests ==="
test_check_creates_lockfile
test_lockfile_covers_all_codes
test_lockfile_stable
test_lockfile_regen
test_corrupt_lockfile
test_top_level_short_code_rejected
test_duplicate_short_codes

if [[ "$MODE" != "check" ]]; then
    test_run_linear
    test_run_parallel
    test_run_explicit_refs
    test_run_mixed_format
fi

echo ""
echo "========================================="
echo "  SHORT-CODE TEST RESULTS"
echo "========================================="
echo "  Passed:  $PASS"
echo "  Failed:  $FAIL"
echo "  Total:   $((PASS + FAIL))"
echo "========================================="

if [[ ${#FAILED_TESTS[@]} -gt 0 ]]; then
    echo ""
    echo "  Failed tests (see $LOGS_DIR/*.log):"
    for t in "${FAILED_TESTS[@]}"; do
        echo "    - $t"
    done
fi

exit "$FAIL"
