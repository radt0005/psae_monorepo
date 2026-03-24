import fs from "node:fs/promises";
import path from "node:path";
import type { BlockList } from "../../../utils/types";
import { fileURLToPath } from 'url';
//import { useServerPocketbase } from "~/server/utils/useServerPocketbase";


//const __dirname = "~/GitHub/psaec" //path.dirname(fileURLToPath(import.meta.url));
const pb = useServerPocketbase();

export default defineEventHandler(
    async (_event) => {


        const pbData = await pb.collection("blocks").getFullList()

        //const defaultPath = "./block-index.json" //path.join(__dirname, "block-index.json");
        //const content = await fs.readFile(defaultPath);
        const data: BlockList ={
            blocks: pbData.map(
                (item) => {
                    return {
                        name: item.name,
                        label: item.label,
                        type: "block"
                    }
                }
            )
        } 
        //JSON.parse(content.toString())

        return data
    }
)