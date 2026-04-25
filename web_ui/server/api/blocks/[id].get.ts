import { useDb } from "~/server/db";
import { makeBlockRepo } from "~/server/db/repositories/blocks";
import type { BlockManifest } from "~/utils/types";

/** Returns the full BlockManifest for a single block, looked up by name. */
export default defineEventHandler(async (event) => {
  const id = getRouterParam(event, "id");
  if (!id) {
    throw createError({ statusCode: 400, statusMessage: "Missing block id" });
  }

  const repo = makeBlockRepo(useDb());
  const row = await repo.getByName(id);
  if (!row) {
    throw createError({ statusCode: 404, statusMessage: "Block not found" });
  }
  return row.manifest as BlockManifest;
});
