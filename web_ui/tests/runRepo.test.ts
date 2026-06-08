import { describe, it, expect } from "vitest";
import { makeTestDb } from "./helpers/db";
import { makeRunRepo } from "~/server/db/repositories/runs";
import { users } from "~/server/db/schema/users";

async function seed() {
  const { db } = await makeTestDb();
  await db.insert(users).values([
    { id: "alice", email: "alice@ex.com", name: "Alice" },
    { id: "bob", email: "bob@ex.com", name: "Bob" },
    { id: "carol", email: "carol@ex.com", name: "Carol" },
  ]);
  return { db, repo: makeRunRepo(db) };
}

const baseRun = (over: Record<string, unknown> = {}) => ({
  id: "r1",
  ownerId: "alice",
  yaml: "blocks: []",
  status: "queued" as const,
  visibility: "private" as const,
  ...over,
});

describe("RunRepo", () => {
  it("create + listOwnedBy is owner-scoped", async () => {
    const { repo } = await seed();
    await repo.create(baseRun());
    expect(await repo.listOwnedBy("alice")).toHaveLength(1);
    expect(await repo.listOwnedBy("bob")).toHaveLength(0);
  });

  it("create allows a null pipelineId (ad-hoc submissions)", async () => {
    const { repo } = await seed();
    const run = await repo.create(baseRun({ pipelineId: null }));
    expect(run.pipelineId).toBeNull();
  });

  it("listPublic only returns public rows", async () => {
    const { repo } = await seed();
    await repo.create(baseRun({ id: "priv", visibility: "private" }));
    await repo.create(baseRun({ id: "pub", visibility: "public" }));
    const pub = await repo.listPublic();
    expect(pub.map((r) => r.id)).toEqual(["pub"]);
  });

  it("getByIdForUser enforces ACL across owner/stranger/shared/public", async () => {
    const { repo } = await seed();
    await repo.create(baseRun({ visibility: "private" }));

    expect(await repo.getByIdForUser("r1", "alice")).not.toBeNull();
    expect(await repo.getByIdForUser("r1", "bob")).toBeNull();

    expect(await repo.share("r1", "alice", "bob")).toBe(true);
    expect(await repo.getByIdForUser("r1", "bob")).not.toBeNull();
    expect(await repo.getByIdForUser("r1", "carol")).toBeNull();

    await repo.setVisibility("r1", "alice", "public");
    expect(await repo.getByIdForUser("r1", "carol")).not.toBeNull();
  });

  it("share + listSharedWith + unshare", async () => {
    const { repo } = await seed();
    await repo.create(baseRun({ visibility: "shared" }));
    expect(await repo.share("r1", "alice", "bob", "view")).toBe(true);
    expect(await repo.listSharedWith("bob")).toHaveLength(1);
    expect(await repo.listSharedWith("carol")).toHaveLength(0);
    expect(await repo.unshare("r1", "alice", "bob")).toBe(true);
    expect(await repo.listSharedWith("bob")).toHaveLength(0);
  });

  it("share/unshare/setVisibility reject non-owners", async () => {
    const { repo } = await seed();
    await repo.create(baseRun());
    expect(await repo.share("r1", "bob", "carol")).toBe(false);
    expect(await repo.unshare("r1", "bob", "carol")).toBe(false);
    expect(await repo.setVisibility("r1", "bob", "public")).toBeNull();
  });

  it("updateStatus stamps started/finished timestamps", async () => {
    const { repo } = await seed();
    await repo.create(baseRun());

    const running = await repo.updateStatus("r1", "running");
    expect(running?.status).toBe("running");
    expect(running?.startedAt).not.toBeNull();
    expect(running?.finishedAt).toBeNull();

    const done = await repo.updateStatus("r1", "succeeded");
    expect(done?.status).toBe("succeeded");
    expect(done?.finishedAt).not.toBeNull();
  });

  it("updateStatus records a failure reason", async () => {
    const { repo } = await seed();
    await repo.create(baseRun());
    const failed = await repo.updateStatus("r1", "failed", {
      error: "block exited 1",
    });
    expect(failed?.status).toBe("failed");
    expect(failed?.error).toBe("block exited 1");
    expect(failed?.finishedAt).not.toBeNull();
  });

  it("appendFile + appendLog + getWithDetails round-trip", async () => {
    const { repo } = await seed();
    await repo.create(baseRun());
    await repo.appendFile({
      runId: "r1",
      blockId: "blk-1",
      name: "result.tif",
      mimeType: "image/tiff",
      sizeBytes: 5_000_000_000, // >2GB — bigint column
      s3Key: "runs/r1/blk-1/raster/result.tif",
    });
    await repo.appendLog({
      runId: "r1",
      blockId: "blk-1",
      stdout: "ok",
      stderr: "",
    });

    const detail = await repo.getWithDetails("r1", "alice");
    expect(detail?.files).toHaveLength(1);
    expect(detail?.files[0].sizeBytes).toBe(5_000_000_000);
    expect(detail?.logs).toHaveLength(1);
    expect(detail?.logs[0].stdout).toBe("ok");
  });

  it("getWithDetails respects ACL (stranger gets null)", async () => {
    const { repo } = await seed();
    await repo.create(baseRun());
    await repo.appendFile({
      runId: "r1",
      name: "a.txt",
      s3Key: "runs/r1/a.txt",
    });
    expect(await repo.getWithDetails("r1", "bob")).toBeNull();
  });

  it("getFile scopes by run + name", async () => {
    const { repo } = await seed();
    await repo.create(baseRun());
    await repo.appendFile({
      runId: "r1",
      name: "out.csv",
      s3Key: "runs/r1/out.csv",
    });
    const f = await repo.getFile("r1", "out.csv");
    expect(f?.s3Key).toBe("runs/r1/out.csv");
    expect(await repo.getFile("r1", "missing.csv")).toBeNull();
  });
});
