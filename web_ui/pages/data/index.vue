<script setup lang="ts">
type DataFileRow = {
  id: string;
  name: string;
  sizeBytes: number;
  mimeType: string;
  visibility: "private" | "shared" | "public";
  ownerId: string;
  createdAt: string;
};

const tabs = [
  { key: "mine", label: "Mine" },
  { key: "shared", label: "Shared with me" },
  { key: "public", label: "Public" },
];
const activeTab = ref<"mine" | "shared" | "public">("mine");

const { data, refresh, pending } = await useFetch<{ items: DataFileRow[] }>(
  "/api/data",
  {
    query: { scope: activeTab },
    default: () => ({ items: [] }),
    watch: [activeTab],
  },
);

const items = computed(() => data.value?.items ?? []);

const fileInput = ref<HTMLInputElement | null>(null);
const uploading = ref(false);
const uploadProgress = ref(0);
const errorMessage = ref<string | null>(null);

const triggerFilePick = () => fileInput.value?.click();

const handleFile = async (event: Event) => {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;
  errorMessage.value = null;
  uploading.value = true;
  uploadProgress.value = 0;

  try {
    // 1. /init — get a presigned URL
    const init = await $fetch<{
      dataFileId: string;
      s3Key: string;
      uploadUrl: string;
      headers: Record<string, string>;
    }>("/api/uploads/data/init", {
      method: "POST",
      body: {
        name: file.name,
        mimeType: file.type || "application/octet-stream",
        sizeBytes: file.size,
      },
    });

    // 2. PUT directly to S3/MinIO with progress
    await new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.upload.addEventListener("progress", (e) => {
        if (e.lengthComputable) {
          uploadProgress.value = Math.round((e.loaded / e.total) * 100);
        }
      });
      xhr.addEventListener("load", () =>
        xhr.status >= 200 && xhr.status < 300
          ? resolve()
          : reject(new Error(`S3 upload failed: ${xhr.status}`)),
      );
      xhr.addEventListener("error", () => reject(new Error("Network error")));
      xhr.open("PUT", init.uploadUrl);
      for (const [k, v] of Object.entries(init.headers ?? {})) {
        xhr.setRequestHeader(k, v);
      }
      xhr.send(file);
    });

    // 3. /complete — write the DB row
    await $fetch("/api/uploads/data/complete", {
      method: "POST",
      body: {
        dataFileId: init.dataFileId,
        s3Key: init.s3Key,
        name: file.name,
        mimeType: file.type || "application/octet-stream",
        sizeBytes: file.size,
        visibility: "private",
      },
    });

    await refresh();
  } catch (e: any) {
    errorMessage.value = e?.message || "Upload failed.";
  } finally {
    uploading.value = false;
    uploadProgress.value = 0;
    if (fileInput.value) fileInput.value.value = "";
  }
};

const handleDelete = async (id: string) => {
  if (!confirm("Delete this file? This cannot be undone.")) return;
  await $fetch(`/api/data/${id}`, { method: "DELETE" });
  await refresh();
};

const handleDownload = async (id: string) => {
  const row = await $fetch<{ downloadUrl: string; name: string }>(
    `/api/data/${id}`,
  );
  const link = document.createElement("a");
  link.href = row.downloadUrl;
  link.download = row.name;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
};

const formatBytes = (n: number) => {
  if (n < 1024) return `${n} B`;
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 ** 3) return `${(n / 1024 ** 2).toFixed(1)} MB`;
  return `${(n / 1024 ** 3).toFixed(2)} GB`;
};
</script>

<template>
  <div class="max-w-6xl mx-auto px-spade-lg py-spade-xl">
    <div class="flex items-center justify-between mb-spade-lg">
      <h1 class="font-heading text-3xl font-bold">Data files</h1>
      <UButton color="primary" icon="i-heroicons-arrow-up-tray" @click="triggerFilePick">
        Upload file
      </UButton>
      <input
        ref="fileInput"
        type="file"
        class="hidden"
        @change="handleFile"
      />
    </div>

    <div
      v-if="uploading"
      class="mb-spade-md p-spade-md bg-spade-blue-light border-l-4 border-spade-blue rounded"
    >
      <p class="text-sm font-semibold mb-spade-sm">
        Uploading… {{ uploadProgress }}%
      </p>
      <div class="w-full h-2 bg-white rounded overflow-hidden">
        <div
          class="h-full bg-spade-blue transition-[width]"
          :style="{ width: uploadProgress + '%' }"
        />
      </div>
    </div>
    <p v-if="errorMessage" class="text-spade-red mb-spade-md">
      {{ errorMessage }}
    </p>

    <UTabs
      :items="tabs"
      :model-value="tabs.findIndex((t) => t.key === activeTab)"
      @change="(i) => (activeTab = tabs[i].key as any)"
      class="mb-spade-md"
    />

    <p v-if="pending" class="text-spade-gray">Loading…</p>
    <p
      v-else-if="items.length === 0"
      class="text-spade-gray text-center py-spade-xl"
    >
      Nothing here yet.
    </p>

    <UTable
      v-else
      :rows="items"
      :columns="[
        { key: 'name', label: 'Name' },
        { key: 'sizeBytes', label: 'Size' },
        { key: 'mimeType', label: 'Type' },
        { key: 'visibility', label: 'Visibility' },
        { key: 'actions', label: '' },
      ]"
    >
      <template #sizeBytes-data="{ row }">
        {{ formatBytes(row.sizeBytes) }}
      </template>
      <template #actions-data="{ row }">
        <UButtonGroup size="xs">
          <UButton @click="handleDownload(row.id)">Download</UButton>
          <UButton
            color="red"
            variant="outline"
            @click="handleDelete(row.id)"
            :disabled="activeTab !== 'mine'"
          >
            Delete
          </UButton>
        </UButtonGroup>
      </template>
    </UTable>
  </div>
</template>
