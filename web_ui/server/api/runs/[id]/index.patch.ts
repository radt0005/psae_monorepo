import { useDb } from "~/server/db";
import { makeRunRepo } from "~/server/db/repositories/runs";

/**
 * Scheduler/worker callback for run progress (worker.md §"Result reporting").
 * Authenticated by a shared bearer secret, NOT a user session — the caller is
 * the scheduler, not a browser.
 *
 * Body (all fields optional, applied in order):
 *   status: "running" | "succeeded" | "failed" | "canceled"
 *   error:  string                         (failure reason)
 *   files:  [{ name, s3Key, blockId?, mimeType?, sizeBytes? }]
 *   logs:   [{ blockId?, stdout?, stderr? }]
 */
export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig();
  // Fall back to the live env var: runtimeConfig defaults are baked at build
  // time (empty in the container image), so read the runtime value directly —
  // same pattern as server/db/index.ts for DATABASE_URL.
  const expected =
    (config.workerCallbackSecret as string) ||
    process.env.WORKER_CALLBACK_SECRET ||
    "";
  if (!expected) {
    throw createError({
      statusCode: 503,
      statusMessage: "Worker callback secret not configured",
    });
  }
  const auth = getRequestHeader(event, "authorization") ?? "";
  const token = auth.replace(/^Bearer\s+/i, "");
  if (token !== expected) {
    throw createError({ statusCode: 401, statusMessage: "Unauthorized" });
  }

  const id = getRouterParam(event, "id");
  if (!id) throw createError({ statusCode: 400, statusMessage: "Missing id" });

  const body = await readBody<{
    status?: "queued" | "running" | "succeeded" | "failed" | "canceled";
    error?: string | null;
    files?: Array<{
      name: string;
      s3Key: string;
      blockId?: string | null;
      mimeType?: string | null;
      sizeBytes?: number | null;
    }>;
    logs?: Array<{
      blockId?: string | null;
      stdout?: string | null;
      stderr?: string | null;
    }>;
  }>(event);

  const repo = makeRunRepo(useDb());

  // The run must exist; the callback never creates runs.
  const existing = await repo.getById(id);
  if (!existing) {
    throw createError({ statusCode: 404, statusMessage: "Run not found" });
  }

  for (const file of body?.files ?? []) {
    if (!file?.name || !file?.s3Key) continue;
    await repo.appendFile({
      runId: id,
      name: file.name,
      s3Key: file.s3Key,
      blockId: file.blockId ?? null,
      mimeType: file.mimeType ?? null,
      sizeBytes: file.sizeBytes ?? null,
    });
  }

  for (const log of body?.logs ?? []) {
    await repo.appendLog({
      runId: id,
      blockId: log.blockId ?? null,
      stdout: log.stdout ?? null,
      stderr: log.stderr ?? null,
    });
  }

  if (body?.status) {
    await repo.updateStatus(id, body.status, {
      error: body.error ?? undefined,
    });
  }

  return { ok: true };
});
