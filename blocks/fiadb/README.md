# fiadb

A Spade block collection (TypeScript / Bun) wrapping the USFS **FIADB-API /
EVALIDator** service — <https://apps.fs.usda.gov/fiadb-api/> — for retrieving
design-based Forest Inventory and Analysis (FIA) population estimates and their
sampling errors directly from the official estimation engine.

This is the estimate-and-variance counterpart to the raw-microdata `data.fia`
block: instead of downloading plot/tree/condition tables, it returns
server-computed estimates (`ESTIMATE`, `VARIANCE`, `SE`, `SE_PERCENT`,
`PLOT_COUNT`) ready to feed downstream `stats` / `sae` blocks.

## Blocks

There are two layers: **friendly per-attribute blocks** for common estimates
(no FIA codes required), and the **low-level `fullreport` / `parameters`** blocks
as the power-user escape hatch and discovery surface.

## Friendly blocks

`fiadb.area`, `fiadb.volume`, `fiadb.biomass`, `fiadb.carbon` — each returns a
design-based population estimate with its sampling error for a state, without
needing to know attribute numbers (`snum`), evaluation codes (`wc`), or grouping
`LABEL_VAR` strings. They share one input surface:

| Input | Values | Default |
| --- | --- | --- |
| `state` | two-letter code (e.g. `ME`) or numeric FIPS | required |
| `year` | inventory year; blank ⇒ most recent evaluation | blank (latest) |
| `group_by` | `none`, `county`, `state`, `ownership`, `forest_type` | `none` |
| `land_basis` | `forest`, `timberland` | `forest` |
| `units` | `imperial`, `si` | `imperial` |

The curated attribute → `snum` mapping (`src/attributes.ts`) is where the FIA
domain knowledge lives — it keeps the attribute paired with a valid land basis:

| Block | forest `snum` | timberland `snum` | imperial → SI |
| --- | --- | --- | --- |
| `area` | 2 | 3 | acres → hectares |
| `volume` | 574171 | 574172 | ft³ → m³ |
| `biomass` | 10 | 13 | dry short tons → tonnes |
| `carbon` | 53000 | 67000 | short tons → tonnes |

**Auto-resolution.** Blank `year` resolves to the state's most recent evaluation
(via the `wc` dictionary's `MOST_RECENT` flag); the resolved `wc`/year is written
to the logs so a run is self-documenting. Set `year` explicitly to pin a result.

**Units.** `units: si` converts `ESTIMATE` and `SE` by the linear factor and
`VARIANCE` by the factor **squared**; `SE_PERCENT` and `PLOT_COUNT` are
unit-invariant.

**Output** is the same tidy `estimates` CSV as `fullreport` — an optional group
column followed by `ESTIMATE, VARIANCE, SE, SE_PERCENT, PLOT_COUNT` — so friendly
and low-level blocks feed `stats` / `sae` interchangeably.

Example `params.yaml` (aboveground biomass by county for Maine, in tonnes):

```yaml
state: ME
group_by: county
units: si
```

## Low-level blocks

### `fiadb.fullreport`

Queries `/fullreport` and writes the grouped estimate cells as a tidy CSV.

Required inputs:

- `wc` — evaluation group code(s): state FIPS + 4-digit inventory year, e.g.
  `102020` (Delaware 2020). Comma-separate multiple codes.
- `snum` — numerator estimate attribute number, e.g. `2` (area of forest land,
  acres). Discover valid numbers with `fiadb.parameters` (`name: snum`).

Common optional inputs:

- `sdenom` — ratio denominator attribute number (estimate becomes `snum/sdenom`).
- `rselected` / `cselected` / `pselected` — row / column / page grouping. **Pass
  the `LABEL_VAR` string** from the corresponding parameter dictionary (e.g.
  `County code and name`), *not* the bare `DB_VAR`. Each active grouping becomes
  a column in the CSV.
- `rtime` / `ctime` / `ptime` — temporal basis for change/growth/mortality
  groupings.
- `strFilter` — advanced SQL-style filter (submitted via POST when long).
- `FIAorRPA` — forest land definition, `FIADEF` (default) or `RPADEF`.
- `estOnly` — `Y` to skip server-side subtotal/total computation.

Output `estimates` — CSV with one column per active grouping dimension followed
by `ESTIMATE, VARIANCE, SE, SE_PERCENT, PLOT_COUNT`. With no grouping, a single
grand-total row. Response warnings and provenance (attribute description,
evaluations, plot counts, GRM ratio warnings) go to the block **logs**.

Example `params.yaml`:

```yaml
wc: "102020"
snum: "2"
rselected: "County code and name"
```

### `fiadb.parameters`

Fetches a parameter dictionary as a JSON array — the valid values and metadata
for a named `/fullreport` parameter.

- Input `name` — one of `snum`, `wc`, `rselected`, `cselected`, `pselected`, …
- Output `dictionary` — JSON array. For `snum`: `ATTRIBUTE_NBR`,
  `ATTRIBUTE_DESCR`, units, etc. For grouping parameters: `LABEL_VAR`, `DB_VAR`,
  and the underlying SQL.

## API notes

- The service returns `outputFormat=NJSON` (flat/normalized JSON) for
  `/fullreport`; the client requests this internally.
- Errors are returned as an **HTTP 200 HTML error page**, not a JSON error. The
  client detects this and fails with a non-zero exit and the embedded
  `Received an Error: …` message, halting the pipeline.
- No authentication is required.
