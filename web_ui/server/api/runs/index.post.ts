import YAML from "yaml";
import { v7 } from "uuid";
import { useDb } from "~/server/db";
import { makeRunRepo } from "~/server/db/repositories/runs";
import { makePipelineRepo } from "~/server/db/repositories/pipelines";
import { requireUser } from "~/server/utils/requireUser";
import { validatePipeline } from "~/utils/validate";
import { resolveShortCodes } from "~/utils/short_codes";
import type { Pipeline } from "~/utils/types";

/**
 * Submit a pipeline for execution.
 *
 * Body is one of:
 *   { pipelineId: string }   — run a saved pipeline (ACL-checked)
 *   { yaml: string, name? }  — run ad-hoc YAML
 *
 * The resolved (UUID-only) YAML is snapshotted onto a `queued` run row.
 * The Go scheduler polls for queued rows via the Postgres outbox pattern
 * (spec/worker.md §Communication) — no RabbitMQ publish needed here.
 */
export default defineEventHandler(async (event) => {
  const user = await requireUser(event);
  const body = await readBody<{
    pipelineId?: string;
    yaml?: string;
    name?: string;
  }>(event);

  const db = useDb();
  let yaml: string;
  let name: string | null = body?.name ?? null;
  let pipelineId: string | null = null;

  if (body?.pipelineId) {
    // Run a saved pipeline — load through the ACL.
    const pipeRepo = makePipelineRepo(db);
    const pipeline = await pipeRepo.getByIdForUser(body.pipelineId, user.id);
    if (!pipeline) {
      throw createError({ statusCode: 404, statusMessage: "Pipeline not found" });
    }
    yaml = pipeline.yaml;
    name = name ?? pipeline.name;
    pipelineId = pipeline.id;
  } else if (body?.yaml) {
    yaml = body.yaml;
  } else {
    throw createError({
      statusCode: 400,
      statusMessage: "pipelineId or yaml is required",
    });
  }

  // Parse + validate the YAML before we accept it.
  let parsed: unknown;
  try {
    parsed = YAML.parse(yaml);
  } catch (e: any) {
    throw createError({
      statusCode: 400,
      statusMessage: `Invalid YAML: ${e?.message}`,
    });
  }

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

  const resolvedYaml = YAML.stringify(resolved);
  const runId = resolved.id ?? v7();

  const repo = makeRunRepo(db);
  const run = await repo.create({
    id: runId,
    pipelineId,
    ownerId: user.id,
    name: name ?? resolved.name ?? null,
    yaml: resolvedYaml,
    status: "queued",
    visibility: "private",
  });

  return run;
});
