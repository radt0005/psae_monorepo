// Shared CSV flattening for FIADB estimate rows. Used by both the low-level
// fullreport block and the friendly per-attribute blocks so they emit an
// identical column contract: <group columns...>, ESTIMATE, VARIANCE, SE,
// SE_PERCENT, PLOT_COUNT.

import type { EstimateRow } from "./client.ts";

export const STAT_COLUMNS = [
  "ESTIMATE",
  "VARIANCE",
  "SE",
  "SE_PERCENT",
  "PLOT_COUNT",
] as const;

// A grouping column: which GRP field on the row holds the value, and the header
// to print for it.
export interface GroupCol {
  field: keyof EstimateRow;
  header: string;
}

// Grouping labels come back as "`<sortcode> <display>" (e.g. "`10001 10001 DE
// Kent" or "`0001 Public"). Strip the leading backtick + sort code, keeping the
// display value (which for geographic groups retains the FIPS code used for
// downstream joins).
export function cleanGroupLabel(value: string | undefined): string {
  if (value === undefined || value === null) return "";
  return String(value).replace(/^`\S+\s+/, "");
}

export function csvField(value: string | number | undefined): string {
  if (value === undefined || value === null) return "";
  const s = String(value);
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

export function toCsv(rows: EstimateRow[], groups: GroupCol[]): string {
  const header = [...groups.map((g) => g.header), ...STAT_COLUMNS];
  const lines = [header.map(csvField).join(",")];
  for (const row of rows) {
    const groupCells = groups.map((g) =>
      cleanGroupLabel(row[g.field] as string | undefined),
    );
    const statCells = STAT_COLUMNS.map((c) => row[c] as number);
    lines.push([...groupCells, ...statCells].map(csvField).join(","));
  }
  return lines.join("\n") + "\n";
}
