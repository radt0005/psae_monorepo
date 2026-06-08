<script setup lang="ts">
import { Handle, Position } from "@vue-flow/core";
import type { EdgeData } from "~/utils/types";
import { isBroadcastEdge, activeMapFor } from "~/utils/mapContext";

const props = defineProps<{ name: string; id: string }>();
const emit = defineEmits(["edit", "delete"]);

const flow = useFlow();
const catalogue = useBlockCatalogue();
const { result: mapCtx } = useMapContext();

const blockName = computed(() => flow.getNameById(props.id));
const meta = computed(() => catalogue.get(blockName.value));

const hasUnresolved = computed(() =>
  flow.edges.some(
    (e) =>
      (e.source === props.id || e.target === props.id) &&
      (e.data as EdgeData | undefined)?.unresolved,
  ),
);

/** This block runs N times inside a map context (downstream of a map). */
const inMapContext = computed(() => mapCtx.value.inContext.has(props.id));

/** The map block whose context this block runs in (for the tooltip). */
const contextMapName = computed(() => {
  const mapId = activeMapFor(mapCtx.value, props.id);
  return mapId ? flow.getNameById(mapId) : null;
});

/** A map block that illegally opens while already inside another context. */
const isNestedMap = computed(() => mapCtx.value.nestedMaps.includes(props.id));

/** Non-mapped dependency fanning into this in-context block (broadcast input). */
const hasBroadcastInput = computed(() =>
  flow.edges.some(
    (e) =>
      e.target === props.id &&
      isBroadcastEdge(mapCtx.value, e.source, props.id),
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
    class="p-spade-md rounded-spade-md shadow-card border border-[#e0e0e0] flex items-center justify-between gap-spade-sm relative w-64"
    :class="[
      kindStyles.ring,
      kindStyles.accent,
      inMapContext ? 'bg-spade-blue/5' : 'bg-spade-white',
      hasUnresolved ? 'border-spade-red ring-1 ring-spade-red/40' : '',
      isNestedMap ? 'border-spade-red ring-2 ring-spade-red/50' : '',
    ]"
  >
    <div class="flex flex-col flex-1 min-w-0">
      <div class="flex items-center gap-spade-xs flex-wrap">
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
        <UBadge
          v-if="inMapContext"
          color="blue"
          variant="subtle"
          size="xs"
          :title="`Runs once per item in the map context opened by '${contextMapName ?? 'a map block'}'.`"
        >
          in map
        </UBadge>
        <UBadge
          v-if="hasBroadcastInput"
          color="gray"
          variant="subtle"
          size="xs"
          title="Has a broadcast input — a non-mapped dependency that feeds every mapped invocation."
        >
          broadcast
        </UBadge>
      </div>
      <span
        v-if="isNestedMap"
        class="text-xs text-spade-red"
        title="A map block cannot open while already inside another map context. Close the outer map with a reduce first."
      >
        ⚠ nested map not allowed
      </span>
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
        class="bg-spade-blue text-white p-1 rounded ring-0"
        icon="i-material-symbols:edit"
        size="xs"
      />
      <UButton
        @click="emit('delete', { id: props.id })"
        class="bg-spade-red text-white p-1 rounded ring-0"
        icon="i-material-symbols:delete-rounded"
        size="xs"
      />
    </div>
    <Handle type="target" :position="Position.Top" />
    <Handle type="source" :position="Position.Bottom" />
  </div>
</template>
