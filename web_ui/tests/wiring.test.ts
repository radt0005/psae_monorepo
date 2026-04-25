import { describe, it, expect } from "vitest";
import {
  portsCompatible,
  resolveConnection,
  hasUnresolvedEdges,
} from "~/utils/wiring";
import type { BlockManifest, EdgeData } from "~/utils/types";

const manifest = (
  id: string,
  inputs: BlockManifest["inputs"],
  outputs: BlockManifest["outputs"],
): BlockManifest => ({
  id,
  version: "1.0",
  kind: "standard",
  network: false,
  inputs,
  outputs,
});

describe("portsCompatible", () => {
  it("matches identical primary types", () => {
    expect(
      portsCompatible(
        { type: "file", format: "GeoTIFF" },
        { type: "file", format: "GeoTIFF" },
      ),
    ).toBe(true);
  });
  it("rejects different primary types", () => {
    expect(
      portsCompatible({ type: "file" }, { type: "directory" }),
    ).toBe(false);
  });
  it("collection requires matching item_type when both declared", () => {
    expect(
      portsCompatible(
        { type: "collection", item_type: "file" },
        { type: "collection", item_type: "file" },
      ),
    ).toBe(true);
    expect(
      portsCompatible(
        { type: "collection", item_type: "file" },
        { type: "collection", item_type: "directory" },
      ),
    ).toBe(false);
  });
});

describe("resolveConnection", () => {
  it("matches one-to-one unambiguously", () => {
    const upstream = manifest(
      "data.sentinel2",
      {},
      { raster: { type: "file", format: "GeoTIFF" } },
    );
    const downstream = manifest(
      "raster.reproject",
      { source: { type: "file", format: "GeoTIFF" } },
      { reprojected: { type: "file" } },
    );
    const out = resolveConnection(upstream, downstream);
    expect(out.kind).toBe("ok");
    if (out.kind === "ok") {
      expect(out.bindings).toEqual([{ output: "raster", input: "source" }]);
    }
  });

  it("flags ambiguous mappings (raster.split → ?) per web_ui.md example", () => {
    const upstream = manifest(
      "raster.split",
      {},
      {
        tiles: {
          type: "file",
          format: "GeoTIFF",
          description: "Individual raster tiles",
        },
        boundary: {
          type: "file",
          format: "GeoTIFF",
          description: "Extent polygon rasterized to grid",
        },
      },
    );
    const downstream = manifest(
      "raster.process",
      {
        source: {
          type: "file",
          format: "GeoTIFF",
          description: "Raster to process",
        },
      },
      {},
    );
    const out = resolveConnection(upstream, downstream);
    expect(out.kind).toBe("ambiguous");
    if (out.kind === "ambiguous") {
      expect(out.candidates.source.sort()).toEqual(["boundary", "tiles"]);
      expect(out.inputDescriptions.source).toBe("Raster to process");
      expect(out.outputDescriptions.tiles).toBe("Individual raster tiles");
      expect(out.outputDescriptions.boundary).toBe(
        "Extent polygon rasterized to grid",
      );
    }
  });

  it("explicit bindings remove their ports from further consideration", () => {
    const upstream = manifest(
      "raster.split",
      {},
      {
        tiles: { type: "file", format: "GeoTIFF" },
        boundary: { type: "file", format: "GeoTIFF" },
      },
    );
    const downstream = manifest(
      "raster.process",
      {
        source: { type: "file", format: "GeoTIFF" },
        mask: { type: "file", format: "GeoTIFF" },
      },
      {},
    );
    const out = resolveConnection(upstream, downstream, [
      { output: "tiles", input: "source" },
    ]);
    expect(out.kind).toBe("ok");
    if (out.kind === "ok") {
      expect(out.bindings).toEqual([
        { output: "tiles", input: "source" },
        { output: "boundary", input: "mask" }, // unambiguous after step 1
      ]);
    }
  });

  it("rejects an explicit binding that names a missing output", () => {
    const upstream = manifest("u", {}, { a: { type: "file" } });
    const downstream = manifest("d", { x: { type: "file" } }, {});
    const out = resolveConnection(upstream, downstream, [
      { output: "missing", input: "x" },
    ]);
    expect(out.kind).toBe("invalid");
  });

  it("rejects an explicit binding with incompatible types", () => {
    const upstream = manifest("u", {}, { a: { type: "file" } });
    const downstream = manifest("d", { x: { type: "directory" } }, {});
    const out = resolveConnection(upstream, downstream, [
      { output: "a", input: "x" },
    ]);
    expect(out.kind).toBe("invalid");
  });

  it("returns ok with partial bindings if some downstream inputs have no candidate", () => {
    // Other upstream blocks may fill the remaining inputs — not our problem here.
    const upstream = manifest("u", {}, { a: { type: "file" } });
    const downstream = manifest(
      "d",
      { x: { type: "file" }, y: { type: "directory" } },
      {},
    );
    const out = resolveConnection(upstream, downstream);
    expect(out.kind).toBe("ok");
    if (out.kind === "ok") {
      expect(out.bindings).toEqual([{ output: "a", input: "x" }]);
    }
  });

  it("two-of-each-type triggers ambiguity rather than an arbitrary pairing", () => {
    const upstream = manifest(
      "u",
      {},
      { a: { type: "file" }, b: { type: "file" } },
    );
    const downstream = manifest(
      "d",
      { x: { type: "file" }, y: { type: "file" } },
      {},
    );
    const out = resolveConnection(upstream, downstream);
    expect(out.kind).toBe("ambiguous");
  });
});

describe("hasUnresolvedEdges", () => {
  it("reports true if any edge.data.unresolved is set", () => {
    const edges = [
      { id: "1", source: "a", target: "b", data: { unresolved: false } as EdgeData },
      { id: "2", source: "b", target: "c", data: { unresolved: true } as EdgeData },
    ];
    expect(hasUnresolvedEdges(edges)).toBe(true);
  });
  it("reports false if no edge is unresolved", () => {
    const edges = [
      { id: "1", source: "a", target: "b", data: { unresolved: false } as EdgeData },
    ];
    expect(hasUnresolvedEdges(edges)).toBe(false);
  });
});
