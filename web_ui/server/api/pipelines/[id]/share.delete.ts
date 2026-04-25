import { useDb } from "~/server/db";
import { makePipelineRepo } from "~/server/db/repositories/pipelines";
import { requireUser } from "~/server/utils/requireUser";

export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const id = getRouterParam(event, "id");
  if (!id) throw createError({ statusCode: 400, statusMessage: "Missing id" });
  const userId = (getQuery(event).userId as string | undefined) ?? "";
  if (!userId) {
    throw createError({
      statusCode: 400,
      statusMessage: "userId query param required",
    });
  }
  const repo = makePipelineRepo(useDb());
  const ok = await repo.unshare(id, user.id, userId);
  if (!ok) {
    throw createError({
      statusCode: 403,
      statusMessage: "Not your pipeline",
    });
  }
  return { ok: true };
});
