import { describe, it, expect, beforeEach } from "vitest";
import { makeTestDb } from "./helpers/db";
import { makePipelineRepo } from "~/server/db/repositories/pipelines";
import { users } from "~/server/db/schema/users";

async function seed() {
  const { db } = await makeTestDb();
  await db.insert(users).values([
    { id: "alice", email: "alice@ex.com", name: "Alice" },
    { id: "bob", email: "bob@ex.com", name: "Bob" },
    { id: "carol", email: "carol@ex.com", name: "Carol" },
  ]);
  return { db, repo: makePipelineRepo(db) };
}

describe("PipelineRepo", () => {
  it("create + listOwnedBy", async () => {
    const { repo } = await seed();
    await repo.create({
      id: "p1",
      ownerId: "alice",
      name: "Alice's pipeline",
      version: "1.0",
      yaml: "blocks: []",
      visibility: "private",
    });
    const mine = await repo.listOwnedBy("alice");
    expect(mine).toHaveLength(1);
    const others = await repo.listOwnedBy("bob");
    expect(others).toHaveLength(0);
  });

  it("listPublic only returns public rows", async () => {
    const { repo } = await seed();
    await repo.create({
      id: "priv",
      ownerId: "alice",
      name: "priv",
      version: "1.0",
      yaml: "blocks: []",
      visibility: "private",
    });
    await repo.create({
      id: "pub",
      ownerId: "alice",
      name: "pub",
      version: "1.0",
      yaml: "blocks: []",
      visibility: "public",
    });
    const pub = await repo.listPublic();
    expect(pub.map((r) => r.id)).toEqual(["pub"]);
  });

  it("share + listSharedWith + unshare", async () => {
    const { repo } = await seed();
    await repo.create({
      id: "p1",
      ownerId: "alice",
      name: "A",
      version: "1",
      yaml: "blocks: []",
      visibility: "shared",
    });
    expect(await repo.share("p1", "alice", "bob", "view")).toBe(true);
    const bob = await repo.listSharedWith("bob");
    expect(bob).toHaveLength(1);
    const carol = await repo.listSharedWith("carol");
    expect(carol).toHaveLength(0);

    expect(await repo.unshare("p1", "alice", "bob")).toBe(true);
    expect(await repo.listSharedWith("bob")).toHaveLength(0);
  });

  it("share rejects non-owners", async () => {
    const { repo } = await seed();
    await repo.create({
      id: "p1",
      ownerId: "alice",
      name: "A",
      version: "1",
      yaml: "blocks: []",
      visibility: "private",
    });
    // bob tries to share alice's pipeline with carol — should fail
    expect(await repo.share("p1", "bob", "carol", "view")).toBe(false);
    expect(await repo.listSharedWith("carol")).toHaveLength(0);
  });

  it("getByIdForUser enforces ACL: owner OK", async () => {
    const { repo } = await seed();
    await repo.create({
      id: "p1",
      ownerId: "alice",
      name: "A",
      version: "1",
      yaml: "blocks: []",
      visibility: "private",
    });
    expect(await repo.getByIdForUser("p1", "alice")).not.toBeNull();
  });

  it("getByIdForUser enforces ACL: stranger blocked on private", async () => {
    const { repo } = await seed();
    await repo.create({
      id: "p1",
      ownerId: "alice",
      name: "A",
      version: "1",
      yaml: "blocks: []",
      visibility: "private",
    });
    expect(await repo.getByIdForUser("p1", "bob")).toBeNull();
  });

  it("getByIdForUser enforces ACL: shared sees it, public any user", async () => {
    const { repo } = await seed();
    await repo.create({
      id: "p1",
      ownerId: "alice",
      name: "A",
      version: "1",
      yaml: "blocks: []",
      visibility: "shared",
    });
    await repo.share("p1", "alice", "bob");
    expect(await repo.getByIdForUser("p1", "bob")).not.toBeNull();
    expect(await repo.getByIdForUser("p1", "carol")).toBeNull();

    await repo.update("p1", "alice", { visibility: "public" });
    expect(await repo.getByIdForUser("p1", "carol")).not.toBeNull();
  });

  it("update rejects non-owners", async () => {
    const { repo } = await seed();
    await repo.create({
      id: "p1",
      ownerId: "alice",
      name: "A",
      version: "1",
      yaml: "blocks: []",
      visibility: "private",
    });
    const row = await repo.update("p1", "bob", { name: "malicious" });
    expect(row).toBeNull();
  });

  it("delete rejects non-owners", async () => {
    const { repo } = await seed();
    await repo.create({
      id: "p1",
      ownerId: "alice",
      name: "A",
      version: "1",
      yaml: "blocks: []",
      visibility: "private",
    });
    expect(await repo.delete("p1", "bob")).toBe(false);
    expect(await repo.delete("p1", "alice")).toBe(true);
    expect(await repo.listOwnedBy("alice")).toHaveLength(0);
  });
});
