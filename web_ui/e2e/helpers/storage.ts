import { expect, type Page } from "@playwright/test";

/**
 * Upload real bytes to S3/MinIO via the presigned-PUT flow and (optionally)
 * register the data-file row. Returns the s3Key so result-download tests can
 * point a run file at an object that genuinely exists in the bucket.
 */
export async function uploadObject(
  page: Page,
  opts: {
    name: string;
    mimeType: string;
    body: string | Buffer;
    register?: boolean; // also call /complete to create the data_files row
    visibility?: "private" | "shared" | "public";
  },
): Promise<{ dataFileId: string; s3Key: string }> {
  const bytes = typeof opts.body === "string" ? Buffer.from(opts.body) : opts.body;

  const initRes = await page.request.post("/api/uploads/data/init", {
    data: { name: opts.name, mimeType: opts.mimeType, sizeBytes: bytes.length },
  });
  expect(initRes.ok(), `init upload: ${initRes.status()}`).toBeTruthy();
  const init = await initRes.json();

  // PUT directly to the presigned URL. Use a bare request context detail:
  // page.request honours absolute URLs, so this hits MinIO, not the app.
  const putRes = await page.request.put(init.uploadUrl, {
    data: bytes,
    headers: { "Content-Type": opts.mimeType },
  });
  expect(putRes.ok(), `s3 PUT: ${putRes.status()} ${await putRes.text()}`).toBeTruthy();

  if (opts.register) {
    const completeRes = await page.request.post("/api/uploads/data/complete", {
      data: {
        dataFileId: init.dataFileId,
        s3Key: init.s3Key,
        name: opts.name,
        mimeType: opts.mimeType,
        sizeBytes: bytes.length,
        visibility: opts.visibility ?? "private",
      },
    });
    expect(completeRes.ok(), `complete upload: ${completeRes.status()}`).toBeTruthy();
  }

  return { dataFileId: init.dataFileId, s3Key: init.s3Key };
}
