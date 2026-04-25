import path from "node:path";
import fs from "node:fs/promises";

export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig();
  const basePath = config.runsDir as string;
  if (!basePath) {
    throw createError({
      statusCode: 500,
      statusMessage: "RUNS_DIR is not configured",
    });
  }

  const id = getRouterParam(event, "id");
  if (!id || !/^[a-zA-Z0-9_-]+$/.test(id)) {
    throw createError({ statusCode: 400, statusMessage: "Invalid run id" });
  }

  const targetDirectory = path.join(basePath, id);
  const resolved = path.resolve(targetDirectory);
  if (!resolved.startsWith(path.resolve(basePath) + path.sep)) {
    throw createError({ statusCode: 400, statusMessage: "Invalid path" });
  }

  const files = await fs.readdir(resolved);
  return { files };
});
