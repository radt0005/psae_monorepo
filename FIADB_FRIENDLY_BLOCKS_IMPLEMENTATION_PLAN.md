# FIADB Friendly Estimate Blocks — Implementation Plan

This plan adds a **friendly, deterministic layer** on top of the existing
low-level `fiadb` collection (`blocks/fiadb/` — `fullreport` + `parameters`). The
goal is that a user can ask for a common FIA population estimate **without knowing
FIA internals** — no attribute numbers (`snum`), no evaluation-group codes
(`wc`), no `LABEL_VAR` grouping strings — while the block encodes the domain
knowledge that keeps the result statistically correct.

Four per-attribute blocks — **`area`, `volume`, `biomass`, `carbon`** — take a
plain surface (`state`, `year?`, `group_by`, `land_basis`, `units`) and resolve
it to a correct `/fullreport` query, reusing the existing `client.ts`. They are
thin wrappers over one shared `runEstimate()`.

```
state ("ME") ─► resolve.ts ─► wc (latest EVAL_GRP for STATECD, MOST_RECENT="Y")
attribute block ─► curated snum + land_basis + unit spec
group_by ("county") ─► LABEL_VAR ("County code and name")
        └─► runEstimate() ─► client.fetchFullReport() ─► unit-convert ─► outputs/estimates/*.csv
                                                                          (resolved wc + warnings ─► logs)
```

The low-level `fullreport` / `parameters` blocks stay as the **power-user
escape hatch** and the discovery surface underneath.

---

## 0. Decisions (settled in discussion)

- **Friendly blocks first; LLM deferred.** A natural-language layer, if ever
  built, is an **authoring-time planner** that emits friendly-block pipelines for
  review — never a runtime block. Rationale: an LLM in the execution path breaks
  the system's cache-key determinism, drags in the [secrets design](SECRETS_IMPLEMENTATION_PLAN.md),
  and carries the highest **silent-wrong-answer** risk. Encoding FIA expertise
  deterministically is both safer and cacheable.
- **Per-attribute blocks**, not one enum block. Each reads explicitly in a
  pipeline and can carry attribute-specific options; duplication is avoided by a
  shared `runEstimate()`.
- **Auto-resolve latest evaluation, override allowed.** Blank `year` ⇒ the
  state's `MOST_RECENT="Y"` evaluation; an explicit `year` pins it. The resolved
  `wc`/year is **logged** so an auto-resolved run is self-documenting.
- **Unit conversion with variance scaling.** `units: imperial|si`. Convert
  `ESTIMATE` and `SE` by the factor, `VARIANCE` by factor², leave `SE_PERCENT`
  and `PLOT_COUNT` unchanged. This owns the fiddly step that
  `reference/01_FIA_estimates_long_format.R` does by hand.

### Key simplification confirmed against the live API

`wc` is **just state + year**; the endpoint resolves the specific evaluation from
the attribute's eval type server-side (a `wc=102020` query for `snum=2` ran
against `evalid=102001`/EXPCURR). So the friendly blocks never reason about eval
types when building `wc` — they only pick the state's latest evaluation group and
inherit the server's clean "no such evaluation" error rather than a silent wrong
answer. The `parameters/wc?outputFormat=JSON` dictionary carries everything
needed: `STATECD`, `EVAL_GRP` (the `wc` code), `MOST_RECENT`, `REPORT_YEAR_NM`.

---

## 1. Open decisions to settle during implementation

Each has a recommendation.

- **`land_basis` as a shared input.** Every attribute has a clean forest/
  timberland `snum` pair (see §4 table), so `land_basis: forest|timberland`
  (default `forest`) is a **common input on all four blocks**, not an
  attribute-specific one. **Recommendation: adopt it as common.**

- **Attribute-specific variant coverage for v1.** `volume` has net/gross/sound
  and bole-wood vs bole-bark forms; `biomass` has dry vs green short tons; `carbon`
  has aboveground vs aboveground+belowground. **Recommendation:** v1 exposes one
  sensible default per block and defers the rest to a `variant`/`measure` enum
  added later (the curated table is structured to make this a data-only change).
  Defaults: volume = *net merchantable bole wood, live*; biomass = *aboveground,
  live, dry short tons*; carbon = *aboveground, live*.

- **`group_by` enum coverage.** **Recommendation:** v1 ships
  `none, county, state, ownership, forest_type`, mapping to the LABEL_VAR strings
  `County code and name`, `State code`, `Ownership group - Major`,
  `Forest type group`. Growable via the curated map.

