import { useDb } from "~/server/db";
import { makeDataFileRepo } from "~/server/db/repositories/dataFiles";
import { useS3 } from "~/server/storage";
import { requireUser } from "~/server/utils/requireUser";

/**
 * Returns metadata + a presigned download URL the browser can hit
 * directly. Range requests are handled by S3 itself, so large files
 * stream cleanly without us proxying bytes.
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const id = getRouterParam(event, "id");
  if (!id) {
    throw createError({ statusCode: 400, statusMessage: "Missing id" });
  }
  const repo = makeDataFileRepo(useDb());
  const row = await repo.getByIdForUser(id, user.id);
  if (!row) {
    throw createError({ statusCode: 404, statusMessage: "Not found" });
  }
  const s3 = useS3();
  const downloadUrl = s3.presign(row.s3Key, {
    method: "GET",
    expiresIn: 60 * 10, // 10 minutes
  });
  return { ...row, downloadUrl };
});
