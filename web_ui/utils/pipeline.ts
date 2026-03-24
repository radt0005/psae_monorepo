import type { Node, Edge } from "@vue-flow/core";
import { v7 } from 'uuid';
import YAML from "yaml";
import type { Pipeline, Block, BlockBuilder, PipelineMetadata, ArgKeyValueStore } from "./types"


const { addNode, connect} = useFlow();

export class PipelineBuilder {
    block_count: number = 0;
    blocks: BlockBuilder[] = []
    data: BlockBuilder[] = []
    metadata: PipelineMetadata = {
        id: v7(),
        version: "0.0.1",
        name: undefined,
        description: undefined
    }



    constructor() {}

    add_block_from_node(node: Node){
        const node_parsed = {
            id: node.id,
            input: [],
            output: [],
            name: node.data.name,
            args: node.data.args || {}
        } as BlockBuilder;

        if( node.data.is_data) {
            this.data.push(node_parsed);
        } else {
            this.blocks.push(node_parsed);

        }

    }

    parse_edge(edge: Edge) {
        const input_id = edge.source;
        const output_id = edge.target;

        for(let block of this.blocks) {
            if(block.id === output_id) {
                block.input.push(input_id)
            }
        }
    }

    set_name(name: string) {
        this.metadata.name = name;
    }

    set_description(desc: string) {
        this.metadata.description = desc;
    }

    export_pipeline() {
        const blocks = this.blocks.map((b) => block_builder_to_block(b));
        const data = this.data.map((d) => block_builder_to_block(d));

        return {
            ...this.metadata,
            blocks: blocks,
            data: data, 
        }
    }
}


function block_builder_to_block(b: BlockBuilder) {
    return {
        id: b.id, 
        name: b.name,
        input: b.input || [],
        args: b.args
    }
}
