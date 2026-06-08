import { useDb } from "~/server/db";
import { makeRunRepo } from "~/server/db/repositories/runs";
import { useS3 } from "~/server/storage";
import { requireUser } from "~/server/utils/requireUser";

/**
 * Presigned download for a single result file. ACL is enforced on the run, so
 * a shared/public run's files are reachable by viewers. S3 handles HTTP Range
 * itself, so large geospatial outputs stream without us proxying bytes.
 *
 * Returns `{ url }` by default; with `?redirect=1` issues a 302 to the
 * presigned URL (handy for plain <a> downloads).
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const id = getRouterParam(event, "id");
  const name = getRouterParam(event, "name");
  if (!id || !name) {
    throw createError({ statusCode: 400, statusMessage: "Missing id or name" });
  }

  const repo = makeRunRepo(useDb());
  // ACL check on the run first.
  const run = await repo.getByIdForUser(id, user.id);
  if (!run) throw createError({ statusCode: 404, statusMessage: "Not found" });

  const file = await repo.getFile(id, decodeURIComponent(name));
  if (!file) {
    throw createError({ statusCode: 404, statusMessage: "File not found" });
  }

  const url = useS3().presign(file.s3Key, {
    method: "GET",
    expiresIn: 60 * 10, // 10 minutes
  });

  if (getQuery(event).redirect) {
    return sendRedirect(event, url, 302);
  }
  return { url, name: file.name, mimeType: file.mimeType, sizeBytes: file.sizeBytes };
});
