<script setup lang="ts">
/**
 * Run detail (web_ui.md §"Browsing Results" + item 7 "share results").
 * Shows status, per-block logs, output files with inline preview/download,
 * and — for the owner — a visibility/share control. Polls while the run is
 * still in flight.
 */
type RunFile = {
  id: string;
  name: string;
  mimeType: string | null;
  sizeBytes: number | null;
  blockId: string | null;
};
type RunLog = {
  id: string;
  blockId: string | null;
  stdout: string | null;
  stderr: string | null;
};
type RunDetail = {
  id: string;
  name: string | null;
  status: "queued" | "running" | "succeeded" | "failed" | "canceled";
  visibility: "private" | "shared" | "public";
  ownerId: string;
  error: string | null;
  createdAt: string;
  startedAt: string | null;
  finishedAt: string | null;
  files: RunFile[];
  logs: RunLog[];
};

const route = useRoute();
const id = route.params.id?.toString() ?? "";
const { user } = useAuth();

const { data: run, refresh, pending, error } = await useFetch<RunDetail>(
  `/api/runs/${id}`,
);

const isOwner = computed(() => run.value && user.value?.id === run.value.ownerId);
const inFlight = computed(
  () => run.value?.status === "queued" || run.value?.status === "running",
);

// Poll while the run is still executing.
let timer: ReturnType<typeof setInterval> | null = null;
watch(
  inFlight,
  (active) => {
    if (active && !timer) timer = setInterval(() => refresh(), 4000);
    else if (!active && timer) {
      clearInterval(timer);
      timer = null;
    }
  },
  { immediate: true },
);
onUnmounted(() => timer && clearInterval(timer));

const statusBadge = computed(() => {
  switch (run.value?.status) {
    case "succeeded":
      return { color: "green", label: "Succeeded" };
    case "failed":
      return { color: "red", label: "Failed" };
    case "running":
      return { color: "blue", label: "Running" };
    case "canceled":
      return { color: "gray", label: "Canceled" };
    default:
      return { color: "yellow", label: "Queued" };
  }
});

async function downloadAll() {
  window.open(`/api/runs/${id}/zip`, "_blank");
}

// --- Sharing (owner only) ---------------------------------------------------
const shareEmail = ref("");
const shareBusy = ref(false);
const shareMsg = ref<string | null>(null);

async function setVisibility(visibility: "private" | "shared" | "public") {
  await $fetch(`/api/runs/${id}`, { method: "PUT", body: { visibility } });
  await refresh();
}

async function shareWith() {
  if (!shareEmail.value) return;
  shareBusy.value = true;
  shareMsg.value = null;
  try {
    await $fetch(`/api/runs/${id}/share`, {
      method: "POST",
      body: { email: shareEmail.value },
    });
    shareMsg.value = `Shared with ${shareEmail.value}`;
    shareEmail.value = "";
  } catch (e: any) {
    shareMsg.value = e?.data?.statusMessage ?? "Could not share";
  } finally {
    shareBusy.value = false;
  }
}
</script>

