<script setup lang="ts">
    const { addNodeFromBlock } = useFlow();
    const emit = defineEmits(["close"])
    const props = defineProps<{targetId: string | null}>();

    const blocks = [
        {
            name: "data-public-fia",
            label: "Public Forest Service Data",
            type: "data"
        },
        {
            name: "data-usgs-dem-1m",
            label: "USGS 1M 3DEP DEM"
        },
        {
            name: "user-file-upload",
            label: "User provided file"
        }
    ];

     const pb = usePB();

    const getBlockData = async () => {
        const pbDataALL = await pb.value.collection("blocks").getFullList()
        const pbData = pbDataALL.filter(
            (item) => ! item.is_data
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
    const { data, status, error, refresh, clear } = await useAsyncData( 'mountains', getBlockData );



    const selectedBlock = ref(blocks[0]);


    const handleSubmit = () => {
        const block = {
            id: selectedBlock.value.name,
            input: [],
            output: "", 
            name: selectedBlock.value.name,
            args: {}
        } as Block;

        addNodeFromBlock(
            block, 
            Math.random() * 400,
            Math.random() * 100,
            selectedBlock.value.label
        );

        emit("close")
    }

    const handleCancel = () => {
        emit("close")
    }

</script>



<template>
    <UCard>
        <template #header>
            <p>Data Editor</p>
        </template>
        <USelectMenu v-model="selectedBlock" :options="blocks" />
        <FlowOptions :item-id="selectedBlock.name"></FlowOptions>
        <template #footer>
                <UButton class="p-4" :onclick="handleSubmit">Submit</UButton>
                <UButton class="p-4" :onclick="handleCancel">Cancel</UButton>
        </template>
    </UCard>
</template>