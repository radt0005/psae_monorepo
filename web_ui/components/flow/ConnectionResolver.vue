<script setup lang="ts">
import type { BlockManifest, EdgeData } from "~/utils/types";
import { resolveConnection, type ResolvedBinding } from "~/utils/wiring";

const props = defineProps<{ source: string; target: string }>();
const emit = defineEmits(["close", "cancel"]);

const flow = useFlow();
const upstream = ref<BlockManifest | null>(null);
const downstream = ref<BlockManifest | null>(null);
const candidates = ref<Record<string, string[]>>({});
const inputDescriptions = ref<Record<string, string | undefined>>({});
const outputDescriptions = ref<Record<string, string | undefined>>({});
const selections = ref<Record<string, string>>({});
const errorMessage = ref<string | null>(null);

const loadManifest = async (
  name: string,
): Promise<BlockManifest | null> => {
  try {
    return await $fetch<BlockManifest>(`/api/blocks/${encodeURIComponent(name)}`);
  } catch {
    return null;
  }
};

onMounted(async () => {
  const upstreamName = flow.getNameById(props.source);
  const downstreamName = flow.getNameById(props.target);
  if (!upstreamName || !downstreamName) {
    errorMessage.value = "Could not find block manifests for this connection.";
    return;
  }
  const [u, d] = await Promise.all([
    loadManifest(upstreamName),
    loadManifest(downstreamName),
  ]);
  upstream.value = u;
  downstream.value = d;
  if (!u || !d) {
    errorMessage.value = "Block manifests could not be loaded.";
    return;
  }
  const out = resolveConnection(u, d);
  if (out.kind === "ambiguous") {
    candidates.value = out.candidates;
    inputDescriptions.value = out.inputDescriptions;
    outputDescriptions.value = out.outputDescriptions;
    // Pre-fill selections with the first candidate so users only need to fix
    // disagreements.
    for (const [input, options] of Object.entries(out.candidates)) {
      if (options.length > 0) selections.value[input] = options[0];
    }
  } else {
    // Should not happen — Base.vue only opens this modal for "ambiguous".
    emit("close");
  }
});

const allInputsBound = computed(() =>
  Object.keys(candidates.value).every(
    (input) => !!selections.value[input],
  ),
);

const handleConfirm = () => {
  if (!allInputsBound.value) return;
  const bindings: ResolvedBinding[] = Object.entries(selections.value).map(
    ([input, output]) => ({ input, output }),
  );
  flow.setEdgeData(props.source, props.target, {
    bindings,
    unresolved: false,
  } satisfies EdgeData);
  emit("close");
};

const handleCancel = () => emit("cancel");
</script>

<template>
  <UCard>
    <template #header>
      <p class="font-semibold">Resolve connection</p>
      <p v-if="upstream && downstream" class="text-sm text-gray-600">
        {{ upstream.id }} → {{ downstream.id }}
      </p>
    </template>

    <p v-if="errorMessage" class="text-red-600">{{ errorMessage }}</p>

    <div v-if="!errorMessage" class="space-y-4">
      <div
        v-for="(options, inputName) in candidates"
        :key="inputName"
        class="border rounded p-3"
      >
        <p class="font-medium">
          Input <code>{{ inputName }}</code>
        </p>
        <p
          v-if="inputDescriptions[inputName]"
          class="text-sm text-gray-600 mb-2"
        >
          {{ inputDescriptions[inputName] }}
        </p>
        <div class="space-y-1">
          <label
            v-for="opt in options"
            :key="opt"
            class="flex items-start gap-2 cursor-pointer"
          >
            <input
              type="radio"
              :name="`input-${inputName}`"
              :value="opt"
              v-model="selections[inputName]"
            />
            <span>
              <code>{{ opt }}</code>
              <span
                v-if="outputDescriptions[opt]"
                class="text-sm text-gray-600 ml-2"
              >— {{ outputDescriptions[opt] }}</span>
            </span>
          </label>
        </div>
      </div>
    </div>

    <template #footer>
      <UButtonGroup>
        <UButton :disabled="!allInputsBound" @click="handleConfirm">
          Confirm
        </UButton>
        <UButton variant="outline" @click="handleCancel">Cancel</UButton>
      </UButtonGroup>
    </template>
  </UCard>
</template>
