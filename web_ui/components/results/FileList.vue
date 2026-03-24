<script setup lang="ts">
const props = defineProps<{id: string}>();
const files = ref<string[]>([]);

onMounted(
    async () => {
        const data = await $fetch(`/api/results/${props.id}`);
        //files.value = data;
        console.log(data)
    }
)

const checkFile = (file: string) => {
    const lowerCaseFile = file.toLowerCase();
    return lowerCaseFile.includes(".csv") || lowerCaseFile.includes(".json") || lowerCaseFile.includes(".geojson")
}
</script>

<template>
    <div class="w-full overflow-y-auto">
        <h1>Results</h1>
        <p>Run ID: {{ props.id }}</p>
        <ul>
            <li class="p-2" v-for="file of files"> 
                <p>{{ file }}</p> 
                <UButton v-if="checkFile(file)" class="pl-5" :to="`results/view/${props.id}/${file}`">View</UButton>
            </li>
        </ul>
    </div>

</template>