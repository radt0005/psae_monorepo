import { Zip, ZipPassThrough } from "fflate";
import { useDb } from "~/server/db";
import { makeRunRepo } from "~/server/db/repositories/runs";
import { useS3 } from "~/server/storage";
import { requireUser } from "~/server/utils/requireUser";

/**
 * Stream every output file of a run as a single ZIP (web_ui.md §"Browsing
 * Results" — "Download all", replacing the old per-file download loop).
 *
 * Files are pulled from S3 and fed through fflate's streaming Zip chunk by
 * chunk, so even multi-GB geospatial outputs never fully buffer in memory.
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const id = getRouterParam(event, "id");
  if (!id) throw createError({ statusCode: 400, statusMessage: "Missing id" });

  const repo = makeRunRepo(useDb());
  const run = await repo.getWithDetails(id, user.id);
  if (!run) throw createError({ statusCode: 404, statusMessage: "Not found" });
  if (run.files.length === 0) {
    throw createError({ statusCode: 404, statusMessage: "No files to download" });
  }

  const s3 = useS3();
  // Disambiguate duplicate filenames produced by different blocks.
  const seen = new Map<string, number>();
  const entryName = (name: string) => {
    const n = seen.get(name) ?? 0;
    seen.set(name, n + 1);
    if (n === 0) return name;
    const dot = name.lastIndexOf(".");
    return dot > 0
      ? `${name.slice(0, dot)}_${n}${name.slice(dot)}`
      : `${name}_${n}`;
  };

  const files = run.files.map((f) => ({ name: entryName(f.name), key: f.s3Key }));

  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      const zip = new Zip((err, chunk, final) => {
        if (err) {
          controller.error(err);
          return;
        }
        controller.enqueue(chunk);
        if (final) controller.close();
      });

      (async () => {
        for (const f of files) {
          const entry = new ZipPassThrough(f.name);
          zip.add(entry);
          const reader = s3.file(f.key).stream().getReader();
          // eslint-disable-next-line no-constant-condition
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            entry.push(value, false);
          }
          entry.push(new Uint8Array(0), true);
        }
        zip.end();
      })().catch((e) => controller.error(e));
    },
  });

  setHeader(event, "Content-Type", "application/zip");
  setHeader(
    event,
    "Content-Disposition",
    `attachment; filename="run-${id}.zip"`,
  );
  return sendStream(event, body as any);
});
