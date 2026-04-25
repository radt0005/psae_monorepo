import { describe, it, expect } from "vitest";
import { makeTestDb } from "./helpers/db";
import { makeBlockRepo } from "~/server/db/repositories/blocks";
import type { BlockManifest } from "~/utils/types";

const manifest: BlockManifest = {
  id: "gdal.rasterize",
  version: "1.0.0",
  kind: "standard",
  network: false,
  description: "Converts vector geometries to raster",
  inputs: { vectors: { type: "file", format: "GeoJSON" } },
  outputs: { raster: { type: "file", format: "GeoTIFF" } },
};

describe("BlockRepo", () => {
  it("upsert + getByName round-trip", async () => {
    const { db } = await makeTestDb();
    const repo = makeBlockRepo(db);
    await repo.upsert({ label: "GDAL Rasterize", manifest });
    const got = await repo.getByName("gdal.rasterize");
    expect(got?.label).toBe("GDAL Rasterize");
    expect(got?.version).toBe("1.0.0");
    expect(got?.manifest.outputs.raster.format).toBe("GeoTIFF");
  });

  it("upsert is idempotent and updates the manifest", async () => {
    const { db } = await makeTestDb();
    const repo = makeBlockRepo(db);
    await repo.upsert({ label: "v1", manifest });
    await repo.upsert({
      label: "v2",
      manifest: { ...manifest, version: "2.0.0" },
    });
    const got = await repo.getByName("gdal.rasterize");
    expect(got?.label).toBe("v2");
    expect(got?.version).toBe("2.0.0");
    const all = await repo.listAll();
    expect(all).toHaveLength(1);
  });

  it("listAll returns rows ordered by name", async () => {
    const { db } = await makeTestDb();
    const repo = makeBlockRepo(db);
    await repo.upsert({
      label: "z",
      manifest: { ...manifest, id: "z.last" },
    });
    await repo.upsert({
      label: "a",
      manifest: { ...manifest, id: "a.first" },
    });
    const all = await repo.listAll();
    expect(all.map((r) => r.name)).toEqual(["a.first", "z.last"]);
  });

  it("delete removes the row", async () => {
    const { db } = await makeTestDb();
    const repo = makeBlockRepo(db);
    await repo.upsert({ label: "x", manifest });
    await repo.delete("gdal.rasterize");
    expect(await repo.getByName("gdal.rasterize")).toBeNull();
  });
});
