import { pgTable, text, timestamp, jsonb, index } from "drizzle-orm/pg-core";
import type { BlockManifest } from "~/utils/types";

/**
 * Block registry — mirrors `~/.spade/blocks/` index from worker.md §"Block
 * Registry" but for the cloud-hosted block catalogue. Source of truth for
 * the editor's block picker and the connection resolver's manifest lookups.
 */
export const blocks = pgTable(
  "blocks",
  {
    id: text("id").primaryKey(), // unique manifest id, e.g. "gdal.rasterize"
    name: text("name").notNull().unique(),
    label: text("label").notNull(),
    version: text("version").notNull(),
    /** Full BlockManifest stored as jsonb for flexibility. */
    manifest: jsonb("manifest").$type<BlockManifest>().notNull(),
    createdAt: timestamp("created_at").notNull().defaultNow(),
    updatedAt: timestamp("updated_at").notNull().defaultNow(),
  },
  (t) => ({
    nameIdx: index("blocks_name_idx").on(t.name),
  }),
);

export type Block = typeof blocks.$inferSelect;
export type NewBlock = typeof blocks.$inferInsert;
