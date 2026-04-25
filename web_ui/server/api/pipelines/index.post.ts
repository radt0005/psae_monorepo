import YAML from "yaml";
import { v7 } from "uuid";
import { useDb } from "~/server/db";
import { makePipelineRepo } from "~/server/db/repositories/pipelines";
import { requireUser } from "~/server/utils/requireUser";
import { validatePipeline } from "~/utils/validate";

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

  // Sanity-check the YAML before persisting.
  let parsed: any;
  try {
    parsed = YAML.parse(body.yaml);
  } catch (e: any) {
    throw createError({
      statusCode: 400,
      statusMessage: `Invalid YAML: ${e?.message}`,
    });
  }
  const result = validatePipeline(parsed);
  if (!result.ok) {
    throw createError({
      statusCode: 400,
      statusMessage: `Pipeline invalid: ${result.errors.join("; ")}`,
    });
  }

  const repo = makePipelineRepo(useDb());
  const created = await repo.create({
    id: parsed.id ?? v7(),
    ownerId: user.id,
    name: body.name,
    description: body.description ?? parsed.description,
    version: body.version ?? parsed.version ?? "0.1.0",
    yaml: body.yaml,
    visibility: body.visibility ?? "private",
  });
  return created;
});