- **SE: scale vs recompute.** The API already returns `SE`, `SE_PERCENT`, and
  `VARIANCE`. **Recommendation:** convert (`SE × factor`, `VARIANCE × factor²`)
  rather than recompute from scratch — cheaper and avoids reintroducing rounding.
  `SE_PERCENT` is unit-invariant and passes through unchanged (a good self-check:
  `SE/ESTIMATE` must be identical before and after conversion).

- **FIPS coverage.** The static `state → STATECD` map must include DC (11) and
  territories (PR 72, VI 78, etc.) to match `data.fia`'s accepted set. **Validate
  the two-letter set against `blocks/data/blocks/fia.yaml`.** Alternatively derive
  the map at runtime from the distinct `STATE`/`STATECD` pairs in the `wc`
  dictionary — **recommended**, since it can't drift from what the API accepts.

---

## 2. Order of Work

Local-testable throughout: each block is runnable via `spade run` against the
live API as soon as it exists.

| Phase | Deliverable | Value |
| --- | --- | --- |
| 1 | Shared `resolve.ts` + `estimate.ts` core | resolution + fetch reusable |
| 2 | Four per-attribute blocks + manifests + curated table | friendly blocks run |
| 3 | Unit conversion (imperial→SI, variance scaling) | SI outputs, no hand math |
| 4 | Integration pipelines in `test/generate.py` | covered end-to-end |
| 5 | Docs (`README.md`, collection reference) | discoverable |

---

## 3. Phase 1 — Shared resolution and estimate core

New files in `blocks/fiadb/src/`:

**`resolve.ts`** — pure resolution, deterministic given the dictionaries:
- `resolveState(state) → STATECD` — build the map from the distinct
  `STATE`/`STATECD` pairs in the `wc` dictionary (fetched via
  `client.fetchParameters("wc")`), accepting a two-letter code or a bare FIPS.
- `resolveWc(STATECD, year?) → wc` — filter the `wc` entries to `STATECD`; if
  `year` is blank pick the entry with `MOST_RECENT === "Y"`; else match
  `EVAL_GRP === Number(\`${fips}${year}\`)`. Throw a clear error listing available
  years (`REPORT_YEAR_NM`) when a requested year has no evaluation.
- `resolveGroupBy(key) → LABEL_VAR | undefined` — the curated §1 map; `none`/blank
  ⇒ `undefined` (no `rselected`).

**`estimate.ts`** — one shared entry all attribute blocks call:
```ts
interface EstimateSpec {
  snum: string;                 // from the curated table (land_basis-resolved)
  unit: UnitSpec;               // native unit + SI factor (§5)
}
interface EstimateArgs {
  state: string; year?: string; group_by?: string; units?: "imperial" | "si";
}
async function runEstimate(spec: EstimateSpec, args: EstimateArgs): Promise<File>
```
It resolves `wc` + `group_by`, calls `client.fetchFullReport({ wc, snum,
rselected })`, logs provenance + the **resolved `wc`/year**, applies unit
conversion (§5), and writes the tidy CSV — the same column contract as
`fullreport` (`group col?, ESTIMATE, VARIANCE, SE, SE_PERCENT, PLOT_COUNT`).

The existing `fullreport.ts` CSV flattener (`toCsv`, `cleanGroupLabel`,
`csvField`) moves into a shared `src/csv.ts` so both `fullreport` and
`runEstimate` use one implementation.

---

## 4. Phase 2 — The four per-attribute blocks

Each block file (`src/area.ts`, `src/volume.ts`, `src/biomass.ts`,
`src/carbon.ts`) is a thin `spadeBlock` handler: read args → pick `snum` from the
curated table by `land_basis` (+ default variant) → call `runEstimate`. Register
each in `src/index.ts`'s dispatcher.

**Curated attribute table** (`src/attributes.ts`) — grounded in the live `snum`
dictionary; this is where FIA expertise lives:

| Block | `land_basis: forest` | `land_basis: timberland` | Native unit | Eval type |
| --- | --- | --- | --- | --- |
| `area`   | `2`      | `3`      | acres          | EXPCURR |
| `volume` | `574171` | `574172` | cubic feet     | EXPVOL  |
| `biomass`| `10`     | `13`     | dry short tons | EXPVOL  |
| `carbon` | `53000`  | `67000`  | short tons     | EXPVOL  |

