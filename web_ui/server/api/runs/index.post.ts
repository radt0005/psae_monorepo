import YAML from "yaml";
import { v7 } from "uuid";
import { useDb } from "~/server/db";
import { makeRunRepo } from "~/server/db/repositories/runs";
import { makePipelineRepo } from "~/server/db/repositories/pipelines";
import { requireUser } from "~/server/utils/requireUser";
import { publishJob } from "~/server/utils/rabbitmq";
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
 * We snapshot the resolved (UUID-only) YAML onto the run, persist a `queued`
 * row, then enqueue to RabbitMQ. The DB write is the source of truth for the
 * UI; the queue hands the job to the scheduler.
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

  // Enqueue for the scheduler. If the broker is down we surface a 502 but the
  // run row remains in `queued` so it can be retried/inspected.
  try {
    await publishJob(user.id, runId, resolvedYaml);
  } catch (e: any) {
    await repo.updateStatus(runId, "failed", {
      error: `Failed to enqueue: ${e?.message}`,
    });
    throw createError({
      statusCode: 502,
      statusMessage: "Could not enqueue run to the scheduler",
    });
  }

  return run;
});
