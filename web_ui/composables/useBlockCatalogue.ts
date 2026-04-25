import type { BlockListItem } from "~/utils/types";

/**
 * Cached lookup of block metadata by name. Populated on demand by the
 * editor; nodes can render kind/network badges without each one re-fetching.
 */
export function useBlockCatalogue() {
  const state = useState<{
    items: Record<string, BlockListItem>;
    loaded: boolean;
  }>("block-catalogue", () => ({ items: {}, loaded: false }));

  const load = async () => {
    if (state.value.loaded) return;
    try {
      const res = await $fetch<{ blocks: BlockListItem[] }>("/api/blocks");
      const map: Record<string, BlockListItem> = {};
      for (const b of res.blocks) map[b.name] = b;
      state.value.items = map;
      state.value.loaded = true;
    } catch (e) {
      console.warn("[block catalogue] load failed", e);
    }
  };

  const get = (name: string | null | undefined): BlockListItem | undefined => {
    if (!name) return undefined;
    return state.value.items[name];
  };

  return { load, get, items: computed(() => state.value.items) };
}
