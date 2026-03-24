import type { Node, Edge } from "@vue-flow/core";


export type Block = {
    id: string, 
    name: string,
    input: string[], 
    output: string | undefined,
    args: ArgKeyValueStore
}

export type Pipeline = {
    name: string,
    version: string,
    description: string | undefined
    data: Block[]
    blocks: Block[]

}



export type ArgKeyValueStore = {
    [key: string]: any
}

export type NodesAndEdges = {
    nodes: Node[],
    edges: Edge[]
}

export type BlockBuilder = {
    id: string, 
    input: string[],
    name: string,
    args: ArgKeyValueStore
}


export type DataBuilder = {
    id: string, 
    input: string | undefined,
    output: string | undefined,
    args: ArgKeyValueStore
}

export type PipelineMetadata = {
    id: string
    version: string
    name: string | undefined
    description: string | undefined
}

export type BlockListItem = {
    name: string,
    label: string, 
    type: string
}

export type BlockList = {
    blocks: BlockListItem[]
}

export type Run = {
    runId: string,
    userId: string, 
    content: string,
    status: "pending" | "error" | "completed"
}