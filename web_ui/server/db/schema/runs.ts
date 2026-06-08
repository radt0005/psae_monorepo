import {
  pgTable,
  text,
  timestamp,
  bigint,
  pgEnum,
  index,
  primaryKey,
} from "drizzle-orm/pg-core";
import { users } from "./users";
import { pipelines } from "./pipelines";
import { visibilityEnum, sharePermissionEnum } from "./pipelines";

/**
 * Lifecycle of a submitted pipeline run.
 *  - queued:    accepted + enqueued to the scheduler, not yet started
 *  - running:   the scheduler has dispatched at least one block
 *  - succeeded: every block finished successfully
 *  - failed:    a block failed (worker.md §"Error Handling" mode 1) and the
 *               pipeline was halted
 *  - canceled:  the user (or an operator) stopped the run
 */
export const runStatusEnum = pgEnum("run_status", [
  "queued",
  "running",
  "succeeded",
  "failed",
  "canceled",
]);

/**
 * web_ui.md §"Browsing Results" — a single execution of a pipeline. The
 * pipeline YAML is snapshotted onto the run so ad-hoc submissions (no saved
 * pipeline) still work and so editing the saved pipeline later doesn't mutate
 * history. `pipelineId` is nullable for exactly that ad-hoc case.
 */
export const runs = pgTable(
  "runs",
  {
    id: text("id").primaryKey(), // UUIDv7, also the scheduler's run id
    pipelineId: text("pipeline_id").references(() => pipelines.id, {
      onDelete: "set null",
    }),
    ownerId: text("owner_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    name: text("name"), // display name snapshotted from the pipeline
    yaml: text("yaml").notNull(), // resolved (UUID-only) pipeline YAML
    status: runStatusEnum("status").notNull().default("queued"),
    error: text("error"), // failure reason when status = failed
    visibility: visibilityEnum("visibility").notNull().default("private"),
    startedAt: timestamp("started_at"),
    finishedAt: timestamp("finished_at"),
    createdAt: timestamp("created_at").notNull().defaultNow(),
    updatedAt: timestamp("updated_at").notNull().defaultNow(),
  },
  (t) => ({
    ownerIdx: index("runs_owner_idx").on(t.ownerId),
    pipelineIdx: index("runs_pipeline_idx").on(t.pipelineId),
    statusIdx: index("runs_status_idx").on(t.status),
  }),
);

/**
 * Output files produced by a run, indexed here with the S3 key the worker
 * wrote them to (worker.md §"Storage Model"). `blockId` is the invocation id
 * that produced the file so the UI can group results by block.
 */
export const runFiles = pgTable(
  "run_files",
  {
    id: text("id").primaryKey(), // UUIDv7
    runId: text("run_id")
      .notNull()
      .references(() => runs.id, { onDelete: "cascade" }),
    blockId: text("block_id"), // invocation id that produced this file
    name: text("name").notNull(), // filename as written to outputs/
    mimeType: text("mime_type"),
    sizeBytes: bigint("size_bytes", { mode: "number" }),
    s3Key: text("s3_key").notNull(),
    createdAt: timestamp("created_at").notNull().defaultNow(),
  },
  (t) => ({
    runIdx: index("run_files_run_idx").on(t.runId),
  }),
);

/**
 * Captured stdout/stderr per block invocation (worker.md §"Logging").
 */
export const runLogs = pgTable(
  "run_logs",
  {
    id: text("id").primaryKey(), // UUIDv7
    runId: text("run_id")
      .notNull()
      .references(() => runs.id, { onDelete: "cascade" }),
    blockId: text("block_id"), // invocation id the logs belong to
    stdout: text("stdout"),
    stderr: text("stderr"),
    createdAt: timestamp("created_at").notNull().defaultNow(),
  },
  (t) => ({
    runIdx: index("run_logs_run_idx").on(t.runId),
  }),
);

/**
 * Per-run sharing ACL (web_ui.md item 7 — "share results"). Mirrors
 * pipeline_shares / data_file_shares.
 */
export const runShares = pgTable(
  "run_shares",
  {
    runId: text("run_id")
      .notNull()
      .references(() => runs.id, { onDelete: "cascade" }),
    userId: text("user_id")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    permission: sharePermissionEnum("permission").notNull().default("view"),
    createdAt: timestamp("created_at").notNull().defaultNow(),
  },
  (t) => ({
    pk: primaryKey({ columns: [t.runId, t.userId] }),
    userIdx: index("run_shares_user_idx").on(t.userId),
  }),
);

export type Run = typeof runs.$inferSelect;
export type NewRun = typeof runs.$inferInsert;
export type RunFile = typeof runFiles.$inferSelect;
export type NewRunFile = typeof runFiles.$inferInsert;
export type RunLog = typeof runLogs.$inferSelect;
export type NewRunLog = typeof runLogs.$inferInsert;
export type RunShare = typeof runShares.$inferSelect;