<template>
  <div class="max-w-4xl mx-auto px-spade-lg py-spade-xl">
    <NuxtLink to="/results" class="text-sm text-spade-gray hover:text-spade-red">
      ← All runs
    </NuxtLink>

    <div v-if="error" class="mt-spade-md">
      <UCard class="border-l-4 border-l-spade-red">
        <p class="font-semibold">Run not found</p>
        <p class="text-sm text-spade-gray-dark">
          It may have been deleted, or you don't have access.
        </p>
      </UCard>
    </div>

    <template v-else-if="run">
      <div class="flex items-center justify-between gap-spade-md mt-spade-sm mb-spade-lg">
        <div class="min-w-0">
          <h1 class="font-heading text-3xl font-bold truncate">
            {{ run.name || "Untitled run" }}
          </h1>
          <p class="text-xs text-spade-gray font-mono truncate">{{ run.id }}</p>
        </div>
        <UBadge :color="statusBadge.color" variant="subtle" size="lg">
          {{ statusBadge.label }}
        </UBadge>
      </div>

      <p v-if="pending && inFlight" class="text-sm text-spade-gray mb-spade-md">
        Refreshing…
      </p>

      <UCard
        v-if="run.status === 'failed' && run.error"
        class="mb-spade-md border-l-4 border-l-spade-red"
      >
        <p class="font-semibold text-spade-red">Run failed</p>
        <pre class="text-xs whitespace-pre-wrap break-words mt-spade-sm">{{
          run.error
        }}</pre>
      </UCard>

      <!-- Output files -->
      <section class="mb-spade-lg">
        <div class="flex items-center justify-between mb-spade-sm">
          <h2 class="font-heading text-xl font-bold">Output files</h2>
          <UButton
            v-if="run.files.length > 0"
            size="sm"
            variant="outline"
            icon="i-heroicons-arrow-down-tray"
            @click="downloadAll"
          >
            Download all (ZIP)
          </UButton>
        </div>
        <p
          v-if="run.files.length === 0"
          class="text-sm text-spade-gray"
        >
          {{ inFlight ? "No files yet — the run is still going." : "No output files." }}
        </p>
        <div v-else class="flex flex-col gap-spade-sm">
          <ResultsRunFileViewer
            v-for="f in run.files"
            :key="f.id"
            :run-id="run.id"
            :name="f.name"
            :mime-type="f.mimeType"
            :size-bytes="f.sizeBytes"
          />
        </div>
      </section>

      <!-- Logs -->
      <section v-if="run.logs.length > 0" class="mb-spade-lg">
        <h2 class="font-heading text-xl font-bold mb-spade-sm">Logs</h2>
        <div class="flex flex-col gap-spade-md">
          <div
            v-for="log in run.logs"
            :key="log.id"
            class="border border-spade-gray-light rounded-spade-md p-spade-sm"
          >
            <p
              v-if="log.blockId"
              class="text-xs font-mono text-spade-gray mb-spade-xs"
            >
              block {{ log.blockId }}
            </p>
            <div v-if="log.stdout">
              <p class="text-xs font-semibold text-spade-gray-dark">stdout</p>
              <pre class="text-xs whitespace-pre-wrap break-words">{{
                log.stdout
              }}</pre>
            </div>
            <div v-if="log.stderr" class="mt-spade-sm">
              <p class="text-xs font-semibold text-spade-red">stderr</p>
              <pre class="text-xs whitespace-pre-wrap break-words">{{
                log.stderr
              }}</pre>
            </div>
          </div>
        </div>
      </section>

      <!-- Sharing (owner only) -->
      <section v-if="isOwner" class="mb-spade-lg">
        <h2 class="font-heading text-xl font-bold mb-spade-sm">Sharing</h2>
        <UCard>
          <div class="flex items-center gap-spade-sm mb-spade-md">
            <span class="text-sm text-spade-gray-dark">Visibility:</span>
            <UButtonGroup>
              <UButton
                v-for="v in (['private', 'shared', 'public'] as const)"
                :key="v"
                size="xs"
                :variant="run.visibility === v ? 'solid' : 'outline'"
                :color="run.visibility === v ? 'primary' : 'gray'"
                @click="setVisibility(v)"
              >
                {{ v }}
              </UButton>
            </UButtonGroup>
          </div>
          <div class="flex items-center gap-spade-sm">
            <UInput
              v-model="shareEmail"
              type="email"
              placeholder="Share with email…"
              class="flex-1"
            />
            <UButton :loading="shareBusy" @click="shareWith">Share</UButton>
          </div>
          <p v-if="shareMsg" class="text-xs text-spade-gray mt-spade-xs">
            {{ shareMsg }}
          </p>
        </UCard>
      </section>
    </template>
  </div>
</template>
