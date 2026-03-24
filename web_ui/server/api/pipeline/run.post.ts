import { exec } from "node:child_process";
import { validate } from "uuid";
import path from "node:path";
import fs from "node:fs/promises"
import { v4 as uuidv4 } from 'uuid'
import amqplib from 'amqplib'

const RABBITMQ_URL = 'amqp://localhost'  // adjust as needed
const QUEUE_NAME = 'user_submissions'

export default defineEventHandler(
    async (event) => {
        const body = await readBody(event);
        const id = body.id;

        const run_id = uuidv4()

        //
        const examplePipeline = `
id: 0195d877-e383-7229-b601-1be1bbc558f6
version: 0.0.1
blocks:
  - id: data-public-fia
    name: data-public-fia
    input: []
    args: {}
  - id: 0195d875-01b0-7229-b600-f1c189c941ce
    name: duckdb-sql-query
    input:
      - data-public-fia
    args:
      query: |-
        SELECT SPCD, DIA, DRYBIO_AG 
        FROM read_csv(?) 
        WHERE INVYR < 2022 
        LIMIT 300;
      path: ""
      bindings: []
      bind_path: ""
  - id: 0195d875-5e62-7229-b600-fe00577d6fa2
    name: random-forest-fit
    input:
      - 0195d875-01b0-7229-b600-f1c189c941ce
    args:
      data_path: ""
      params: {}
      target: DRYBIO_AG
      save_path: ""
  - id: 0195d875-f238-7229-b601-02838f7897b1
    name: random-forest-predict
    input:
      - 0195d875-5e62-7229-b600-fe00577d6fa2
      - 0195d876-4667-7229-b601-0e8f340d1a24
    args:
      data_path: ""
      model_path: ""
  - id: 0195d876-4667-7229-b601-0e8f340d1a24
    name: duckdb-sql-query
    input:
      - data-public-fia
    args:
      query: |-
        SELECT SPCD, DIA
        FROM read_csv(?) 
        WHERE INVYR = 2022 
        LIMIT 100;
      path: ""
      bindings: []
      bind_path: ""
data: []`

        const message = {
            user_id: id,
            run_id: run_id,
            content: examplePipeline
            //body.pipeline as string,
          }

        console.log(message)
        // verify that this is a UUID
        if(validate(id)){
            
            //const workDir = `/home/krbundy/.psae/runs/${id}`;
            //fs.mkdir(workDir)
            //const pipelinePath = path.join(workDir, "pipeline.yaml")
            //const currentDir = path.resolve(".");

            //const out_res = await fs.writeFile(pipelinePath, body.pipeline)
            const conn = await amqplib.connect(RABBITMQ_URL)
            const channel = await conn.createChannel()

            await channel.assertQueue(QUEUE_NAME, { durable: true })

            const success = channel.sendToQueue(
                QUEUE_NAME,
                Buffer.from(JSON.stringify(message)),
                { persistent: true }
            )
            console.log(message)
            await channel.close()
            await conn.close()

            
                        
            return {
                success,
                id: run_id,
              }


        } else {
            return {
                id: run_id,
                error: "Invalid UUID provided"
            }
        }

        
    }
)