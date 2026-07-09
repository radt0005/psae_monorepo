import { describe, expect, test, beforeEach, afterEach } from "bun:test";
import { writeFileSync, existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { Glob } from "bun";
import yaml from "js-yaml";
import { run } from "../src/run.ts";
import { spadeBlock } from "../src/metadata.ts";
import {
  File,
  RasterFile,
  JsonFile,
  FileCollection,
} from "../src/types.ts";
import {
  setupWorkDir,
  createInputFile,
  writeParams,
} from "./helpers.ts";

let workDir: { path: string; originalCwd: string };

beforeEach(() => {
  workDir = setupWorkDir();
});

afterEach(() => {
  process.chdir(workDir.originalCwd);
});

function rglob(dir: string): string[] {
  const results: string[] = [];
  const entries = readdirSync(dir, { withFileTypes: true });
  for (const e of entries) {
    const full = join(dir, e.name);
    if (e.isDirectory()) {
      results.push(...rglob(full));
    } else {
      results.push(full);
    }
  }
  return results;
}

describe("run", () => {
  test("simple handler", () => {
    createInputFile("source", "data.tif");
    let calledWith: Record<string, unknown> = {};

    const handler = spadeBlock({ inputs: { source: File } })(
      (args: { source: File }) => {
        calledWith = args;
      }
    );

    run(handler);
    expect(calledWith.source).toBeInstanceOf(File);
  });

  test("with params and inputs", () => {
    writeParams({ buffer: 30, method: "bilinear" });
    createInputFile("raster", "data.tif");
    let calledWith: Record<string, unknown> = {};

    const handler = spadeBlock({
      inputs: { raster: RasterFile, buffer: "number", method: "string" },
    })((args: { raster: RasterFile; buffer: number; method: string }) => {
      calledWith = args;
    });

    run(handler);
    expect(calledWith.raster).toBeInstanceOf(RasterFile);
    expect(calledWith.buffer).toBe(30);
    expect(calledWith.method).toBe("bilinear");
  });

  test("with typed inputs", () => {
    createInputFile("image", "satellite.tif");
    let calledWith: Record<string, unknown> = {};

    const handler = spadeBlock({ inputs: { image: RasterFile } })(
      (args: { image: RasterFile }) => {
        calledWith = args;
      }
    );

    run(handler);
    expect(calledWith.image).toBeInstanceOf(RasterFile);
  });

  test("with output", () => {
    createInputFile("source", "data.tif");
    const resultPath = join(workDir.path, "processed.tif");
    writeFileSync(resultPath, "processed data");

    const handler = spadeBlock({
      inputs: { source: RasterFile },
      output: RasterFile,
    })((args: { source: RasterFile }) => {
      return new RasterFile(resultPath);
    });

    run(handler);
    const outputFiles = rglob(join(workDir.path, "outputs"));
    expect(outputFiles).toHaveLength(1);
    expect(readFileSync(outputFiles[0], "utf-8")).toBe("processed data");
  });

  test("with dict output", () => {
    createInputFile("source", "data.tif");
    const rasterPath = join(workDir.path, "result.tif");
    writeFileSync(rasterPath, "raster");
    const jsonPath = join(workDir.path, "stats.json");
    writeFileSync(jsonPath, '{"mean": 42}');

    const handler = spadeBlock({ inputs: { source: File } })(
      (args: { source: File }) => {
        return {
          raster: new RasterFile(rasterPath),
          stats: new File(jsonPath),
        };
      }
    );

    run(handler);
    expect(
      existsSync(join(workDir.path, "outputs", "raster", "result.tif"))
    ).toBe(true);
    expect(
      existsSync(join(workDir.path, "outputs", "stats", "stats.json"))
    ).toBe(true);
  });

  test("async handler output is awaited before writing", async () => {
    createInputFile("source", "data.tif");
    const resultPath = join(workDir.path, "async.tif");
    writeFileSync(resultPath, "async data");

    const handler = spadeBlock({
      inputs: { source: File },
      output: File,
    })(async (args: { source: File }) => {
      await Promise.resolve();
      return new File(resultPath);
    });

    await run(handler);
    const outputFiles = rglob(join(workDir.path, "outputs"));
    expect(outputFiles).toHaveLength(1);
    expect(readFileSync(outputFiles[0], "utf-8")).toBe("async data");
  });

  test("async handler rejection propagates", async () => {
    createInputFile("source", "data.tif");

    const handler = spadeBlock({ inputs: { source: File } })(
      async (args: { source: File }) => {
        throw new Error("async processing failed");
      }
    );

    await expect(run(handler)).rejects.toThrow("async processing failed");
  });

  test("handler exception propagates", () => {
    createInputFile("source", "data.tif");

    const handler = spadeBlock({ inputs: { source: File } })(
      (args: { source: File }) => {
        throw new Error("processing failed");
      }
    );

    expect(() => run(handler)).toThrow("processing failed");
  });

  test("no return value produces no output", () => {
    createInputFile("source", "data.tif");

    const handler = spadeBlock({ inputs: { source: File } })(
      (args: { source: File }) => {}
    );

    run(handler);
    const outputFiles = rglob(join(workDir.path, "outputs"));
    expect(outputFiles).toHaveLength(0);
  });

  test("extra params are filtered", () => {
    writeParams({ expected: "value", extra: "ignored" });
    createInputFile("source", "data.tif");
    let calledWith: Record<string, unknown> = {};

    const handler = spadeBlock({
      inputs: { source: File, expected: "string" },
    })((args: Record<string, unknown>) => {
      calledWith = args;
    });

    run(handler);
    expect(calledWith.expected).toBe("value");
    expect(calledWith.extra).toBeUndefined();
  });

  test("full workflow", () => {
    writeParams({ resolution: 10, method: "nearest" });
    createInputFile("reference", "ref.tif", "reference raster data");
    createInputFile("target", "tgt.tif", "target raster data");

    const resultPath = join(workDir.path, "reprojected.tif");
    writeFileSync(resultPath, "reprojected output");

    const handler = spadeBlock({
      inputs: {
        reference: RasterFile,
        target: RasterFile,
        resolution: "number",
        method: "string",
      },
      output: RasterFile,
    })(
      (args: {
        reference: RasterFile;
        target: RasterFile;
        resolution: number;
        method: string;
      }) => {
        expect(args.reference.path).toContain("ref.tif");
        expect(args.target.path).toContain("tgt.tif");
        expect(args.resolution).toBe(10);
        expect(args.method).toBe("nearest");
        return new RasterFile(resultPath);
      }
    );

    run(handler);
    const outputFiles = rglob(join(workDir.path, "outputs"));
    expect(outputFiles).toHaveLength(1);
    expect(readFileSync(outputFiles[0], "utf-8")).toBe("reprojected output");
  });
});
