import { eq } from "drizzle-orm";
import { useDb } from "~/server/db";
import { makeRunRepo } from "~/server/db/repositories/runs";
import { requireUser } from "~/server/utils/requireUser";
import { users } from "~/server/db/schema/users";

/**
 * Share a run with another user by email (web_ui.md item 7).
 * Body: { email: string, permission?: "view" | "edit" }
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

  const repo = makeRunRepo(db);
  const ok = await repo.share(id, user.id, target.id, body.permission ?? "view");
  if (!ok) {
    throw createError({ statusCode: 403, statusMessage: "Not your run" });
  }
  return { ok: true };
});
