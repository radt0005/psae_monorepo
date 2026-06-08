import { useDb } from "~/server/db";
import { makeRunRepo } from "~/server/db/repositories/runs";
import { requireUser } from "~/server/utils/requireUser";

/**
 * List runs:
 *   GET /api/runs?scope=mine    (default)
 *   GET /api/runs?scope=shared
 *   GET /api/runs?scope=public
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const scope = (getQuery(event).scope as string | undefined) ?? "mine";

  const repo = makeRunRepo(useDb());
  if (scope === "shared") return { runs: await repo.listSharedWith(user.id) };
  if (scope === "public") return { runs: await repo.listPublic() };
  return { runs: await repo.listOwnedBy(user.id) };
});
