export const useBlocks = () => useState("blocks", 
    () => {
        const blockList = ref<any[]>([]);
        const schemas = ref<Record<string, any>>({});

        const loadBlockList = async () => {
            const response = await $fetch("/api/blocks/list");
            blockList.value = response.blocks;
        };

        const loadSchemaByName = async (name: string) => {
            const response = await $fetch(`/api/blocks/args?name=${name}`)
            schemas.value[name] = response
            return response
        }

        const getSchemaByName = async (id: string) => {
            let retrieved = schemas.value[id];
            if(retrieved === undefined){
                const retr = await loadSchemaByName(id);
                return retr
            } 
            return retrieved
        }

        return {
            blockList, 
            schemas,
            loadBlockList, 
            loadSchemaByName,
            getSchemaByName
        }        
    }
)