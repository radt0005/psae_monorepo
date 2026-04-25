import * as YAML from "yaml";
import { v7 } from "uuid";
import path from "node:path";
import fs from "node:fs/promises";
import { existsSync, mkdirSync } from "node:fs";

export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig();
  const runsDir = config.runsDir as string;
  if (!runsDir) {
    throw createError({
      statusCode: 500,
      statusMessage: "RUNS_DIR is not configured",
    });
  }

  const body = await readBody(event);
  const pipeline_yaml = body.pipeline as string;
  const pipeline = YAML.parse(pipeline_yaml);

  const id = pipeline.id || v7();
  const savePath = path.join(runsDir, id);

  if (!existsSync(savePath)) {
    mkdirSync(savePath, { recursive: true });
  }

  const pipelinePath = path.join(savePath, "pipeline.yaml");
  await fs.writeFile(pipelinePath, pipeline_yaml);

  return { id };
});
