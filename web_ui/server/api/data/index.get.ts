import { useDb } from "~/server/db";
import { makeDataFileRepo } from "~/server/db/repositories/dataFiles";
import { requireUser } from "~/server/utils/requireUser";

export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const scope = (getQuery(event).scope as string | undefined) ?? "mine";
  const repo = makeDataFileRepo(useDb());
  if (scope === "shared") return { items: await repo.listSharedWith(user.id) };
  if (scope === "public") return { items: await repo.listPublic() };
  return { items: await repo.listOwnedBy(user.id) };
});
