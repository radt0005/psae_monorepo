<script setup lang="ts">
    const pb = usePB()
    const props = defineProps<{id: string, name: string}>();


    const { data, refresh, status } = useAsyncData(`file_${props.id}`,
        async () => {
            const file = await pb.value.collection("files").getOne(props.id);
            const token = await pb.value.files.getToken()
            const url = pb.value.files.getUrl(file, props.name, {token: token})
            console.log(url)

            return url
        }
     )
</script>
<template>
    <div v-if="status === 'success'">
        <img v-if="data !== null" :src="data">
    </div>


</template>