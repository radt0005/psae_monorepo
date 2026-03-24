<script setup lang="ts">
import { v7 } from "uuid";

    const props = defineProps<{targetId: string | null, data: boolean}>();
    const flow = useFlow();
    const emit = defineEmits(["close"]);
    const A = useBlockArgs();
    let isUpdate = false;
    
    const id = props.targetId === null ? v7() : props.targetId;
    const formData = ref<any>({})
    const blocks = ref<BlockListItem[]>([]);

    const pb = usePB();

    const getBlockData = async () => {
        const pbDataALL = await pb.value.collection("blocks").getFullList()


        const pbData = pbDataALL.filter(
            (item) => {
                if( props.data ) {
                    return item.is_data                   
                } else {
                   return  !item.is_data
                }
            }
        )

        //const defaultPath = "./block-index.json" //path.join(__dirname, "block-index.json");
        //const content = await fs.readFile(defaultPath);
        const data: BlockList = {
            blocks: pbData.map(
                (item) => {
                    return {
                        name: item.name,
                        label: item.label,
                        type: "block"
                    }
                }
            )
        } 
        //JSON.parse(content.toString())

        return data
    }

    

    const getBlockSchema = async (name: string) => {

        const pbData = await pb.value.collection("blocks").getFirstListItem(`name="${name}"`);

        //const filepath = `/home/krbundy/GitHub/psaec/blocks/${name}.json`;
        //const data = await fs.readFile(filepath);

        const parsed_data: any = pbData.schema; //JSON.parse(data.toString());

        return parsed_data

    }

    
    const { data, status, error, refresh, clear } = await useAsyncData( props.data ? 'blocks' : 'data', getBlockData );


    const schema = ref<any>(null);

    
    if(status.value === "success"){
    }
    const selectedBlock = ref<BlockListItem>();
    
    // wait for the data to load
    watch(status, () => {
        if(status.value === "success"){
            selectedBlock.value = data.value?.blocks[0]
            blocks.value = data.value?.blocks || []
        }
    });

    watch(selectedBlock, 
        async () => {
            if(selectedBlock.value?.name){
                const response = await getBlockSchema(selectedBlock.value?.name);//$fetch(`/api/schema/${selectedBlock.value?.name}`)
                schema.value = response;
            }
        }
    )

    onMounted(
        async () => {
                console.log(data.value)
            if(data.value?.blocks){
                blocks.value = data.value?.blocks
            }

            if(selectedBlock.value){
                schema.value = await $fetch(`/api/schema/${selectedBlock.value?.name}`)
                
            }
            if(props.targetId !== null) {
                isUpdate = true;
                // load the data from the store
                const args = flow.getArgsById(props.targetId);
                const selectedName = flow.getNameById(props.targetId);
                console.log(`Selected Block Name: ${selectedName}`)
                if(selectedName !== null) {

                    const selectedIndex = blocks.value.findIndex(
                        (value) => value.name == selectedName
                    )

                    if(selectedIndex > -1){
                        selectedBlock.value = blocks.value[selectedIndex]
                    }
                }
                formData.value = args;
                console.log(`Loaded args: ${JSON.stringify(args)}`)


            } else {
                // handle NULL case
                // currently don't need to do anything, actually
                
            }
        }
    )

    const handleUpdate = (data: any) => {
        formData.value = data;
    }


    const handleSubmit = () => {

        // 

        A.value.updateArgs(props.targetId!, formData.value)
        console.log(`Form submitted, saving args: ${JSON.stringify(formData.value)}`)
        console.log(A.value.getArgsById(props.targetId!))
        if( isUpdate ) {
            // save data in the store 
            flow.updateNode(
                id, 
                selectedBlock.value?.label || "",
                selectedBlock.value?.name || "",
                formData.value
                
            )


        } else {
        const block = {
            id: id,
            output: "",
            input: [],
            name: selectedBlock.value?.name || "",
            args: formData.value,
            type: "block"
        } as Block;


            flow.addNodeFromBlock(
                block, 
                Math.random() * 500,
                100 + Math.random() * 400,
                selectedBlock.value?.label || ""

            );
            // update
        }

        emit("close");
    };

    const handleCancel = () => {
        emit("close")
    }
</script>



<template>
    <UCard>
        <template #header>
            <p v-if="props.data">Data Editor</p>
            <p v-else >Block Editor</p>
        </template>
        <USelectMenu v-model="selectedBlock" :options="blocks" class="text-black"/>
        <div v-if="schema">
            <SchemaForm :schema="schema.parameters ? schema.parameters : schema" :targetId="props.targetId" @update:modelValue="handleUpdate"> </SchemaForm>
        </div>
        <template #footer>
                <UButton class="m-4" :onclick="handleSubmit">Submit</UButton>
                <UButton class="m-4" :onclick="handleCancel">Cancel</UButton>
        </template>
    </UCard>
</template>