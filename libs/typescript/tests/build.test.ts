import { describe, expect, test } from "bun:test";
import { build } from "../src/build.ts";
import { spadeBlock } from "../src/metadata.ts";
import {
  File,
  Directory,
  RasterFile,
  VectorFile,
  JsonFile,
  FileCollection,
  RasterFileCollection,
} from "../src/types.ts";

describe("build", () => {
  test("simple function with File input", () => {
    const handler = spadeBlock({ inputs: { source: File } })(() => {});
    const manifest = build(handler);
    expect(manifest.inputs).toHaveProperty("source");
    expect((manifest.inputs as any).source.type).toBe("file");
  });

  test("typed inputs", () => {
    const handler = spadeBlock({
      inputs: { raster: RasterFile, vector: VectorFile },
    })(() => {});
    const manifest = build(handler);
    expect((manifest.inputs as any).raster).toEqual({
      type: "file",
      format: "GeoTIFF",
    });
    expect((manifest.inputs as any).vector).toEqual({
      type: "file",
      format: "GeoJSON",
    });
  });

  test("scalar inputs", () => {
    const handler = spadeBlock({
      inputs: { name: "string", count: "number", enabled: "boolean" },
    })(() => {});
    const manifest = build(handler);
    expect((manifest.inputs as any).name.type).toBe("string");
    expect((manifest.inputs as any).count.type).toBe("number");
    expect((manifest.inputs as any).enabled.type).toBe("boolean");
  });

  test("with return type", () => {
    const handler = spadeBlock({
      inputs: { source: File },
      output: RasterFile,
    })(() => {});
    const manifest = build(handler);
    expect(manifest.outputs).toHaveProperty("raster");
    expect((manifest.outputs as any).raster.type).toBe("file");
    expect((manifest.outputs as any).raster.format).toBe("GeoTIFF");
  });

  test("with description", () => {
    const handler = spadeBlock({
      inputs: { source: File },
      description: "Processes input data.",
    })(() => {});
    const manifest = build(handler);
    expect(manifest.description).toBe("Processes input data.");
  });

  test("no metadata returns empty", () => {
    const handler = () => {};
    const manifest = build(handler);
    expect(manifest.inputs).toEqual({});
    expect(manifest.outputs).toEqual({});
  });

  test("collection input", () => {
    const handler = spadeBlock({
      inputs: { tiles: RasterFileCollection },
    })(() => {});
    const manifest = build(handler);
    expect((manifest.inputs as any).tiles.type).toBe("collection");
    expect((manifest.inputs as any).tiles.item_type).toBe("file");
    expect((manifest.inputs as any).tiles.format).toBe("GeoTIFF");
  });

  test("no return type gives empty outputs", () => {
    const handler = spadeBlock({ inputs: { source: File } })(() => {});
    const manifest = build(handler);
    expect(manifest.outputs).toEqual({});
  });

  test("mixed inputs with output and description", () => {
    const handler = spadeBlock({
      inputs: { raster: RasterFile, buffer: "number", normalize: "boolean" },
      output: RasterFile,
      description: "Normalizes raster data.",
    })(() => {});
    const manifest = build(handler);
    const inputs = manifest.inputs as any;
    expect(Object.keys(inputs)).toHaveLength(3);
    expect(inputs.raster.type).toBe("file");
    expect(inputs.buffer.type).toBe("number");
    expect(inputs.normalize.type).toBe("boolean");
    expect((manifest.outputs as any).raster.type).toBe("file");
    expect(manifest.description).toBe("Normalizes raster data.");
  });

  test("float input maps to number", () => {
    const handler = spadeBlock({
      inputs: { value: "number" },
    })(() => {});
    const manifest = build(handler);
    expect((manifest.inputs as any).value.type).toBe("number");
  });
});
