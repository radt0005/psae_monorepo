<script setup lang="ts">

const props = defineProps<{id: string, name: string}>();
const displayData = ref<any>({content: "loading..."});

onMounted(
    async () => {
        const response = await $fetch(`/api/results/${props.id}/${props.name}`);
        displayData.value = response;
    }
)

</script>

<template>
    <UContainer>
        <pre>
            {{ displayData }}
        </pre>
    </UContainer>
</template>