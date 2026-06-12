import { useDb } from "~/server/db";
import { makeBlockRepo } from "~/server/db/repositories/blocks";
import type { BlockList, BlockListItem } from "~/utils/types";

export default defineEventHandler(async (_event) => {
  const repo = makeBlockRepo(useDb());
  const rows = await repo.listAll();

  const items: BlockListItem[] = rows.map((row) => ({
    id: row.manifest.id,
    name: row.name,
    label: row.label,
    kind: row.manifest.kind,
    network: row.manifest.network,
    description: row.manifest.description,
  }));

  return { blocks: items } satisfies BlockList;
});
