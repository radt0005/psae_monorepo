#!/usr/bin/env bash
#
# Local block seeder — stand-in for the cloud Plugin Registry during local
# development.  See seed/Dockerfile for the rationale.  Idempotent: both the
# block index (RegisterBlock upserts) and the Postgres mirror (ON CONFLICT)
# tolerate re-runs, so it is safe to re-invoke on every `docker compose up`.
#
# Required environment:
#   SPADE_DIR     install root on the shared worker volume (e.g. /var/spade)
#   DATABASE_URL  Postgres DSN for the web UI database (blocks table)

set -euo pipefail

: "${SPADE_DIR:=/var/spade}"
: "${DATABASE_URL:?DATABASE_URL is required}"
export SPADE_DIR

# Rust collections ship as self-contained binaries; install straight from the
# image (pre-compiled at build time, so this is a fast register-only step).
RUST_COLLECTIONS=(base data)

# Python collections build a venv with absolute paths and an editable `spade`
# lib.  Both must resolve inside the worker, which shares only $SPADE_DIR — so
# the source tree, the uv-managed Python, and the uv cache all live there.
PY_COLLECTIONS=(gdal)
# Pin the interpreter: the gdal collection installs `gdal==3.10` from the
# girder large_image_wheels, which ship binary wheels only up to cp313.  Left
# to its own devices uv grabs the newest `>=3.12` (3.14), for which no wheel
# exists and the `--no-build` pin then fails.
export UV_PYTHON=3.12
export UV_PYTHON_INSTALL_DIR="$SPADE_DIR/uv/python"
export UV_CACHE_DIR="$SPADE_DIR/uv/cache"
PY_SRC="$SPADE_DIR/src"   # holds blocks/<c> + libs/python on the volume

# R collections install their package library (spade + jsonlite + yaml) via
# setup.R.  The worker's isolate sandbox derives R_LIBS_USER from $HOME
# (core/executor.go, languageSandboxBinds: it uses $HOME/R when no versioned
# arch-library exists), so the library must live on the shared volume under a
# $HOME the worker also uses.  We install into $SPADE_DIR/rhome/R here and the
# worker service sets HOME=$SPADE_DIR/rhome in docker-compose.yml so its
# executor falls back to that exact path.
R_COLLECTIONS=(sae stats)
R_HOME_DIR="$SPADE_DIR/rhome"

mkdir -p "$SPADE_DIR" "$UV_PYTHON_INSTALL_DIR" "$UV_CACHE_DIR"

echo "==> Installing Rust collections into worker volume ($SPADE_DIR)"
for c in "${RUST_COLLECTIONS[@]}"; do
  echo "--- spade install $c (rust)"
  spade install "/seed/blocks/$c"
done

echo "==> Installing Python collections into worker volume ($SPADE_DIR)"
# Stage the source on the volume preserving the `../../libs/python` relation
# so the editable spade path the install bakes in resolves in the worker.
mkdir -p "$PY_SRC/blocks" "$PY_SRC/libs"
cp -a /seed/libs/python "$PY_SRC/libs/python"
for c in "${PY_COLLECTIONS[@]}"; do
  echo "--- spade install $c (python; first run downloads wheels)"
  cp -a "/seed/blocks/$c" "$PY_SRC/blocks/$c"
  spade install "$PY_SRC/blocks/$c"
done

echo "==> Installing R collections into worker volume ($SPADE_DIR)"
# setup.R reads R_LIBS_USER and installs there; HOME must match so the worker's
# executor resolves the same library path.  First run compiles spade + deps from
# source (a few minutes); the volume persists so re-runs skip the compiled deps.
export HOME="$R_HOME_DIR"
export R_LIBS_USER="$R_HOME_DIR/R"
mkdir -p "$R_LIBS_USER"
for c in "${R_COLLECTIONS[@]}"; do
  echo "--- spade install $c (r; first run compiles spade + deps)"
  spade install "/seed/blocks/$c"
done

echo "==> Mirroring block manifests into Postgres (blocks table)"
for c in "${RUST_COLLECTIONS[@]}" "${PY_COLLECTIONS[@]}" "${R_COLLECTIONS[@]}"; do
  for f in "/seed/blocks/$c/blocks/"*.yaml; do
    [ -e "$f" ] || continue
    id=$(yq '.id' "$f")
    version=$(yq '.version' "$f")
    manifest=$(yq -o=json '.' "$f")
    echo "    upsert $id ($version)"
    # psql's :'var' interpolation safely quotes each value as a string
    # literal, so the JSON manifest is escaped correctly before the ::jsonb
    # cast parses it.  Columns mirror web_ui/server/db/schema/blocks.ts.
    psql "$DATABASE_URL" \
      --set=ON_ERROR_STOP=1 \
      --set=id="$id" \
      --set=version="$version" \
      --set=manifest="$manifest" <<'SQL'
INSERT INTO blocks (id, name, label, version, manifest, created_at, updated_at)
VALUES (:'id', :'id', :'id', :'version', :'manifest'::jsonb, now(), now())
ON CONFLICT (name) DO UPDATE
  SET label      = EXCLUDED.label,
      version    = EXCLUDED.version,
      manifest   = EXCLUDED.manifest,
      updated_at = now();
SQL
  done
done

total=$(( ${#RUST_COLLECTIONS[@]} + ${#PY_COLLECTIONS[@]} + ${#R_COLLECTIONS[@]} ))
echo "==> Seed complete: $total collection(s) installed and mirrored."
