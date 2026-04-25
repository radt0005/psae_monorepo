import {
  pgTable,
  text,
  timestamp,
  pgEnum,
  index,
  primaryKey,
} from "drizzle-orm/pg-core";
import { users } from "./users";

export const visibilityEnum = pgEnum("visibility", [
  "private",
  "shared",
  "public",
]);

export const sharePermissionEnum = pgEnum("share_permission", [
  "view",
  "edit",
]);

/**
 * web_ui.md §"Pipelines" — store + share saved pipeline YAML.
 */
export const pipelines = pgTable(
  "pipelines",
  {
    id: text("id").primaryKey(), // UUIDv7 generated client-side
    ownerId: text("owner_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    name: text("name").notNull(),
    description: text("description"),
    version: text("version").notNull(),
    yaml: text("yaml").notNull(),
    visibility: visibilityEnum("visibility").notNull().default("private"),
    createdAt: timestamp("created_at").notNull().defaultNow(),
    updatedAt: timestamp("updated_at").notNull().defaultNow(),
  },
  (t) => ({
    ownerIdx: index("pipelines_owner_idx").on(t.ownerId),
    visibilityIdx: index("pipelines_visibility_idx").on(t.visibility),
  }),
);

export const pipelineShares = pgTable(
  "pipeline_shares",
  {
    pipelineId: text("pipeline_id")
      .notNull()
      .references(() => pipelines.id, { onDelete: "cascade" }),
    userId: text("user_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    permission: sharePermissionEnum("permission").notNull().default("view"),
    createdAt: timestamp("created_at").notNull().defaultNow(),
  },
  (t) => ({
    pk: primaryKey({ columns: [t.pipelineId, t.userId] }),
    userIdx: index("pipeline_shares_user_idx").on(t.userId),
  }),
);

export type Pipeline = typeof pipelines.$inferSelect;
export type NewPipeline = typeof pipelines.$inferInsert;
export type PipelineShare = typeof pipelineShares.$inferSelect;
