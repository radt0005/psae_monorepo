<script setup lang="ts">
const pb = usePB()

const props = defineProps<{id: string, out: boolean}>();

const { data, refresh, status } = useAsyncData(`std_${props.id}`, 
    async () => {
        console.log("Loading data")
        const results = await pb.value.collection("job_status").getFirstListItem(
            `run_id="${props.id}"`
        )
        console.log(`Results: ${JSON.stringify(results)}`)

        return results

        
    }
)


</script>

<template>
    <h1 class="p-2" style="font-size: large; font-weight: 100;" >Standard {{ props.out ? "Output"  : "Error"  }}</h1>
    {{ data }}
    <div v-if="status === 'success'">
        <pre v-if="data !== null" style="word-wrap: break-word; white-space: pre-wrap;">
            {{ props.out ? data.stdout : data.stderr }}
        </pre>
    </div>

</template>
