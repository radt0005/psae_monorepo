import { v7 } from "uuid";
import { useS3 } from "~/server/storage";
import { requireUser } from "~/server/utils/requireUser";

/**
 * Begin an upload. Returns a presigned PUT URL the browser uses to stream
 * the file directly to S3/MinIO. We don't pre-allocate the DB row — the
 * client posts to /api/uploads/data/complete after the upload finishes.
 *
 * For very large files (>100 MB) the client should fall back to multipart;
 * Bun's S3 supports it but we'll add the multipart endpoints in a follow-up
 * if/when needed.
 *
 * Body: { name: string, mimeType: string, sizeBytes: number }
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const body = await readBody<{
    name: string;
    mimeType: string;
    sizeBytes: number;
  }>(event);
  if (!body?.name || !body?.mimeType) {
    throw createError({
      statusCode: 400,
      statusMessage: "name and mimeType required",
    });
  }
  if (body.sizeBytes <= 0) {
    throw createError({
      statusCode: 400,
      statusMessage: "sizeBytes must be > 0",
    });
  }

  const dataFileId = v7();
  const s3Key = `data/${user.id}/${dataFileId}/${body.name}`;
  const s3 = useS3();
  const url = s3.presign(s3Key, {
    method: "PUT",
    type: body.mimeType,
    expiresIn: 60 * 30, // 30 minutes for the upload window
  });

  return {
    dataFileId,
    s3Key,
    uploadUrl: url,
    method: "PUT",
    headers: { "Content-Type": body.mimeType },
    expiresIn: 60 * 30,
  };
});
