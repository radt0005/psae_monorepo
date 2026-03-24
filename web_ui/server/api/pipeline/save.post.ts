import * as YAML from "yaml";
import { v7 } from "uuid";
import path from "node:path";
import fs from "node:fs/promises";
import { existsSync, mkdirSync } from "node:fs";


export default defineEventHandler(
    async (event) => {
        const body = await readBody(event);
        const pipeline_yaml = body.pipeline;
        const pipeline = YAML.parse(pipeline_yaml);

        // extract the pipeline ID
        const id = pipeline.id || v7();

        // create the folder to hold the runs

        // save the pipeline in the folder

        const savePath = `/home/krbundy/.psae/runs/${id}`;
 
        // create directory if it does not exist
        if(!existsSync(savePath)){
            mkdirSync(savePath);
        }

        // save pipeline.yaml to the file

        const pipelinePath = path.join(savePath, "pipeline.yaml");
        const saveResult = await fs.writeFile(pipelinePath, pipeline_yaml)
        console.log(saveResult);
        


        return {
            id: id,    
        }
    }
)