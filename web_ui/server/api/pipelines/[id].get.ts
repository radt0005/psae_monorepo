import { useDb } from "~/server/db";
import { makePipelineRepo } from "~/server/db/repositories/pipelines";
import { requireUser } from "~/server/utils/requireUser";

export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const id = getRouterParam(event, "id");
  if (!id) {
    throw createError({ statusCode: 400, statusMessage: "Missing id" });
  }
  const repo = makePipelineRepo(useDb());
  const row = await repo.getByIdForUser(id, user.id);
  if (!row) {
    throw createError({ statusCode: 404, statusMessage: "Not found" });
  }
  return row;
});
