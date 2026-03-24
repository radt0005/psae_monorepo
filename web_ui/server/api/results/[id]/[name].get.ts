import path from "node:path";
import fs from "node:fs/promises";
import parse from "csv-simple-parser";
import type { Options } from "csv-simple-parser";



export default defineEventHandler(

    async (event) => {

        // this needs to be updated to prevent path traversal attacks
        const name = getRouterParam(event, "name") || "test";
        const id = getRouterParam(event, "id") || "example.csv";

        const basePath = "/home/krbundy/.psae/runs";

        const filePath = path.join(basePath, id, name );

        const data = await fs.readFile(filePath);
        let response_data: any;

        // this limits the data to CSV and JSON/GeoJSON files
        try {

            // based on the file type, read the correct way.
            if(filePath.includes(".csv")){
                // load CSV file into an object
                //const options: Options = {}
                response_data = parse(data.toString(), {
                    header: true
                });
            } else {
                // otherwise assume it's a CSV or 
                response_data = JSON.parse(data.toString())
            }
            return response_data
        } catch(e){
            console.error(e);
            return {
                error: "File not a valid type"
            }
        }

    }
)