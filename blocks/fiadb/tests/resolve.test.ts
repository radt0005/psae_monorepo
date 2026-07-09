import { describe, expect, test } from "bun:test";
import {
  stateToFips,
  pickWc,
  pickGroupBy,
  type WcEntry,
} from "../src/resolve.ts";

const wc = (STATECD: number, EVAL_GRP: number, MOST_RECENT = "N"): WcEntry => ({
  STATE: "",
  STATECD,
  EVALID: "",
  EVAL_GRP,
  REPORT_YEAR_NM: "",
  MOST_RECENT,
});

const DICT: WcEntry[] = [
  wc(23, 232024, "Y"),
  wc(23, 232023),
  wc(23, 232020),
  wc(10, 102020, "Y"),
];

describe("stateToFips", () => {
  test("two-letter code (case-insensitive)", () => {
    expect(stateToFips("ME")).toBe(23);
    expect(stateToFips("me")).toBe(23);
  });
  test("bare numeric FIPS passes through", () => {
    expect(stateToFips("23")).toBe(23);
  });
  test("unknown code throws", () => {
    expect(() => stateToFips("ZZ")).toThrow("Unknown state");
  });
  test("empty throws", () => {
    expect(() => stateToFips("")).toThrow("required");
  });
});

describe("pickWc", () => {
  test("blank year picks MOST_RECENT", () => {
    expect(pickWc(DICT, 23)).toEqual({ wc: "232024", year: 2024 });
  });
  test("explicit year matches EVAL_GRP", () => {
    expect(pickWc(DICT, 23, "2020")).toEqual({ wc: "232020", year: 2020 });
  });
  test("unknown year throws with available years", () => {
    expect(() => pickWc(DICT, 23, "1850")).toThrow("2020, 2023, 2024");
  });
  test("state with no evaluations throws", () => {
    expect(() => pickWc(DICT, 99)).toThrow("No FIA evaluations");
  });
  test("falls back to max EVAL_GRP when no MOST_RECENT flag", () => {
    const noFlag = [wc(5, 52021), wc(5, 52023), wc(5, 52022)];
    expect(pickWc(noFlag, 5)).toEqual({ wc: "52023", year: 2023 });
  });
});

describe("pickGroupBy", () => {
  test("none / blank -> undefined", () => {
    expect(pickGroupBy("none")).toBeUndefined();
    expect(pickGroupBy("")).toBeUndefined();
    expect(pickGroupBy(undefined)).toBeUndefined();
  });
  test("known key -> LABEL_VAR with friendly header", () => {
    expect(pickGroupBy("county")).toEqual({
      header: "county",
      label: "County code and name",
    });
    expect(pickGroupBy("Ownership")).toEqual({
      header: "ownership",
      label: "Ownership group - Major",
    });
  });
  test("unknown key throws", () => {
    expect(() => pickGroupBy("planet")).toThrow("Unknown group_by");
  });
});
