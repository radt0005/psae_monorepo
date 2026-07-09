// Shared core for the friendly per-attribute blocks (area, volume, biomass,
// carbon). Resolves friendly inputs to a /fullreport query, fetches it, applies
// optional unit conversion, and writes the same tidy CSV contract as the
// low-level fullreport block.

import { File } from "spade";
import { writeFileSync } from "node:fs";
import { fetchFullReport } from "./client.ts";
import { resolveWc, pickGroupBy } from "./resolve.ts";
import { toCsv, type GroupCol } from "./csv.ts";
import { convertRow } from "./units.ts";
import type { EstimateSpec } from "./attributes.ts";

export interface EstimateArgs {
  state: string;
  year?: string;
  group_by?: string;
  units?: string;
}

export async function runEstimate(
  spec: EstimateSpec,
  args: EstimateArgs,
): Promise<File> {
  if (!args.state || String(args.state).trim() === "") {
    throw new Error("'state' is required");
  }

  const { wc, year, statecd } = await resolveWc(args.state, args.year);
  const group = pickGroupBy(args.group_by);

  console.log(
    `Resolved: state FIPS ${statecd}, wc ${wc} (${year}), snum ${spec.snum}` +
      (group ? `, group_by ${group.header}` : ""),
  );

  const report = await fetchFullReport({
    wc,
    snum: spec.snum,
    rselected: group?.label,
  });

  // Provenance and warnings -> logs, not the CSV.
  const meta = report.metadata;
  if (meta.numEstDesc) console.log(`Attribute: ${meta.numEstDesc}`);
  if (meta.evalGrps && meta.evalGrps.length) {
    console.log(`Evaluations: ${meta.evalGrps.join(", ")}`);
  }
  if (meta.grmRatioWarning) {
    console.error("WARNING: GRM ratio warning flagged by FIADB-API for this query.");
  }
  if (Array.isArray(meta.warnings) && meta.warnings.length) {
    for (const w of meta.warnings) console.error(`WARNING: ${w}`);
  }

  // Unit conversion.
  const units = String(args.units ?? "").trim().toLowerCase() || "imperial";
  let rows = report.estimates ?? [];
  let unitLabel = spec.unit.imperial;
  if (units === "si") {
    rows = rows.map((r) => convertRow(r, spec.unit.factor));
    unitLabel = spec.unit.si;
  } else if (units !== "imperial") {
    throw new Error(`Unknown units '${args.units}'. Options: imperial, si`);
  }
  console.log(`Units: ${unitLabel}`);

  const groups: GroupCol[] = group ? [{ field: "GRP1", header: group.header }] : [];
  const csv = toCsv(rows, groups);

  const outPath = "estimates.csv";
  writeFileSync(outPath, csv);
  console.log(`Wrote ${rows.length} estimate row(s) to ${outPath}`);
  return new File(outPath);
}
