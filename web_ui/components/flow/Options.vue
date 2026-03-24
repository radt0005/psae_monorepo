<script setup lang="ts">
    const props = defineProps<{itemId: string}>();

    const data = await fetch("/schema", {
        method: 'POST',
        body: JSON.stringify({
            target: props.itemId
        })
    });

    const schema = await data.json();

    console.log(schema)


</script>


<template>
    <p>{{ $props.itemId }}</p>
    <div v-if="schema">
        <p>{{ schema }}</p>
    </div>
</template>