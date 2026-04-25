import { useDb } from "~/server/db";
import { makePipelineRepo } from "~/server/db/repositories/pipelines";
import { requireUser } from "~/server/utils/requireUser";

/**
 * List the user's pipelines, plus optional public/shared tabs:
 *   GET /api/pipelines              → mine
 *   GET /api/pipelines?scope=mine
 *   GET /api/pipelines?scope=shared
 *   GET /api/pipelines?scope=public
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const scope = (getQuery(event).scope as string | undefined) ?? "mine";

  const repo = makePipelineRepo(useDb());
  if (scope === "shared") return { pipelines: await repo.listSharedWith(user.id) };
  if (scope === "public") return { pipelines: await repo.listPublic() };
  return { pipelines: await repo.listOwnedBy(user.id) };
});
