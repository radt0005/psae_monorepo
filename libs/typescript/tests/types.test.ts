import { describe, expect, test } from "bun:test";
import {
  File,
  Directory,
  RasterFile,
  VectorFile,
  TabularFile,
  JsonFile,
  FileCollection,
  RasterFileCollection,
  VectorFileCollection,
  TabularFileCollection,
} from "../src/types.ts";

describe("File types", () => {
  test("File construction", () => {
    const f = new File("/tmp/data.tif");
    expect(f.path).toBe("/tmp/data.tif");
  });

  test("RasterFile extends File", () => {
    const f = new RasterFile("/tmp/data.tif");
    expect(f).toBeInstanceOf(File);
    expect(f.path).toBe("/tmp/data.tif");
  });

  test("VectorFile extends File", () => {
    const f = new VectorFile("/tmp/data.geojson");
    expect(f).toBeInstanceOf(File);
  });

  test("TabularFile extends File", () => {
    const f = new TabularFile("/tmp/data.csv");
    expect(f).toBeInstanceOf(File);
  });

  test("JsonFile extends File", () => {
    const f = new JsonFile("/tmp/data.json");
    expect(f).toBeInstanceOf(File);
  });

  test("Directory construction", () => {
    const d = new Directory("/tmp/dir");
    expect(d.path).toBe("/tmp/dir");
  });
});

describe("Collection types", () => {
  test("FileCollection construction", () => {
    const c = new FileCollection(["/tmp/a.tif", "/tmp/b.tif"]);
    expect(c.paths).toEqual(["/tmp/a.tif", "/tmp/b.tif"]);
  });

  test("RasterFileCollection extends FileCollection", () => {
    const c = new RasterFileCollection(["/tmp/a.tif"]);
    expect(c).toBeInstanceOf(FileCollection);
  });

  test("VectorFileCollection extends FileCollection", () => {
    const c = new VectorFileCollection(["/tmp/a.geojson"]);
    expect(c).toBeInstanceOf(FileCollection);
  });

  test("TabularFileCollection extends FileCollection", () => {
    const c = new TabularFileCollection(["/tmp/a.csv"]);
    expect(c).toBeInstanceOf(FileCollection);
  });
});
