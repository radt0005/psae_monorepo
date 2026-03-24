export const usePipelineStore = defineStore("pipeline", () => {

    const pipelineText = ref("");

    return {
        text: pipelineText
    }

});