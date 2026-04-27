#!/usr/bin/env bash
#
# run_tests.sh — Run spade integration test pipelines
#
# Usage:
#   ./run_tests.sh              Run only local (non-network) pipelines
#   ./run_tests.sh --all        Run all pipelines including network-dependent ones
#   ./run_tests.sh --network    Run only network-dependent pipelines
#   ./run_tests.sh --check      Only validate pipelines (spade check), don't run
#   ./run_tests.sh --generate   Only generate fixtures and pipelines, don't run
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PIPELINES_DIR="$SCRIPT_DIR/pipelines"
OUTPUT_DIR="$SCRIPT_DIR/output"
LOGS_DIR="$SCRIPT_DIR/logs"
FIXTURES_DIR="$SCRIPT_DIR/fixtures"

# Resolve spade binary — prefer the repo-local build, fall back to PATH
if [[ -x "$SCRIPT_DIR/../cli/spade" ]]; then
    SPADE="$SCRIPT_DIR/../cli/spade"
elif command -v spade &>/dev/null; then
    SPADE="$(command -v spade)"
else
    echo "ERROR: spade binary not found. Build it with: cd ../cli && go build -o spade ."
    exit 1
fi

# ---------------------------------------------------------------------------
# Parse flags
# ---------------------------------------------------------------------------
MODE="local"        # local | all | network | check | generate
for arg in "$@"; do
    case "$arg" in
        --all)      MODE="all" ;;
        --network)  MODE="network" ;;
        --check)    MODE="check" ;;
        --generate) MODE="generate" ;;
        -h|--help)
            echo "Usage: $0 [--all|--network|--check|--generate]"
            echo ""
            echo "  (default)    Run local (non-network) pipelines"
            echo "  --all        Run all pipelines including network ones"
            echo "  --network    Run only network-dependent pipelines"
            echo "  --check      Validate pipelines with 'spade check' only"
            echo "  --generate   Generate fixtures and pipelines only"
            exit 0
            ;;
        *)
            echo "Unknown flag: $arg" >&2
            exit 1
            ;;
    esac
done

# ---------------------------------------------------------------------------
# Step 1: Generate fixtures and pipelines
# ---------------------------------------------------------------------------
echo "=== Step 1: Generating fixtures and pipelines ==="
if [[ "$MODE" == "generate" ]]; then
    (cd "$SCRIPT_DIR" && uv run python generate.py)
    echo "Done (generate only)."
    exit 0
fi

# ---------------------------------------------------------------------------
# Step 2: Prepare output and log directories
# ---------------------------------------------------------------------------
mkdir -p "$OUTPUT_DIR" "$LOGS_DIR"
# The isolate sandbox runs as a remapped uid (typically 100000), so any
# directory that block args reference must be world-writable for
# data.write / data.write_collection pipelines to succeed.
chmod 0777 "$OUTPUT_DIR" "$FIXTURES_DIR" 2>/dev/null || true

# ---------------------------------------------------------------------------
# Step 3: Classify pipelines
# ---------------------------------------------------------------------------
mapfile -t ALL_PIPELINES < <(find "$PIPELINES_DIR" -name '*.yaml' -type f | sort)

LOCAL_PIPELINES=()
NETWORK_PIPELINES=()
for p in "${ALL_PIPELINES[@]}"; do
    if grep -q '\[NETWORK\]' "$p" 2>/dev/null; then
        NETWORK_PIPELINES+=("$p")
    else
        LOCAL_PIPELINES+=("$p")
    fi
done

# Select which pipelines to run based on mode
PIPELINES_TO_RUN=()
case "$MODE" in
    local)   PIPELINES_TO_RUN=("${LOCAL_PIPELINES[@]}") ;;
    network) PIPELINES_TO_RUN=("${NETWORK_PIPELINES[@]}") ;;
    all|check) PIPELINES_TO_RUN=("${ALL_PIPELINES[@]}") ;;
esac

echo ""
echo "=== Pipeline summary ==="
echo "  Local pipelines:   ${#LOCAL_PIPELINES[@]}"
echo "  Network pipelines: ${#NETWORK_PIPELINES[@]}"
echo "  Selected to run:   ${#PIPELINES_TO_RUN[@]}  (mode=$MODE)"
echo ""

# ---------------------------------------------------------------------------
# Step 4: Validate / Run
# ---------------------------------------------------------------------------
PASS=0
FAIL=0
SKIP=0
ERRORS=()

for pipeline_file in "${PIPELINES_TO_RUN[@]}"; do
    name="$(basename "$pipeline_file" .yaml)"
    log_file="$LOGS_DIR/${name}.log"

    if [[ "$MODE" == "check" ]]; then
        # Validate only
        printf "  CHECK  %-50s " "$name"
        if "$SPADE" check "$pipeline_file" > "$log_file" 2>&1; then
            echo "OK"
            PASS=$((PASS + 1))
        else
            echo "FAIL"
            FAIL=$((FAIL + 1))
            ERRORS+=("$name")
        fi
    else
        # Full run
        printf "  RUN    %-50s " "$name"
        pipeline_output_dir="$OUTPUT_DIR/$name"
        mkdir -p "$pipeline_output_dir"

        if "$SPADE" run "$pipeline_file" \
            --no-ui \
            --keep-work-dir \
            > "$log_file" 2>&1; then
            echo "OK"
            PASS=$((PASS + 1))
        else
            exit_code=$?
            echo "FAIL (exit $exit_code)"
            FAIL=$((FAIL + 1))
            ERRORS+=("$name")
        fi
    fi
done

# ---------------------------------------------------------------------------
# Step 5: Summary
# ---------------------------------------------------------------------------
echo ""
echo "========================================="
echo "  RESULTS"
echo "========================================="
echo "  Passed:  $PASS"
echo "  Failed:  $FAIL"
echo "  Total:   $((PASS + FAIL))"
echo "========================================="

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo ""
    echo "  Failed pipelines:"
    for e in "${ERRORS[@]}"; do
        echo "    - $e  (see logs/${e}.log)"
    done
fi

echo ""
echo "Errors: $FAIL"

exit "$FAIL"
