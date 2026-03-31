import { describe, expect, test, beforeEach, afterEach } from "bun:test";
import {
  writeFileSync,
  mkdirSync,
  existsSync,
  readFileSync,
  readdirSync,
} from "node:fs";
import { join } from "node:path";
import yaml from "js-yaml";
import {
  inferOutputName,
  readBlockManifest,
  writeOutputs,
} from "../src/output.ts";
import {
  File,
  Directory,
  FileCollection,
  RasterFile,
  VectorFile,
  JsonFile,
  RasterFileCollection,
} from "../src/types.ts";
import { setupWorkDir } from "./helpers.ts";

let workDir: { path: string; originalCwd: string };

beforeEach(() => {
  workDir = setupWorkDir();
});

afterEach(() => {
  process.chdir(workDir.originalCwd);
});

describe("inferOutputName", () => {
  test("File", () => {
    expect(inferOutputName(new File("/tmp/a"))).toBe("file");
  });

  test("RasterFile", () => {
    expect(inferOutputName(new RasterFile("/tmp/a"))).toBe("raster");
  });

  test("VectorFile", () => {
    expect(inferOutputName(new VectorFile("/tmp/a"))).toBe("vector");
  });

  test("JsonFile", () => {
    expect(inferOutputName(new JsonFile("/tmp/a"))).toBe("json");
  });

  test("FileCollection", () => {
    expect(inferOutputName(new FileCollection([]))).toBe("files");
  });

  test("RasterFileCollection", () => {
    expect(inferOutputName(new RasterFileCollection([]))).toBe("rasters");
  });
});

describe("writeOutputs", () => {
  test("null result produces no output", () => {
    writeOutputs(null);
    expect(readdirSync(join(workDir.path, "outputs"))).toHaveLength(0);
  });

  test("single RasterFile output", () => {
    const src = join(workDir.path, "tmp_result.tif");
    writeFileSync(src, "raster data");
    writeOutputs(new RasterFile(src));
    const outputDir = join(workDir.path, "outputs", "raster");
    expect(existsSync(outputDir)).toBe(true);
    expect(existsSync(join(outputDir, "tmp_result.tif"))).toBe(true);
    expect(readFileSync(join(outputDir, "tmp_result.tif"), "utf-8")).toBe(
      "raster data"
    );
  });

  test("single file with manifest uses manifest name", () => {
    const src = join(workDir.path, "result.tif");
    writeFileSync(src, "data");
    const manifest = { custom_output: { type: "file", format: "GeoTIFF" } };
    writeOutputs(new RasterFile(src), manifest);
    expect(
      existsSync(join(workDir.path, "outputs", "custom_output", "result.tif"))
    ).toBe(true);
  });

  test("dict output", () => {
    const srcRaster = join(workDir.path, "result.tif");
    writeFileSync(srcRaster, "raster");
    const srcJson = join(workDir.path, "summary.json");
    writeFileSync(srcJson, '{"key": "value"}');

    writeOutputs({
      raster: new RasterFile(srcRaster),
      summary: new JsonFile(srcJson),
    });

    expect(
      existsSync(join(workDir.path, "outputs", "raster", "result.tif"))
    ).toBe(true);
    expect(
      existsSync(join(workDir.path, "outputs", "summary", "summary.json"))
    ).toBe(true);
  });

  test("collection output", () => {
    const paths: string[] = [];
    for (let i = 0; i < 3; i++) {
      const p = join(workDir.path, `tile_${i}.tif`);
      writeFileSync(p, `tile ${i}`);
      paths.push(p);
    }
    writeOutputs(new RasterFileCollection(paths));
    const outputDir = join(workDir.path, "outputs", "rasters");
    expect(existsSync(outputDir)).toBe(true);
    expect(readdirSync(outputDir)).toHaveLength(3);
  });

  test("directory output", () => {
    const srcDir = join(workDir.path, "result_dir");
    mkdirSync(srcDir);
    writeFileSync(join(srcDir, "file1.txt"), "a");
    writeFileSync(join(srcDir, "file2.txt"), "b");

    writeOutputs(new Directory(srcDir));
    const outputDir = join(workDir.path, "outputs", "directory");
    expect(existsSync(outputDir)).toBe(true);
    expect(existsSync(join(outputDir, "file1.txt"))).toBe(true);
    expect(existsSync(join(outputDir, "file2.txt"))).toBe(true);
  });

  test("preserves filename", () => {
    const src = join(workDir.path, "my_custom_name.geojson");
    writeFileSync(src, "geojson data");
    writeOutputs(new VectorFile(src));
    expect(
      existsSync(
        join(workDir.path, "outputs", "vector", "my_custom_name.geojson")
      )
    ).toBe(true);
  });
});

describe("readBlockManifest", () => {
  test("no manifest returns null", () => {
    expect(readBlockManifest()).toBeNull();
  });

  test("block.yaml in CWD", () => {
    const manifest = {
      id: "test.block",
      outputs: { raster: { type: "file", format: "GeoTIFF" } },
    };
    writeFileSync("block.yaml", yaml.dump(manifest));
    const result = readBlockManifest();
    expect(result).toEqual({ raster: { type: "file", format: "GeoTIFF" } });
  });

  test("env var manifest takes priority", () => {
    const manifestPath = join(workDir.path, "external_block.yaml");
    const manifest = { outputs: { output: { type: "file" } } };
    writeFileSync(manifestPath, yaml.dump(manifest));
    const original = process.env["SPADE_BLOCK_MANIFEST"];
    process.env["SPADE_BLOCK_MANIFEST"] = manifestPath;
    try {
      const result = readBlockManifest();
      expect(result).toEqual({ output: { type: "file" } });
    } finally {
      if (original === undefined) {
        delete process.env["SPADE_BLOCK_MANIFEST"];
      } else {
        process.env["SPADE_BLOCK_MANIFEST"] = original;
      }
    }
  });
});
