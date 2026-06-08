<script setup lang="ts">
import parse from "csv-simple-parser";

/**
 * Inline viewer for a single run output file. Given a run id + filename it
 * fetches a short-lived presigned URL from /api/runs/:id/files/:name, then
 * renders small files inline (image / CSV table / JSON / text). Large or
 * binary files fall back to a download link — S3 serves the bytes directly
 * with Range support, so we never proxy them through the app.
 */
const props = defineProps<{
  runId: string;
  name: string;
  mimeType?: string | null;
  sizeBytes?: number | null;
}>();

// Don't try to inline anything bigger than this; offer a download instead.
const INLINE_LIMIT = 2 * 1024 * 1024; // 2 MB

const ext = computed(() => props.name.split(".").pop()?.toLowerCase() ?? "");
const kind = computed<"image" | "csv" | "json" | "text" | "binary">(() => {
  const e = ext.value;
  const m = props.mimeType ?? "";
  if (m.startsWith("image/") || ["png", "jpg", "jpeg", "gif", "webp", "svg"].includes(e))
    return "image";
  if (e === "csv" || m === "text/csv") return "csv";
  if (e === "json" || e === "geojson" || m.includes("json")) return "json";
  if (
    ["txt", "log", "md", "yaml", "yml", "wkt", "tsv"].includes(e) ||
    m.startsWith("text/")
  )
    return "text";
  return "binary";
});

const tooBig = computed(
  () => props.sizeBytes != null && props.sizeBytes > INLINE_LIMIT,
);

const url = ref<string | null>(null);
const textBody = ref<string | null>(null);
const rows = ref<any[] | null>(null);
const error = ref<string | null>(null);
const loading = ref(false);

async function presign(): Promise<string> {
  const res = await $fetch<{ url: string }>(
    `/api/runs/${props.runId}/files/${encodeURIComponent(props.name)}`,
  );
  return res.url;
}

async function load() {
  loading.value = true;
  error.value = null;
  try {
    const u = await presign();
    url.value = u;
    if (kind.value === "image" || kind.value === "binary" || tooBig.value)
      return; // rendered via the URL / download link directly
    const body = await (await fetch(u)).text();
    if (kind.value === "csv") {
      rows.value = parse(body, { header: true }) as any[];
    } else if (kind.value === "json") {
      try {
        textBody.value = JSON.stringify(JSON.parse(body), null, 2);
      } catch {
        textBody.value = body;
      }
    } else {
      textBody.value = body;
    }
  } catch (e: any) {
    error.value = e?.message ?? "Failed to load file";
  } finally {
    loading.value = false;
  }
}

// Owner clicks "View" to expand — lazy so we don't presign every file at once.
const expanded = ref(false);
async function toggle() {
  expanded.value = !expanded.value;
  if (expanded.value && url.value === null) await load();
}

async function download() {
  const u = url.value ?? (await presign());
  window.open(u, "_blank");
}
</script>

<template>
  <div class="border border-spade-gray-light rounded-spade-md">
    <div class="flex items-center justify-between gap-spade-md p-spade-sm">
      <button
        class="font-mono text-sm text-left truncate hover:text-spade-red"
        @click="toggle"
      >
        <span class="text-spade-gray">{{ expanded ? "▾" : "▸" }}</span>
        {{ name }}
      </button>
      <div class="flex items-center gap-spade-sm shrink-0">
        <span v-if="sizeBytes != null" class="text-xs text-spade-gray">
          {{ (sizeBytes / 1024).toFixed(1) }} KB
        </span>
        <UButton size="2xs" variant="ghost" @click="download">Download</UButton>
      </div>
    </div>

    <div v-if="expanded" class="border-t border-spade-gray-light p-spade-sm">
      <p v-if="loading" class="text-sm text-spade-gray">Loading…</p>
      <p v-else-if="error" class="text-sm text-spade-red">{{ error }}</p>

      <p v-else-if="tooBig" class="text-sm text-spade-gray">
        File is large ({{ (sizeBytes! / 1024 / 1024).toFixed(1) }} MB) —
        <button class="text-spade-red underline" @click="download">
          download
        </button>
        to view.
      </p>

      <img
        v-else-if="kind === 'image' && url"
        :src="url"
        :alt="name"
        class="max-w-full rounded-spade-sm"
      />

      <div v-else-if="kind === 'csv' && rows" class="overflow-x-auto">
        <UTable :rows="rows" />
      </div>

      <pre
        v-else-if="textBody !== null"
        class="text-xs whitespace-pre-wrap break-words max-h-96 overflow-y-auto"
        >{{ textBody }}</pre
      >

      <p v-else class="text-sm text-spade-gray">
        No inline preview available —
        <button class="text-spade-red underline" @click="download">
          download
        </button>
        to view.
      </p>
    </div>
  </div>
</template>
