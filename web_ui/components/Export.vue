<script setup lang="ts">
import { PipelineBuilder } from '~/utils/pipeline';
import YAML from "yaml";
import { v7 } from "uuid"
    const {nodes, edges } = useFlow();
    const pipelineText = ref<string[]>([]);
    const emit = defineEmits(["close"]);
    const pb = usePB();
    

    
    const export_pipeline = () => {
        pipelineText.value = [];
        const pipeline_parser = new PipelineBuilder();
        for(let node of nodes) {
            pipeline_parser.add_block_from_node(node);
        }

        for(let edge of edges) {
            pipeline_parser.parse_edge(edge);
        }


        const pipeline = pipeline_parser.export_pipeline();
        const base_text = YAML.stringify(pipeline);
        base_text.split("\n").forEach(
            (piece) => pipelineText.value.push(piece)
        )
        console.log(pipeline);
    }

    const runPipeline = async () => {
        const yamlContent = pipelineText.value.join("\n"); // Reconstruct YAML
        const authData = await pb.value.collection('users').authRefresh();
        const userId = authData.record.id
        //const userId = pb.value.authStore.record?.id;

        // write to pocketbase (which initializes the run)
        pb.value.collection("runs").create<Run>(
            {
                userId: userId,
                runId: v7(),
                content: yamlContent,
                status: "pending"
            }
        )
        .then((response) => {
            // navigate only when the submission is complete
            console.log(response);
            navigateTo(`/results/${response.runId || "test"}`);
        })


            /*
        const response = await $fetch("/api/pipeline/save", {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: {
                pipeline: yamlContent
            },
        });

        if(response) {
            setTimeout(
                () => {
                },
                1000
            )
        }
            */

    }



    const downloadYAML = () => {
    const yamlContent = pipelineText.value.join("\n"); // Reconstruct YAML
    const blob = new Blob([yamlContent], { type: "text/yaml" });
    const url = URL.createObjectURL(blob);
    
    const link = document.createElement("a");
    link.href = url;
    link.download = "pipeline.yaml";
    document.body.appendChild(link);
    link.click();
    
    // Cleanup
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
};


    const handleClose = () => emit("close")
</script>


<template>
    <UCard class="overflow-y-auto">
        <template #header>
            <p>Export as YAML</p>
        </template>
        <div >

            <div v-for="text in pipelineText" >
                
                <pre>{{ text }}</pre>
            </div>
        </div>

        <template #footer>
            <UButtonGroup>
                <UButton class="p-3" @click="export_pipeline">Export as YAML</UButton>
                <UButton class="p-3" @click="handleClose">Close</UButton>
                <UButton class="p-3" @click="downloadYAML" v-if="pipelineText.length > 0">Download</UButton>
                <UButton class="p-3" @click="runPipeline" v-if="pipelineText.length > 0">Run it!</UButton>
            </UButtonGroup>

        </template>

    </UCard>

</template>
<style scoped>
pre {
    white-space: pre-wrap; /* Ensures text wraps within the pre block */
    word-break: break-word; /* Breaks long words to prevent overflow */
}
</style>