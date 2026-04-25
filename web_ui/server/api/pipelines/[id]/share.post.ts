import { useDb } from "~/server/db";
import { makePipelineRepo } from "~/server/db/repositories/pipelines";
import { requireUser } from "~/server/utils/requireUser";
import { eq } from "drizzle-orm";
import { users } from "~/server/db/schema/users";

/**
 * Body: { email: string, permission?: "view" | "edit" }
 * Looks up the target user by email then writes a share row. Returns
 * 404 if the email doesn't resolve to a user.
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const id = getRouterParam(event, "id");
  if (!id) throw createError({ statusCode: 400, statusMessage: "Missing id" });
  const body = await readBody<{
    email: string;
    permission?: "view" | "edit";
  }>(event);
  if (!body?.email) {
    throw createError({ statusCode: 400, statusMessage: "email required" });
  }

  const db = useDb();
  const target = (
    await db.select().from(users).where(eq(users.email, body.email)).limit(1)
  )[0];
  if (!target) {
    throw createError({
      statusCode: 404,
      statusMessage: "No user with that email",
    });
  }

  const repo = makePipelineRepo(db);
  const ok = await repo.share(id, user.id, target.id, body.permission ?? "view");
  if (!ok) {
    throw createError({
      statusCode: 403,
      statusMessage: "Not your pipeline",
    });
  }
  return { ok: true };
});
