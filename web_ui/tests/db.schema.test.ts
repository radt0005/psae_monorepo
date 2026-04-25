import { describe, it, expect } from "vitest";
import { eq } from "drizzle-orm";
import { makeTestDb } from "./helpers/db";
import { users, blocks, pipelines, dataFiles } from "~/server/db/schema";

describe("db schema — round trip", () => {
  it("inserts and reads a user", async () => {
    const { db } = await makeTestDb();
    await db.insert(users).values({
      id: "u1",
      email: "alice@example.com",
      name: "Alice",
    });
    const rows = await db
      .select()
      .from(users)
      .where(eq(users.email, "alice@example.com"));
    expect(rows).toHaveLength(1);
    expect(rows[0].name).toBe("Alice");
  });

  it("stores a BlockManifest as jsonb", async () => {
    const { db } = await makeTestDb();
    await db.insert(blocks).values({
      id: "gdal.rasterize",
      name: "gdal.rasterize",
      label: "GDAL Rasterize",
      version: "1.0.0",
      manifest: {
        id: "gdal.rasterize",
        version: "1.0.0",
        kind: "standard",
        network: false,
        inputs: { vectors: { type: "file", format: "GeoJSON" } },
        outputs: { raster: { type: "file", format: "GeoTIFF" } },
      },
    });
    const rows = await db
      .select()
      .from(blocks)
      .where(eq(blocks.name, "gdal.rasterize"));
    expect(rows[0].manifest.outputs.raster.format).toBe("GeoTIFF");
  });

  it("enforces FK from pipelines.owner_id → user.id", async () => {
    const { db } = await makeTestDb();
    await db.insert(users).values({
      id: "u1",
      email: "alice@example.com",
    });
    await db.insert(pipelines).values({
      id: "p1",
      ownerId: "u1",
      name: "test",
      version: "1.0",
      yaml: "blocks: []",
      visibility: "private",
    });
    const list = await db.select().from(pipelines);
    expect(list).toHaveLength(1);
    expect(list[0].visibility).toBe("private");
  });

  it("data_files records carry s3_key + size", async () => {
    const { db } = await makeTestDb();
    await db.insert(users).values({ id: "u1", email: "a@b.com" });
    await db.insert(dataFiles).values({
      id: "f1",
      ownerId: "u1",
      name: "tiles.zip",
      sizeBytes: 1_000_000_000,
      mimeType: "application/zip",
      s3Key: "u1/f1.zip",
      visibility: "private",
    });
    const r = await db.select().from(dataFiles);
    expect(r[0].sizeBytes).toBe(1_000_000_000);
    expect(r[0].s3Key).toBe("u1/f1.zip");
  });
});
