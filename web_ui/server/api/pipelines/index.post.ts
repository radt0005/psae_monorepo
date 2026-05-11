import YAML from "yaml";
import { v7 } from "uuid";
import { useDb } from "~/server/db";
import { makePipelineRepo } from "~/server/db/repositories/pipelines";
import { requireUser } from "~/server/utils/requireUser";
import { validatePipeline } from "~/utils/validate";
import { resolveShortCodes } from "~/utils/short_codes";
import type { Pipeline } from "~/utils/types";

export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const body = await readBody<{
    name: string;
    description?: string;
    version?: string;
    yaml: string;
    visibility?: "private" | "shared" | "public";
  }>(event);

  if (!body?.yaml || !body?.name) {
    throw createError({
      statusCode: 400,
      statusMessage: "name and yaml are required",
    });
  }

  // Parse the uploaded YAML.
  let parsed: any;
  try {
    parsed = YAML.parse(body.yaml);
  } catch (e: any) {
    throw createError({
      statusCode: 400,
      statusMessage: `Invalid YAML: ${e?.message}`,
    });
  }

  // Resolve any short codes against fresh UUIDv7s (spec/pipeline.md §6.7).
  // No lockfile is persisted on the server -- each upload mints fresh
  // UUIDs.  Local and cloud caches are intentionally independent.
  let resolved: Pipeline;
  try {
    resolved = resolveShortCodes(parsed as Pipeline);
  } catch (e: any) {
    throw createError({
      statusCode: 400,
      statusMessage: `Pipeline invalid: ${e?.message}`,
    });
  }

  const result = validatePipeline(resolved);
  if (!result.ok) {
    throw createError({
      statusCode: 400,
      statusMessage: `Pipeline invalid: ${result.errors.join("; ")}`,
    });
  }

  // Persist the resolved (UUID-only) YAML so that subsequent loads
  // produce stable invocation ids -- the cloud cache property mirrors
  // the lockfile property the CLI provides locally.
  const resolvedYaml = YAML.stringify(resolved);

  const repo = makePipelineRepo(useDb());
  const created = await repo.create({
    id: resolved.id ?? v7(),
    ownerId: user.id,
    name: body.name,
    description: body.description ?? resolved.description,
    version: body.version ?? resolved.version ?? "0.1.0",
    yaml: resolvedYaml,
    visibility: body.visibility ?? "private",
  });
  return created;
});
