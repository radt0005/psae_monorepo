// Curated FIA attribute table for the friendly per-attribute blocks.
//
// This is where the domain knowledge lives: each friendly attribute maps to the
// correct FIADB `snum` for both land bases (forest vs timberland), paired with
// its native unit spec. Encoding these pairings here is the safety mechanism that
// prevents the silent-wrong-answer combinations a raw fullreport query (or an
// LLM) can produce. snum values were verified against the live
// /fullreport/parameters/snum dictionary.

import { UNIT_SPECS, type UnitSpec } from "./units.ts";

export interface AttributeSpec {
  // FIADB numerator attribute number, keyed by land basis.
  snum: { forest: string; timberland: string };
  unit: UnitSpec;
}

// Resolved input to runEstimate: a single snum and its unit spec.
export interface EstimateSpec {
  snum: string;
  unit: UnitSpec;
}

// v1 attribute set. Defaults per attribute (documented in the plan):
//   area   — area of forest/timberland land (EXPCURR)
//   volume — net merchantable bole wood volume of live trees (EXPVOL)
//   biomass— aboveground biomass of live trees >=1", dry short tons (EXPVOL)
//   carbon — aboveground carbon in live trees, short tons (EXPVOL)
export const ATTRIBUTES: Record<string, AttributeSpec> = {
  area: { snum: { forest: "2", timberland: "3" }, unit: UNIT_SPECS.area },
  volume: { snum: { forest: "574171", timberland: "574172" }, unit: UNIT_SPECS.volume },
  biomass: { snum: { forest: "10", timberland: "13" }, unit: UNIT_SPECS.biomass },
  carbon: { snum: { forest: "53000", timberland: "67000" }, unit: UNIT_SPECS.carbon },
};

// Resolve an attribute + land basis to a concrete estimate spec.
export function specFor(name: string, land_basis?: string): EstimateSpec {
  const attr = ATTRIBUTES[name];
  if (!attr) {
    throw new Error(
      `unknown attribute '${name}' (expected one of: ${Object.keys(ATTRIBUTES).join(", ")})`,
    );
  }
  const basis = (land_basis ?? "").trim().toLowerCase() || "forest";
  if (basis !== "forest" && basis !== "timberland") {
    throw new Error(`Unknown land_basis '${land_basis}'. Options: forest, timberland`);
  }
  return {
    snum: basis === "timberland" ? attr.snum.timberland : attr.snum.forest,
    unit: attr.unit,
  };
}
