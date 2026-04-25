import {
  pgTable,
  text,
  timestamp,
  bigint,
  index,
  primaryKey,
} from "drizzle-orm/pg-core";
import { users } from "./users";
import { visibilityEnum, sharePermissionEnum } from "./pipelines";

/**
 * web_ui.md §"User Interface for Pipeline Generation" — user-uploaded data
 * (often large geospatial files) that can be referenced from pipelines.
 * Bytes live in MinIO/S3; this table is the index + ACL.
 */
export const dataFiles = pgTable(
  "data_files",
  {
    id: text("id").primaryKey(), // UUIDv7
    ownerId: text("owner_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    name: text("name").notNull(), // user-facing display name
    sizeBytes: bigint("size_bytes", { mode: "number" }).notNull(),
    mimeType: text("mime_type").notNull(),
    s3Key: text("s3_key").notNull(),
    visibility: visibilityEnum("visibility").notNull().default("private"),
    createdAt: timestamp("created_at").notNull().defaultNow(),
    updatedAt: timestamp("updated_at").notNull().defaultNow(),
  },
  (t) => ({
    ownerIdx: index("data_files_owner_idx").on(t.ownerId),
    s3KeyIdx: index("data_files_s3_key_idx").on(t.s3Key),
  }),
);

export const dataFileShares = pgTable(
  "data_file_shares",
  {
    dataFileId: text("data_file_id")
      .notNull()
      .references(() => dataFiles.id, { onDelete: "cascade" }),
    userId: text("user_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    permission: sharePermissionEnum("permission").notNull().default("view"),
    createdAt: timestamp("created_at").notNull().defaultNow(),
  },
  (t) => ({
    pk: primaryKey({ columns: [t.dataFileId, t.userId] }),
    userIdx: index("data_file_shares_user_idx").on(t.userId),
  }),
);

export type DataFile = typeof dataFiles.$inferSelect;
export type NewDataFile = typeof dataFiles.$inferInsert;
export type DataFileShare = typeof dataFileShares.$inferSelect;
