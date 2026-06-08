import { useDb } from "~/server/db";
import { makeRunRepo } from "~/server/db/repositories/runs";
import { requireUser } from "~/server/utils/requireUser";

/**
 * Owner-only run update. Currently just visibility (private | shared | public)
 * for the share UI on the run detail page.
 * Body: { visibility: "private" | "shared" | "public" }
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const id = getRouterParam(event, "id");
  if (!id) throw createError({ statusCode: 400, statusMessage: "Missing id" });
  const body = await readBody<{
    visibility?: "private" | "shared" | "public";
  }>(event);
  if (!body?.visibility) {
    throw createError({ statusCode: 400, statusMessage: "visibility required" });
  }

  const repo = makeRunRepo(useDb());
  const row = await repo.setVisibility(id, user.id, body.visibility);
  if (!row) {
    throw createError({ statusCode: 403, statusMessage: "Not your run" });
  }
  return row;
});