Manifest sketch (`blocks/fiadb/blocks/biomass.yaml`):
```yaml
id: fiadb.biomass
version: 0.1.0
kind: standard
network: true
description: >-
  Aboveground biomass of live trees (design-based FIA estimate + sampling error)
  for a state, optionally grouped, in imperial or SI units.
inputs:
  state:      { type: string, description: Two-letter state code (or FIPS). }
  year:       { type: string, description: Inventory year; blank = most recent evaluation. }
  group_by:   { type: string, description: "none | county | state | ownership | forest_type" }
  land_basis: { type: string, description: "forest (default) | timberland" }
  units:      { type: string, description: "imperial (default) | si" }
outputs:
  estimates:
    type: file
    format: CSV
    description: Estimate cells with ESTIMATE, VARIANCE, SE, SE_PERCENT, PLOT_COUNT.
```
`area` / `volume` / `carbon` mirror this; `volume` gains an optional `measure`
input later (§1).

**Test now:** `spade run` a one-block pipeline
(`args: { state: ME, group_by: county }`) → county-level biomass CSV.

---

## 5. Phase 3 — Unit conversion

`src/units.ts` — per-attribute `UnitSpec { imperial: label, si: label, factor }`:

| Attribute | Imperial | SI | factor |
| --- | --- | --- | --- |
| area    | acres          | hectares       | 0.404686  |
| volume  | cubic feet     | cubic meters   | 0.0283168 |
| biomass | dry short tons | metric tonnes  | 0.907185  |
| carbon  | short tons     | metric tonnes  | 0.907185  |

When `units === "si"`, `runEstimate` maps each row: `ESTIMATE *= f`, `SE *= f`,
`VARIANCE *= f*f`; `SE_PERCENT`, `PLOT_COUNT`, and group columns untouched.
`imperial` (default) passes values through verbatim. Assert in a unit test that
`SE/ESTIMATE` is invariant across conversion.

---

## 6. Phase 4 — Integration tests

Extend `test/generate.py`:
- Add `BLOCK_DEFAULTS` for `fiadb.area|volume|biomass|carbon`
  (`{state, year, group_by, land_basis, units}`, all defaulted to `""`).
- Extend `_fiadb_pipelines()` with `[NETWORK]` pipelines:
  - `fiadb_biomass_county` — `biomass`, `state: ME`, `group_by: county`, `units: si`.
  - `fiadb_area_state` — `area`, `state: RI` (no grouping, imperial).
  - `fiadb_carbon_ownership` — `carbon`, `state: DE`, `group_by: ownership`.
  - `fiadb_biomass_summary` — `biomass → stats.summary` (cross-collection, mirrors
    the existing `fiadb_fullreport_summary`).

Validate with `spade check` and run under `run_tests.sh --network`.

Unit tests (`blocks/fiadb`, `bun test`): `resolve.ts` (latest vs explicit year,
bad year error, state map) against a small fixture of `wc` entries; `units.ts`
scaling + `SE_PERCENT` invariant; `csv.ts` shared flattener.

---

## 7. Phase 5 — Docs

- Extend `blocks/fiadb/README.md`: a "Friendly blocks" section with the attribute
  table, the `group_by`/`land_basis`/`units` enums, and a note that `fullreport`
  remains the escape hatch and `parameters` the discovery surface.
- One worked example end-to-end (`biomass`, `state: ME`, `group_by: county`,
  `units: si`) showing the CSV and the logged resolved `wc`.

---

## 8. Cross-cutting & out of scope

**Cross-cutting**
- **Determinism / caching.** With `year` blank, inputs are stable but the API's
  newest evaluation can change over time; the block logs the resolved `wc`/year so
  any run is auditable, and users needing a frozen result set `year`. No LLM in the
  runtime path preserves cache-key determinism.
- **Correctness over convenience.** The curated `snum` + forest/timberland pairing
  is the safety mechanism — it prevents the silent-wrong-answer combinations a raw
  `fullreport` (or an LLM) can produce. Eval-type matching stays server-side.
- **One CSV contract.** Friendly blocks and `fullreport` emit the identical column
  shape, so downstream `stats`/`sae` blocks treat them interchangeably.

**Out of scope (this plan)**
- The LLM authoring-time planner (separate effort; emits friendly-block pipelines).
- Ratio estimates (`sdenom`), change/growth/removal/mortality attributes, and
  circular lat/lon/radius estimates — reachable via `fullreport`.
- Attribute `variant`/`measure` enums beyond the v1 defaults (table is structured
  to add them as data).
- Multi-state / multi-`wc` estimates in one call.
