import { describe, expect, test, beforeEach, afterEach } from "bun:test";
import { writeFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";
import yaml from "js-yaml";
import { loadParams, scanInputs, buildFunctionArgs } from "../src/scanning.ts";
import {
  File,
  Directory,
  FileCollection,
  RasterFile,
  RasterFileCollection,
} from "../src/types.ts";
import { setupWorkDir, createInputFile, createInputCollection, writeParams } from "./helpers.ts";

let workDir: { path: string; originalCwd: string };

beforeEach(() => {
  workDir = setupWorkDir();
});

afterEach(() => {
  process.chdir(workDir.originalCwd);
});

describe("loadParams", () => {
  test("basic params", () => {
    writeParams({ buffer_distance: 30, method: "bilinear" });
    const params = loadParams();
    expect(params).toEqual({ buffer_distance: 30, method: "bilinear" });
  });

  test("empty file", () => {
    writeFileSync("params.yaml", "");
    expect(loadParams()).toEqual({});
  });

  test("missing file", () => {
    expect(loadParams()).toEqual({});
  });
});

describe("scanInputs", () => {
  test("single file with type hint", () => {
    createInputFile("raster", "data.tif");
    const result = scanInputs({ raster: RasterFile });
    expect(result.raster).toBeInstanceOf(RasterFile);
    expect((result.raster as RasterFile).path).toContain("data.tif");
  });

  test("untyped single file defaults to File", () => {
    createInputFile("source", "data.tif");
    const result = scanInputs({});
    expect(result.source).toBeInstanceOf(File);
  });

  test("Directory input", () => {
    const inputDir = join("inputs", "source");
    mkdirSync(inputDir, { recursive: true });
    writeFileSync(join(inputDir, "file1.shp"), "data");
    writeFileSync(join(inputDir, "file2.dbf"), "data");
    const result = scanInputs({ source: Directory });
    expect(result.source).toBeInstanceOf(Directory);
    expect((result.source as Directory).path).toContain("inputs/source");
  });

  test("collection input", () => {
    createInputCollection("tiles", ["001.tif", "002.tif", "003.tif"]);
    const result = scanInputs({ tiles: RasterFileCollection });
    expect(result.tiles).toBeInstanceOf(RasterFileCollection);
    expect((result.tiles as RasterFileCollection).paths).toHaveLength(3);
  });

  test("multiple inputs", () => {
    createInputFile("reference", "ref.tif");
    createInputFile("target", "tgt.tif");
    const result = scanInputs({ reference: RasterFile, target: RasterFile });
    expect(result.reference).toBeDefined();
    expect(result.target).toBeDefined();
  });

  test("empty input directory with type hint throws", () => {
    mkdirSync(join("inputs", "empty"), { recursive: true });
    expect(() => scanInputs({ empty: RasterFile })).toThrow(/empty/);
  });

  test("untyped multiple files defaults to FileCollection", () => {
    createInputCollection("data", ["a.tif", "b.tif"]);
    const result = scanInputs({});
    expect(result.data).toBeInstanceOf(FileCollection);
    expect((result.data as FileCollection).paths).toHaveLength(2);
  });

  test("no inputs directory returns empty", () => {
    // chdir to a temp dir without inputs/
    const { mkdtempSync } = require("node:fs");
    const { tmpdir } = require("node:os");
    const emptyDir = mkdtempSync(join(tmpdir(), "spade_empty_"));
    process.chdir(emptyDir);
    expect(scanInputs({})).toEqual({});
    process.chdir(workDir.path);
  });
});

describe("buildFunctionArgs", () => {
  test("params and inputs merged", () => {
    writeParams({ buffer: 30 });
    createInputFile("raster", "data.tif");
    const args = buildFunctionArgs({
      inputs: { raster: RasterFile, buffer: "number" },
    });
    expect(args.buffer).toBe(30);
    expect(args.raster).toBeInstanceOf(RasterFile);
  });

  test("inputs take precedence over params", () => {
    writeParams({ raster: "should_be_overridden" });
    createInputFile("raster", "data.tif");
    const args = buildFunctionArgs({ inputs: { raster: RasterFile } });
    expect(args.raster).toBeInstanceOf(RasterFile);
  });
});
