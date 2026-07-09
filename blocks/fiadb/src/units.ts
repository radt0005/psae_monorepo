// Unit specifications and conversion for FIADB estimates.
//
// FIADB returns imperial units (acres, cubic feet, short tons). The `units: si`
// option converts them. Because each estimate carries an estimate AND its
// sampling variance, conversion must scale VARIANCE by the square of the linear
// factor (and SE by the factor) — the step reference/01_FIA_estimates_long_format.R
// does by hand. SE_PERCENT and PLOT_COUNT are unit-invariant.

import type { EstimateRow } from "./client.ts";

export interface UnitSpec {
  imperial: string; // display label for the native (imperial) unit
  si: string; // display label for the SI unit
  factor: number; // multiply an imperial estimate by this to get SI
}

export const UNIT_SPECS = {
  area: { imperial: "acres", si: "hectares", factor: 0.404686 },
  volume: { imperial: "cubic feet", si: "cubic meters", factor: 0.0283168 },
  biomass: { imperial: "dry short tons", si: "metric tonnes", factor: 0.907185 },
  carbon: { imperial: "short tons", si: "metric tonnes", factor: 0.907185 },
} satisfies Record<string, UnitSpec>;

// Return a copy of the row with the estimate and its dispersion scaled by
// `factor`. VARIANCE scales by factor^2; SE by factor; SE_PERCENT and
// PLOT_COUNT (and any GRP labels) are left unchanged.
export function convertRow(row: EstimateRow, factor: number): EstimateRow {
  return {
    ...row,
    ESTIMATE: row.ESTIMATE * factor,
    SE: row.SE * factor,
    VARIANCE: row.VARIANCE * factor * factor,
  };
}
