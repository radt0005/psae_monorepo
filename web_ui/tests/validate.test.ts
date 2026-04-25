import { describe, it, expect } from "vitest";
import { validatePipeline } from "~/utils/validate";
import type { BlockManifest, Pipeline } from "~/utils/types";

const pipeline = (overrides: Partial<Pipeline> = {}): Pipeline => ({
  id: "p",
  name: "test",
  version: "1.0",
  blocks: [],
  ...overrides,
});

describe("validatePipeline — structural checks", () => {
  it("passes a trivial single-source pipeline", () => {
    const p = pipeline({
      blocks: [{ id: "a", name: "src", inputs: [], args: {} }],
    });
    expect(validatePipeline(p).ok).toBe(true);
  });

  it("flags duplicate block ids", () => {
    const p = pipeline({
      blocks: [
        { id: "a", name: "src", inputs: [], args: {} },
        { id: "a", name: "src", inputs: [], args: {} },
      ],
    });
    const r = validatePipeline(p);
    expect(r.ok).toBe(false);
    expect(r.errors[0]).toMatch(/Duplicate block id/);
  });

  it("flags missing referenced block ids (bare)", () => {
    const p = pipeline({
      blocks: [
        { id: "b", name: "x", inputs: ["does-not-exist"], args: {} },
      ],
    });
    const r = validatePipeline(p);
    expect(r.ok).toBe(false);
    expect(r.errors[0]).toMatch(/missing block id/);
  });

  it("flags missing referenced block ids (explicit)", () => {
    const p = pipeline({
      blocks: [
        {
          id: "b",
          name: "x",
          inputs: [{ block: "ghost", output: "anything" }],
          args: {},
        },
      ],
    });
    const r = validatePipeline(p);
    expect(r.ok).toBe(false);
  });

  it("detects a cycle", () => {
    const p = pipeline({
      blocks: [
        { id: "a", name: "x", inputs: ["b"], args: {} },
        { id: "b", name: "x", inputs: ["a"], args: {} },
      ],
    });
    const r = validatePipeline(p);
    expect(r.ok).toBe(false);
    expect(r.errors.find((e) => e.includes("cycle"))).toBeDefined();
  });

  it("does not flag legitimate fan-in", () => {
    const p = pipeline({
      blocks: [
        { id: "a", name: "x", inputs: [], args: {} },
        { id: "b", name: "x", inputs: [], args: {} },
        { id: "c", name: "x", inputs: ["a", "b"], args: {} },
      ],
    });
    expect(validatePipeline(p).ok).toBe(true);
  });
});

describe("validatePipeline — manifest-aware checks", () => {
  const manifests: Record<string, BlockManifest> = {
    "data.src": {
      id: "data.src",
      version: "1.0",
      kind: "standard",
      network: false,
      inputs: {},
      outputs: { raster: { type: "file", format: "GeoTIFF" } },
    },
    "raster.process": {
      id: "raster.process",
      version: "1.0",
      kind: "standard",
      network: false,
      inputs: {
        source: { type: "file", format: "GeoTIFF" },
        threshold: { type: "number" },
      },
      outputs: { result: { type: "file" } },
    },
    "core.map.files": {
      id: "core.map.files",
      version: "1.0",
      kind: "map",
      network: false,
      inputs: { source: { type: "collection" } },
      outputs: { manifest: { type: "expansion" } },
    },
    "core.map.broken": {
      id: "core.map.broken",
      version: "1.0",
      kind: "map",
      network: false,
      inputs: { source: { type: "collection" } },
      outputs: { wrong: { type: "file" } }, // missing expansion
    },
    "core.reduce.mosaic": {
      id: "core.reduce.mosaic",
      version: "1.0",
      kind: "reduce",
      network: false,
      inputs: { tiles: { type: "collection", item_type: "file" } },
      outputs: { mosaic: { type: "file" } },
    },
    "core.reduce.broken": {
      id: "core.reduce.broken",
      version: "1.0",
      kind: "reduce",
      network: false,
      inputs: { x: { type: "file" } }, // no collection
      outputs: { result: { type: "file" } },
    },
  };

  it("flags unknown block names", () => {
    const p = pipeline({
      blocks: [{ id: "a", name: "ghost", inputs: [], args: {} }],
    });
    const r = validatePipeline(p, { manifests });
    expect(r.ok).toBe(false);
  });

  it("flags missing required scalar args", () => {
    const p = pipeline({
      blocks: [
        { id: "a", name: "data.src", inputs: [], args: {} },
        // missing `threshold` arg
        { id: "b", name: "raster.process", inputs: ["a"], args: {} },
      ],
    });
    const r = validatePipeline(p, { manifests });
    expect(r.ok).toBe(false);
    expect(r.errors.find((e) => e.includes("threshold"))).toBeDefined();
  });

  it("accepts a complete pipeline with the args present", () => {
    const p = pipeline({
      blocks: [
        { id: "a", name: "data.src", inputs: [], args: {} },
        {
          id: "b",
          name: "raster.process",
          inputs: ["a"],
          args: { threshold: 0.5 },
        },
      ],
    });
    const r = validatePipeline(p, { manifests });
    expect(r.ok).toBe(true);
  });

  it("flags explicit references to non-existent outputs", () => {
    const p = pipeline({
      blocks: [
        { id: "a", name: "data.src", inputs: [], args: {} },
        {
          id: "b",
          name: "raster.process",
          inputs: [{ block: "a", output: "nope" }],
          args: { threshold: 1 },
        },
      ],
    });
    const r = validatePipeline(p, { manifests });
    expect(r.ok).toBe(false);
    expect(r.errors.find((e) => e.includes("nope"))).toBeDefined();
  });

  it("requires map blocks to have an expansion output", () => {
    const p = pipeline({
      blocks: [{ id: "m", name: "core.map.broken", inputs: [], args: {} }],
    });
    const r = validatePipeline(p, { manifests });
    expect(r.ok).toBe(false);
    expect(r.errors.find((e) => e.includes("expansion"))).toBeDefined();
  });

  it("requires reduce blocks to have a collection input", () => {
    const p = pipeline({
      blocks: [{ id: "r", name: "core.reduce.broken", inputs: [], args: {} }],
    });
    const r = validatePipeline(p, { manifests });
    expect(r.ok).toBe(false);
    expect(r.errors.find((e) => e.includes("collection"))).toBeDefined();
  });

  it("accepts a well-formed map → reduce pipeline", () => {
    const p = pipeline({
      blocks: [
        { id: "src", name: "data.src", inputs: [], args: {} },
        { id: "m", name: "core.map.files", inputs: ["src"], args: {} },
        { id: "r", name: "core.reduce.mosaic", inputs: ["m"], args: {} },
      ],
    });
    const r = validatePipeline(p, { manifests });
    expect(r.ok).toBe(true);
  });
});
