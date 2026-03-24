import type { Node, Edge } from '@vue-flow/core'
import YAML from 'yaml';
import type { Pipeline, Block } from "~/utils/types";
import { v7 } from "uuid";


export const useFlow = defineStore("flow", () => {
    const nodes = ref<Node[]>([]);
    const edges = ref<Edge[]>([]);
    const nextNodeId = ref<number>(0);

    const connect = (node1: string, node2: string) => {
        const edge = {
            id: `${node1}->${node2}`,
            source: node1,
            target: node2
        } as Edge;

        edges.value.push(edge);
    };


    function yaml_to_nodes(pipeline: string) {
        const pipe = YAML.parse(pipeline) as Pipeline;

        let node_index_by_id = {} as { [key: string]: number };

        // parse Data section into Nodes
        if(pipe.data !== undefined) {
            if(pipe.data.length > 0){

                pipe.data.forEach(
                    (block) => {
                        // create the node from the block information
                        const node = {
                            id: nextNodeId.value.toString(),
                            position: {
                                x: Math.random() * 400,
                                y: 100 + Math.random() * 400
                            },
                            type: "block",
                            data: {
                                id: block.id,
                                label: block.name,
                                name: block.name,
                                args: block.args,
                            }
                        } as Node;
                        nodes.value.push(node);
                        // store the id-index pair for faster parsing of edges.
                        node_index_by_id[block.id] = nextNodeId.value;
                        nextNodeId.value += 1;
                        
                    }
                );
            }
        }


        // parse blocks into nodes
        pipe.blocks.forEach(
            (block) => {
                const node = {
                    id: nextNodeId.value.toString(),
                    position: {
                        x: Math.random() * 400,
                        y: 100 + Math.random() * 400
                    },
                    type: "block",
                    data: {
                        id: block.id,
                        label: block.name,
                        name: block.name,
                        args: block.args,
                    }
                } as Node;

                nodes.value.push(node);
                node_index_by_id[block.id] = nextNodeId.value;
                nextNodeId.value += 1;
            }
        );

        // second pass parsing blocks into edges (now that we have cached the node IDs)
        pipe.blocks.forEach(
            (block) => {
                const from_ids = block.input;
                if(from_ids.length > 0){
                    for(let id of from_ids) {
                        const to_id = block.id;
                        const from_index = node_index_by_id[id];
                        const to_index = node_index_by_id[to_id];
                        const edge_id = `${from_index}->${to_index}`;
                        
                        // only attempt to add edges that are in the pipeline
                        if (to_index !== undefined && from_index !== undefined) {
                            
                            const edge = {
                                id: edge_id,
                                source: from_index.toString(),
                                target: to_index.toString()
                            } as Edge;
                            
                            edges.value.push(edge);
                        }
                    }
                }
            }
        );
    }


        const disconnect = (node1: number, node2: number) => {

        }

        const addNode = (label: string, name: string,  x: number, y: number, args?: any) => {
            const nodeId = v7(); //`${nextNodeId.value}`; 
            nextNodeId.value += 1;

            nodes.value.push(
                {
                    id: nodeId,
                    position: {
                        x, y
                    },
                    type: "block",
                    data: {
                        label,
                        name,
                        args
                    }
                }
            );
            return nodeId
        }

        const removeNode = (id: string) => {
            console.log("Before delete:", nodes.value);
            nodes.value = nodes.value.filter(n => n.id !== id);
            console.log("After delete:", nodes.value);


        };


        const updateNode = (id: string, label: string, name: string, args: any) => {
            //const nodeId = id.toString();

            const found_index = nodes.value.findIndex((value, index) => {
                return value.id === id
            });
            if (found_index >= 0) {

                let oldNode: Node = nodes.value[found_index];
                oldNode.data = {
                    label, 
                    name,
                    args
                };


                nodes.value[found_index] = oldNode;
                return true
            } else {
                return false
            }
        }

        const addNodeFromBlock = (block: Block, x: number, y: number, label: string)  => {
            const nodeId = `${nextNodeId.value}`;
            nextNodeId.value += 1;

            nodes.value.push(
                {
                    id: block.id,
                    position: {
                        x, y
                    },
                    data: {
                        label: label || block.id,
                        name: block.name,
                        args: block.args,
                    },
                    type: "block"
                }
            );
            return nodeId
        }

        const getArgsById = (id: string) => {
            const found_index = nodes.value.findIndex((value, index) => {
                return value.id === id
            });
            if( found_index >= 0) {
                return nodes.value[found_index].data.args
            } else {
                return null
            }
        }

        const getNameById = (id: string) => {
            const found_index = nodes.value.findIndex((value, index) => {
                return value.id === id
            });
            if( found_index >= 0) {
                return nodes.value[found_index].data.name
            } else {
                return null
            }

        }

        return {
            nodes,
            edges,
            connect,
            disconnect,
            addNode,
            addNodeFromBlock,
            removeNode,
            updateNode,
            yaml_to_nodes,
            getArgsById,
            getNameById
        }
    });