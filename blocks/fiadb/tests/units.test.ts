import { describe, expect, test } from "bun:test";
import { convertRow, UNIT_SPECS } from "../src/units.ts";
import type { EstimateRow } from "../src/client.ts";

const row: EstimateRow = {
  ESTIMATE: 1000,
  VARIANCE: 400,
  SE: 20,
  SE_PERCENT: 2,
  PLOT_COUNT: 50,
  GRP1: "`0001 Public",
};

describe("convertRow", () => {
  test("scales ESTIMATE and SE by factor, VARIANCE by factor^2", () => {
    const f = UNIT_SPECS.area.factor;
    const out = convertRow(row, f);
    expect(out.ESTIMATE).toBeCloseTo(1000 * f, 10);
    expect(out.SE).toBeCloseTo(20 * f, 10);
    expect(out.VARIANCE).toBeCloseTo(400 * f * f, 10);
  });

  test("SE_PERCENT, PLOT_COUNT and group labels are invariant", () => {
    const out = convertRow(row, UNIT_SPECS.biomass.factor);
    expect(out.SE_PERCENT).toBe(2);
    expect(out.PLOT_COUNT).toBe(50);
    expect(out.GRP1).toBe("`0001 Public");
  });

  test("SE/ESTIMATE ratio is preserved (relative error unchanged)", () => {
    const out = convertRow(row, UNIT_SPECS.volume.factor);
    expect(out.SE / out.ESTIMATE).toBeCloseTo(row.SE / row.ESTIMATE, 12);
  });

  test("VARIANCE stays consistent with SE^2 after conversion", () => {
    const out = convertRow(row, UNIT_SPECS.carbon.factor);
    // input already satisfies VARIANCE == SE^2 (400 == 20^2)
    expect(out.VARIANCE).toBeCloseTo(out.SE * out.SE, 6);
  });
});
