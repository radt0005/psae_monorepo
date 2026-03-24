<script setup lang="ts">
    import  parse  from "csv-simple-parser"
    const props = defineProps<{id: string, name: string}>();
    const pb = usePB()

     const { data, refresh, status } = useAsyncData(`file_${props.id}`,
        async () => {
            let returnData: any[] = [];
            const file = await pb.value.collection("files").getOne(props.id);
            const token = await pb.value.files.getToken()
            const url = pb.value.files.getUrl(file, props.name, {token: token})
            const file_fetch = await fetch(url)
            const file_text = await file_fetch.text()
            console.log(file_text)
            if(file_text.startsWith('{"code":404,')){
                return [{"status": "404 File Not Found"}]
            }

            try {
                returnData = parse(file_text, {header: true})
            } catch(err) {
                console.error(err)
            }



            console.log(url)

            return returnData
        }
     )

    onMounted(
        async () => {

            
        }
    )
</script>
<template>
    <div v-if="status === 'success'">

        <div v-if="data !== null" class="w-full overflow-y-auto">
            <UTable :rows="data" class="flex-1"></UTable>
        </div>
    </div>
</template>