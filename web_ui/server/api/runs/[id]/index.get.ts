import { useDb } from "~/server/db";
import { makeRunRepo } from "~/server/db/repositories/runs";
import { requireUser } from "~/server/utils/requireUser";

/**
 * Single run with its output files + logs (ACL-checked). Files carry their
 * S3 key but not a download URL — the browser fetches that per-file from
 * `GET /api/runs/:id/files/:name` so URLs aren't minted until needed.
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const id = getRouterParam(event, "id");
  if (!id) throw createError({ statusCode: 400, statusMessage: "Missing id" });

  const repo = makeRunRepo(useDb());
  const run = await repo.getWithDetails(id, user.id);
  if (!run) throw createError({ statusCode: 404, statusMessage: "Not found" });
  return run;
});
