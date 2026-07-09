// Resolution of friendly inputs (state, year, group_by) into low-level
// /fullreport parameters. The pure helpers (stateToFips, pickWc, pickGroupBy)
// are unit-tested without network; resolveWc wraps them with a cached fetch of
// the live `wc` dictionary.

import { fetchParameters } from "./client.ts";

// A row of the /fullreport/parameters/wc dictionary.
export interface WcEntry {
  STATE: string;
  STATECD: number;
  EVALID: string;
  EVAL_GRP: number; // the wc code, e.g. 232024 (Maine 2024)
  REPORT_YEAR_NM: string;
  MOST_RECENT: string; // "Y" | "N"
}

// USPS two-letter code -> state/territory FIPS. The `wc` dictionary carries full
// state names and FIPS but not two-letter codes, so this map is static. Covers
// the same set as data.fia (50 states + DC + territories).
export const STATE_FIPS: Record<string, number> = {
  AL: 1, AK: 2, AZ: 4, AR: 5, CA: 6, CO: 8, CT: 9, DE: 10, FL: 12, GA: 13,
  HI: 15, ID: 16, IL: 17, IN: 18, IA: 19, KS: 20, KY: 21, LA: 22, ME: 23,
  MD: 24, MA: 25, MI: 26, MN: 27, MS: 28, MO: 29, MT: 30, NE: 31, NV: 32,
  NH: 33, NJ: 34, NM: 35, NY: 36, NC: 37, ND: 38, OH: 39, OK: 40, OR: 41,
  PA: 42, RI: 44, SC: 45, SD: 46, TN: 47, TX: 48, UT: 49, VT: 50, VA: 51,
  WA: 53, WV: 54, WI: 55, WY: 56, DC: 11, PR: 72, VI: 78, GU: 66, AS: 60,
  MP: 69,
};

// group_by friendly key -> rselected LABEL_VAR string.
const GROUP_BY_LABELS: Record<string, string> = {
  county: "County code and name",
  state: "State code",
  ownership: "Ownership group - Major",
  forest_type: "Forest type group",
};

// Accept a two-letter code (case-insensitive) or a bare numeric FIPS.
export function stateToFips(state: string): number {
  const s = String(state ?? "").trim();
  if (s === "") throw new Error("'state' is required");
  if (/^\d+$/.test(s)) return Number(s);
  const fips = STATE_FIPS[s.toUpperCase()];
  if (fips === undefined) {
    throw new Error(`Unknown state '${state}' (expected a two-letter code or FIPS)`);
  }
  return fips;
}

// Choose the evaluation-group code for a state. Blank year -> the MOST_RECENT
// entry; an explicit year must match an existing EVAL_GRP (fips*10000 + year).
export function pickWc(
  entries: WcEntry[],
  statecd: number,
  year?: string,
): { wc: string; year: number } {
  const forState = entries.filter((e) => e.STATECD === statecd);
  if (forState.length === 0) {
    throw new Error(`No FIA evaluations found for state FIPS ${statecd}`);
  }
  const y = String(year ?? "").trim();
  if (y !== "") {
    const target = Number(`${statecd}${y}`);
    const match = forState.find((e) => e.EVAL_GRP === target);
    if (!match) {
      const years = [...new Set(forState.map((e) => String(e.EVAL_GRP).slice(-4)))].sort();
      throw new Error(
        `No ${y} evaluation for state FIPS ${statecd}. Available years: ${years.join(", ")}`,
      );
    }
    return { wc: String(match.EVAL_GRP), year: Number(y) };
  }
  const recent =
    forState.find((e) => e.MOST_RECENT === "Y") ??
    [...forState].sort((a, b) => b.EVAL_GRP - a.EVAL_GRP)[0];
  return { wc: String(recent.EVAL_GRP), year: Number(String(recent.EVAL_GRP).slice(-4)) };
}

// Map a friendly group_by key to a grouping column. Blank/"none" -> undefined
// (no grouping). The header is the friendly key itself (stable, short).
export function pickGroupBy(
  key?: string,
): { header: string; label: string } | undefined {
  const k = String(key ?? "").trim().toLowerCase();
  if (k === "" || k === "none") return undefined;
  const label = GROUP_BY_LABELS[k];
  if (!label) {
    throw new Error(
      `Unknown group_by '${key}'. Options: none, ${Object.keys(GROUP_BY_LABELS).join(", ")}`,
    );
  }
  return { header: k, label };
}

let _wcCache: WcEntry[] | null = null;

async function wcDictionary(): Promise<WcEntry[]> {
  if (!_wcCache) {
    _wcCache = (await fetchParameters("wc")) as unknown as WcEntry[];
  }
  return _wcCache;
}

// Resolve a friendly state + optional year to a wc code, fetching the live `wc`
// dictionary (cached per process).
export async function resolveWc(
  state: string,
  year?: string,
): Promise<{ wc: string; year: number; statecd: number }> {
  const statecd = stateToFips(state);
  const entries = await wcDictionary();
  const { wc, year: resolvedYear } = pickWc(entries, statecd, year);
  return { wc, year: resolvedYear, statecd };
}
