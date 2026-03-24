<script setup lang="ts">
const outputs = ref<string[]>([]);
const route = useRoute();
const id: string = typeof route.params.id === "string" ? route.params.id : route.params.id.join("");

// load the files with a time, polling every few seconds for updates

const checkUpdates = async () => {
    const response: {files: string[]} = await $fetch(`/api/results/${id}`);
    outputs.value = response.files;
}

const autoCheckUpdates = () => {
    setTimeout(
        () => {
            checkUpdates()
        },
        5000
    )
}

onMounted(
    () => {
        autoCheckUpdates();
    }
);

// some options here: getting logs, etc.

</script>

<template>
    <p>In Progress....</p>
    <p>Outputs Produced so far</p>
    <ul>
        <li v-for="f in outputs">{{ f }}</li>
    </ul>
</template>