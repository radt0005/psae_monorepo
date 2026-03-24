export const useBlockArgs = () => useState("block_args", 
    () => {
        const argsById = ref<Record<string, any>>({});

        const updateArgs = (id: string, value: any) => {
            argsById.value[id] = value;
        }

        const getArgsById = (id: string) => {
            return argsById.value[id] || {}
        }

        return {
            argsById: argsById, 
            updateArgs: updateArgs, 
            getArgsById: getArgsById
        }
    }
)