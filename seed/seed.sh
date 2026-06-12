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

# Collections to seed.  Both Rust for now; GDAL (Python) + sae (R) are a
# later pass once the toolchains are added to this image.
COLLECTIONS=(base data)

mkdir -p "$SPADE_DIR"

echo "==> Installing block collections into worker volume ($SPADE_DIR)"
for c in "${COLLECTIONS[@]}"; do
  echo "--- spade install $c"
  # SPADE_DIR drives both the install dir ($SPADE_DIR/blocks/...) and the
  # registry path ($SPADE_DIR/registry.db); the worker reads the same paths
  # off the shared volume (see docker-compose.yml: SPADE_REGISTRY).
  spade install "/seed/blocks/$c"
done

echo "==> Mirroring block manifests into Postgres (blocks table)"
for c in "${COLLECTIONS[@]}"; do
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

echo "==> Seed complete: ${#COLLECTIONS[@]} collection(s) installed and mirrored."
