import { describe, expect, test } from "bun:test";
import { cleanGroupLabel, csvField, toCsv, type GroupCol } from "../src/csv.ts";
import type { EstimateRow } from "../src/client.ts";

describe("cleanGroupLabel", () => {
  test("strips backtick + sort code, keeps display (with FIPS)", () => {
    expect(cleanGroupLabel("`10001 10001 DE Kent")).toBe("10001 DE Kent");
    expect(cleanGroupLabel("`0001 Public")).toBe("Public");
  });
  test("undefined -> empty string", () => {
    expect(cleanGroupLabel(undefined)).toBe("");
  });
});

describe("csvField", () => {
  test("quotes fields containing comma/quote/newline", () => {
    expect(csvField("a,b")).toBe('"a,b"');
    expect(csvField('say "hi"')).toBe('"say ""hi"""');
    expect(csvField("plain")).toBe("plain");
    expect(csvField(42)).toBe("42");
  });
});

describe("toCsv", () => {
  const rows: EstimateRow[] = [
    { ESTIMATE: 1, VARIANCE: 2, SE: 3, SE_PERCENT: 4, PLOT_COUNT: 5, GRP1: "`0001 Public" },
    { ESTIMATE: 6, VARIANCE: 7, SE: 8, SE_PERCENT: 9, PLOT_COUNT: 10, GRP1: "`0002 Private" },
  ];

  test("one grouping column uses friendly header and cleaned labels", () => {
    const groups: GroupCol[] = [{ field: "GRP1", header: "ownership" }];
    const csv = toCsv(rows, groups).trimEnd().split("\n");
    expect(csv[0]).toBe("ownership,ESTIMATE,VARIANCE,SE,SE_PERCENT,PLOT_COUNT");
    expect(csv[1]).toBe("Public,1,2,3,4,5");
    expect(csv[2]).toBe("Private,6,7,8,9,10");
  });

  test("no grouping -> stat columns only", () => {
    const csv = toCsv([rows[0]], []).trimEnd().split("\n");
    expect(csv[0]).toBe("ESTIMATE,VARIANCE,SE,SE_PERCENT,PLOT_COUNT");
    expect(csv[1]).toBe("1,2,3,4,5");
  });
});
