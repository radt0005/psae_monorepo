import { useDb } from "~/server/db";
import { makeDataFileRepo } from "~/server/db/repositories/dataFiles";
import { requireUser } from "~/server/utils/requireUser";

/**
 * Finalize an upload. The client posts the metadata + the s3Key/dataFileId
 * it received from /init, after the PUT to S3 succeeds.
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const body = await readBody<{
    dataFileId: string;
    s3Key: string;
    name: string;
    mimeType: string;
    sizeBytes: number;
    visibility?: "private" | "shared" | "public";
  }>(event);
  if (
    !body?.dataFileId ||
    !body?.s3Key ||
    !body?.name ||
    !body?.mimeType ||
    !body?.sizeBytes
  ) {
    throw createError({
      statusCode: 400,
      statusMessage: "Missing fields",
    });
  }

  const repo = makeDataFileRepo(useDb());
  const row = await repo.create({
    id: body.dataFileId,
    ownerId: user.id,
    name: body.name,
    sizeBytes: body.sizeBytes,
    mimeType: body.mimeType,
    s3Key: body.s3Key,
    visibility: body.visibility ?? "private",
  });
  return row;
});
