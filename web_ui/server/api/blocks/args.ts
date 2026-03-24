import fs from "node:fs/promises"

export default defineEventHandler(
    async (event) => {
        const params = getQuery(event);
        const name = params["name"]?.toString();

        // load the data based on the name
        const slug = name?.replaceAll("::", "-").toLowerCase()
        const filename = `./blocks/${slug}.json`;
        const data = await fs.readFile(filename);
        const data_str = data.toString()


        return JSON.parse(data_str)
    }
)