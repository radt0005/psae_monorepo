import path from "node:path";
import fs from "node:fs/promises";

export default defineEventHandler(
    async (event) => {
        const id = getRouterParam(event, "id") || "test";
        const basePath = `/home/krbundy/.psae/runs/`
        const targetDirectory = path.join(basePath, id);
        const files = await fs.readdir(targetDirectory);

        return {
            files: files
        }
    }
)