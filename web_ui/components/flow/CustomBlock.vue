<script setup lang="ts">
import { Handle, Position } from "@vue-flow/core";
import type { EdgeData } from "~/utils/types";

const props = defineProps<{ name: string; id: string }>();
const emit = defineEmits(["edit", "delete"]);

const flow = useFlow();
const catalogue = useBlockCatalogue();

const blockName = computed(() => flow.getNameById(props.id));
const meta = computed(() => catalogue.get(blockName.value));

const hasUnresolved = computed(() =>
  flow.edges.some(
    (e) =>
      (e.source === props.id || e.target === props.id) &&
      (e.data as EdgeData | undefined)?.unresolved,
  ),
);

const kindStyles = computed(() => {
  switch (meta.value?.kind) {
    case "map":
      return {
        ring: "ring-2 ring-spade-blue/40",
        accent: "border-l-4 border-l-spade-blue",
        badge: { color: "blue", label: "map" },
      };
    case "reduce":
      return {
        ring: "ring-2 ring-spade-green/40",
        accent: "border-l-4 border-l-spade-green",
        badge: { color: "green", label: "reduce" },
      };
    default:
      return { ring: "", accent: "", badge: null };
  }
});
</script>

<template>
  <div
    class="p-spade-md bg-spade-white rounded-spade-md shadow-card border border-[#e0e0e0] flex items-center justify-between gap-spade-sm relative w-64"
    :class="[
      kindStyles.ring,
      kindStyles.accent,
      hasUnresolved ? 'border-spade-red ring-1 ring-spade-red/40' : '',
    ]"
  >
    <div class="flex flex-col flex-1 min-w-0">
      <div class="flex items-center gap-spade-xs">
        <span class="font-semibold truncate">{{ name }}</span>
        <UBadge
          v-if="kindStyles.badge"
          :color="kindStyles.badge.color"
          variant="subtle"
          size="xs"
        >
          {{ kindStyles.badge.label }}
        </UBadge>
        <UBadge
          v-if="meta?.network"
          color="amber"
          variant="subtle"
          size="xs"
          title="Requires network access"
        >
          net
        </UBadge>
      </div>
      <span
        v-if="hasUnresolved"
        class="text-xs text-spade-red"
        title="A connection on this block needs disambiguation."
      >
        ⚠ unresolved connection
      </span>
    </div>
    <div class="flex space-x-1 shrink-0">
      <UButton
        @click="emit('edit', { ...props })"
        class="bg-primary-600 text-white p-1 rounded"
        icon="i-material-symbols:edit"
        size="xs"
      />
      <UButton
        @click="emit('delete', { id: props.id })"
        class="bg-spade-red text-white p-1 rounded"
        icon="i-material-symbols:delete-rounded"
        size="xs"
      />
    </div>
    <Handle type="target" :position="Position.Top" />
    <Handle type="source" :position="Position.Bottom" />
  </div>
</template>
