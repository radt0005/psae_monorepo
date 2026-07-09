// Block: fiadb.fullreport
//
// Queries the FIADB-API /fullreport endpoint for a design-based population
// estimate and its sampling error, and writes the grouped estimate cells as a
// tidy CSV. Warnings and provenance from the response metadata are written to
// the logs, not the CSV.

import { spadeBlock, File } from "spade";
import { writeFileSync } from "node:fs";
import { fetchFullReport, type EstimateRow, type FullReportParams } from "./client.ts";
import { toCsv, type GroupCol } from "./csv.ts";

interface Args {
  wc: string;
  snum: string;
  sdenom?: string;
  rselected?: string;
  cselected?: string;
  pselected?: string;
  rtime?: string;
  ctime?: string;
  ptime?: string;
  strFilter?: string;
  FIAorRPA?: string;
  estOnly?: string;
}

// Determine which grouping dimensions are active and how their columns map to
// the GRP1/GRP2/GRP3 fields on each estimate row.
function activeGroups(args: Args): GroupCol[] {
  const groups: GroupCol[] = [];
  const dims: Array<[keyof EstimateRow, string | undefined]> = [
    ["GRP1", args.rselected],
    ["GRP2", args.cselected],
    ["GRP3", args.pselected],
  ];
  for (const [field, label] of dims) {
    if (label && label.trim() !== "" && label.trim().toLowerCase() !== "none") {
      groups.push({ field, header: label.trim() });
    }
  }
  return groups;
}

export const handler = spadeBlock({
  inputs: {
    wc: "string",
    snum: "string",
    sdenom: "string",
    rselected: "string",
    cselected: "string",
    pselected: "string",
    rtime: "string",
    ctime: "string",
    ptime: "string",
    strFilter: "string",
    FIAorRPA: "string",
    estOnly: "string",
  },
  output: File,
  description:
    "Fetch a design-based FIA population estimate and sampling error from the FIADB-API /fullreport endpoint as a tidy CSV.",
})(async (args: Args): Promise<File> => {
  if (!args.wc || String(args.wc).trim() === "") {
    throw new Error("'wc' (evaluation group code) is required");
  }
  if (!args.snum || String(args.snum).trim() === "") {
    throw new Error("'snum' (numerator estimate attribute) is required");
  }

  const params: FullReportParams = {
    wc: String(args.wc),
    snum: String(args.snum),
    sdenom: args.sdenom,
    rselected: args.rselected,
    cselected: args.cselected,
    pselected: args.pselected,
    rtime: args.rtime,
    ctime: args.ctime,
    ptime: args.ptime,
    strFilter: args.strFilter,
    FIAorRPA: args.FIAorRPA,
    estOnly: args.estOnly,
  };

  const report = await fetchFullReport(params);

  // Provenance and warnings -> logs (captured by the runtime), not the CSV.
  const meta = report.metadata;
  if (meta.numEstDesc) console.log(`Attribute: ${meta.numEstDesc}`);
  if (meta.evalGrps && meta.evalGrps.length) {
    console.log(`Evaluations: ${meta.evalGrps.join(", ")}`);
  }
  if (typeof meta.numEvalPlotCount === "number") {
    console.log(`Evaluation plot count: ${meta.numEvalPlotCount}`);
  }
  if (meta.grmRatioWarning) {
    console.error("WARNING: GRM ratio warning flagged by FIADB-API for this query.");
  }
  if (Array.isArray(meta.warnings) && meta.warnings.length) {
    for (const w of meta.warnings) console.error(`WARNING: ${w}`);
  }

  const groups = activeGroups(args);
  // `estimates` holds the fully-crossed grouping cells; when no grouping is
  // requested it contains the single grand-total row.
  const rows = report.estimates ?? [];
  const csv = toCsv(rows, groups);

  const outPath = "estimates.csv";
  writeFileSync(outPath, csv);
  console.log(`Wrote ${rows.length} estimate row(s) to ${outPath}`);
  return new File(outPath);
});
