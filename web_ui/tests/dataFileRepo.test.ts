import { describe, it, expect } from "vitest";
import { makeTestDb } from "./helpers/db";
import { makeDataFileRepo } from "~/server/db/repositories/dataFiles";
import { users } from "~/server/db/schema/users";

async function seed() {
  const { db } = await makeTestDb();
  await db.insert(users).values([
    { id: "alice", email: "alice@ex.com" },
    { id: "bob", email: "bob@ex.com" },
    { id: "carol", email: "carol@ex.com" },
  ]);
  return { db, repo: makeDataFileRepo(db) };
}

const sampleFile = {
  id: "f1",
  ownerId: "alice",
  name: "tiles.zip",
  sizeBytes: 5_000_000_000, // 5 GB — geospatial-realistic
  mimeType: "application/zip",
  s3Key: "data/alice/f1/tiles.zip",
  visibility: "private" as const,
};

describe("DataFileRepo", () => {
  it("create + listOwnedBy preserves >2 GB sizes", async () => {
    const { repo } = await seed();
    await repo.create(sampleFile);
    const mine = await repo.listOwnedBy("alice");
    expect(mine).toHaveLength(1);
    expect(mine[0].sizeBytes).toBe(5_000_000_000);
  });

  it("getByIdForUser ACL: owner OK, stranger blocked", async () => {
    const { repo } = await seed();
    await repo.create(sampleFile);
    expect(await repo.getByIdForUser("f1", "alice")).not.toBeNull();
    expect(await repo.getByIdForUser("f1", "bob")).toBeNull();
  });

  it("share grants access; non-owner cannot share", async () => {
    const { repo } = await seed();
    await repo.create({ ...sampleFile, visibility: "shared" });
    expect(await repo.share("f1", "alice", "bob")).toBe(true);
    expect(await repo.getByIdForUser("f1", "bob")).not.toBeNull();
    // bob can't share alice's file with carol
    expect(await repo.share("f1", "bob", "carol")).toBe(false);
    expect(await repo.getByIdForUser("f1", "carol")).toBeNull();
  });

  it("public visibility lets any user access", async () => {
    const { repo } = await seed();
    await repo.create({ ...sampleFile, visibility: "public" });
    expect(await repo.getByIdForUser("f1", "carol")).not.toBeNull();
    expect((await repo.listPublic())[0].id).toBe("f1");
  });

  it("delete returns the row and is owner-scoped", async () => {
    const { repo } = await seed();
    await repo.create(sampleFile);
    expect(await repo.delete("f1", "bob")).toBeNull();
    const removed = await repo.delete("f1", "alice");
    expect(removed?.s3Key).toBe("data/alice/f1/tiles.zip");
    expect(await repo.listOwnedBy("alice")).toHaveLength(0);
  });
});
