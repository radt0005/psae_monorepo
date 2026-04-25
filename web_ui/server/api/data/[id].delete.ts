import { useDb } from "~/server/db";
import { makeDataFileRepo } from "~/server/db/repositories/dataFiles";
import { useS3 } from "~/server/storage";
import { requireUser } from "~/server/utils/requireUser";

export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const id = getRouterParam(event, "id");
  if (!id) {
    throw createError({ statusCode: 400, statusMessage: "Missing id" });
  }
  const repo = makeDataFileRepo(useDb());
  const removed = await repo.delete(id, user.id);
  if (!removed) {
    throw createError({
      statusCode: 403,
      statusMessage: "Not your data file",
    });
  }
  // Best-effort delete from S3 — db row is gone either way.
  try {
    await useS3().delete(removed.s3Key);
  } catch (e) {
    console.warn(`[data delete] S3 delete failed for ${removed.s3Key}`, e);
  }
  return { ok: true };
});
