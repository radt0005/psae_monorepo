import { eq } from "drizzle-orm";
import { blocks, type Block, type NewBlock } from "../schema/blocks";
import type { BlockManifest } from "~/utils/types";
import type { Db } from "../index";

export type BlockRepo = ReturnType<typeof makeBlockRepo>;

export function makeBlockRepo(db: Db) {
  return {
    async listAll(): Promise<Block[]> {
      return db.select().from(blocks).orderBy(blocks.name);
    },

    async getByName(name: string): Promise<Block | null> {
      const rows = await db
        .select()
        .from(blocks)
        .where(eq(blocks.name, name))
        .limit(1);
      return rows[0] ?? null;
    },

    async getById(id: string): Promise<Block | null> {
      const rows = await db
        .select()
        .from(blocks)
        .where(eq(blocks.id, id))
        .limit(1);
      return rows[0] ?? null;
    },

    /**
     * Insert or update a block by name. Used by the admin upload endpoint
     * and by the migration script that ports rows from PocketBase.
     */
    async upsert(input: {
      label: string;
      manifest: BlockManifest;
    }): Promise<Block> {
      const row: NewBlock = {
        id: input.manifest.id,
        name: input.manifest.id,
        label: input.label,
        version: input.manifest.version,
        manifest: input.manifest,
        updatedAt: new Date(),
      };
      const inserted = await db
        .insert(blocks)
        .values(row)
        .onConflictDoUpdate({
          target: blocks.name,
          set: {
            label: row.label,
            version: row.version,
            manifest: row.manifest,
            updatedAt: row.updatedAt,
          },
        })
        .returning();
      return inserted[0];
    },

    async delete(name: string): Promise<void> {
      await db.delete(blocks).where(eq(blocks.name, name));
    },
  };
}
