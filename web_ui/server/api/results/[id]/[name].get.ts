import path from "node:path";
import fs from "node:fs/promises";
import parse from "csv-simple-parser";

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
  const name = getRouterParam(event, "name");
  if (!id || !/^[a-zA-Z0-9_-]+$/.test(id)) {
    throw createError({ statusCode: 400, statusMessage: "Invalid run id" });
  }
  if (!name || name.includes("/") || name.includes("..")) {
    throw createError({ statusCode: 400, statusMessage: "Invalid file name" });
  }

  const filePath = path.join(basePath, id, name);
  const resolved = path.resolve(filePath);
  if (!resolved.startsWith(path.resolve(basePath) + path.sep)) {
    throw createError({ statusCode: 400, statusMessage: "Invalid path" });
  }

  try {
    const data = await fs.readFile(resolved);
    if (resolved.endsWith(".csv")) {
      return parse(data.toString(), { header: true });
    }
    return JSON.parse(data.toString());
  } catch (e) {
    console.error(e);
    throw createError({ statusCode: 415, statusMessage: "Unsupported file" });
  }
});
