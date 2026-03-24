<script setup lang="ts">
    const pb = usePB();

    const props = defineProps<{id: string, name: string}>();
    console.log(props)

    const { data, refresh, status } = useAsyncData(`file_${props.id}`,
        async () => {
            const file = await pb.value.collection("files").getOne(props.id);
            const token = await pb.value.files.getToken()
            const url = pb.value.files.getUrl(file, props.name, {token: token})
            const file_fetch = await fetch(url)
            const file_text = await file_fetch.text()

            console.log(url)

            return file_text
        }
     )

    watch(props, () => {
        refresh()
    })

</script>

<template>
    <pre style="word-wrap: break-word; white-space: pre-wrap;">

        {{ data }}
    </pre>
</template>
<style>
    pre {
        word-wrap: break-word;
    }
    p {
        word-wrap: break-word;
    }
    body {
  word-wrap: break-word;
}
</style>