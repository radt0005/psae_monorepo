import YAML from "yaml";
import { useDb } from "~/server/db";
import { makeBlockRepo } from "~/server/db/repositories/blocks";
import { requireUser } from "~/server/utils/requireUser";
import type { BlockManifest } from "~/utils/types";

/**
 * Upsert a block manifest. Accepts JSON `{label, manifest}` or a raw YAML
 * body for convenience when uploading from a CLI. Admin-only.
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  if (user.role !== "admin") {
    throw createError({ statusCode: 403, statusMessage: "Admin only" });
  }

  const contentType = getHeader(event, "content-type") ?? "";
  let label: string;
  let manifest: BlockManifest;

  if (contentType.includes("yaml")) {
    const raw = await readRawBody(event);
    if (!raw) {
      throw createError({ statusCode: 400, statusMessage: "Empty body" });
    }
    manifest = YAML.parse(typeof raw === "string" ? raw : raw.toString());
    label = manifest.id;
  } else {
    const body = await readBody<{ label?: string; manifest: BlockManifest }>(
      event,
    );
    if (!body?.manifest) {
      throw createError({
        statusCode: 400,
        statusMessage: "Missing `manifest` in body",
      });
    }
    manifest = body.manifest;
    label = body.label ?? manifest.id;
  }

  if (!manifest.id || !manifest.version) {
    throw createError({
      statusCode: 400,
      statusMessage: "Manifest missing required `id` or `version`",
    });
  }

  const repo = makeBlockRepo(useDb());
  const row = await repo.upsert({ label, manifest });
  return row;
});
