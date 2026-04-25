import { useDb } from "~/server/db";
import { makePipelineRepo } from "~/server/db/repositories/pipelines";
import { requireUser } from "~/server/utils/requireUser";

export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const id = getRouterParam(event, "id");
  if (!id) {
    throw createError({ statusCode: 400, statusMessage: "Missing id" });
  }
  const body = await readBody<{
    name?: string;
    description?: string;
    version?: string;
    yaml?: string;
    visibility?: "private" | "shared" | "public";
  }>(event);

  const repo = makePipelineRepo(useDb());
  const updated = await repo.update(id, user.id, body);
  if (!updated) {
    throw createError({
      statusCode: 403,
      statusMessage: "Not your pipeline",
    });
  }
  return updated;
});
