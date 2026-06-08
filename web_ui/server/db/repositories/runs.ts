import { and, asc, desc, eq, inArray } from "drizzle-orm";
import { v7 } from "uuid";
import {
  runs,
  runFiles,
  runLogs,
  runShares,
  type Run,
  type NewRun,
  type RunFile,
  type NewRunFile,
  type RunLog,
  type NewRunLog,
} from "../schema/runs";
import type { Db } from "../index";

export type RunRepo = ReturnType<typeof makeRunRepo>;

export type RunWithDetails = Run & {
  files: RunFile[];
  logs: RunLog[];
};

/**
 * Run repository — owner / shared / public ACLs per web_ui.md §"Browsing
 * Results" + item 7 (share results). Mirrors the pipeline/data-file repos so
 * the access model is identical across resources. Worker callbacks
 * (status/file/log writes) go through the append* / updateStatus methods.
 */
export function makeRunRepo(db: Db) {
  return {
    async create(input: NewRun): Promise<Run> {
      const [row] = await db.insert(runs).values(input).returning();
      return row;
    },

    async listOwnedBy(userId: string): Promise<Run[]> {
      return db
        .select()
        .from(runs)
        .where(eq(runs.ownerId, userId))
        .orderBy(desc(runs.createdAt));
    },

    async listSharedWith(userId: string): Promise<Run[]> {
      const shareRows = await db
        .select({ runId: runShares.runId })
        .from(runShares)
        .where(eq(runShares.userId, userId));
      const ids = shareRows.map((r) => r.runId);
      if (ids.length === 0) return [];
      return db
        .select()
        .from(runs)
        .where(inArray(runs.id, ids))
        .orderBy(desc(runs.createdAt));
    },

    async listPublic(): Promise<Run[]> {
      return db
        .select()
        .from(runs)
        .where(eq(runs.visibility, "public"))
        .orderBy(desc(runs.createdAt));
    },

    /**
     * Fetch a run only if the user may see it (owner, shared with, or public).
     * Returns null otherwise so callers can 404.
     */
    async getByIdForUser(id: string, userId: string): Promise<Run | null> {
      const row = (
        await db.select().from(runs).where(eq(runs.id, id)).limit(1)
      )[0];
      if (!row) return null;
      if (row.ownerId === userId || row.visibility === "public") return row;

      const share = (
        await db
          .select()
          .from(runShares)
          .where(and(eq(runShares.runId, id), eq(runShares.userId, userId)))
          .limit(1)
      )[0];
      return share ? row : null;
    },

    /**
     * Same ACL as getByIdForUser, but eagerly loads files + logs for the run
     * detail page.
     */
    async getWithDetails(
      id: string,
      userId: string,
    ): Promise<RunWithDetails | null> {
      const run = await this.getByIdForUser(id, userId);
      if (!run) return null;
      const [files, logs] = await Promise.all([
        db
          .select()
          .from(runFiles)
          .where(eq(runFiles.runId, id))
          .orderBy(asc(runFiles.name)),
        db
          .select()
          .from(runLogs)
          .where(eq(runLogs.runId, id))
          .orderBy(asc(runLogs.createdAt)),
      ]);
      return { ...run, files, logs };
    },

    /**
     * Fetch a run with no ACL check. Only for trusted server-side callers
     * (the scheduler/worker callback), never a browser-facing path.
     */
    async getById(id: string): Promise<Run | null> {
      const row = (
        await db.select().from(runs).where(eq(runs.id, id)).limit(1)
      )[0];
      return row ?? null;
    },

    /** Look up a single result file row scoped to a run (for downloads). */
    async getFile(runId: string, name: string): Promise<RunFile | null> {
      const row = (
        await db
          .select()
          .from(runFiles)
          .where(and(eq(runFiles.runId, runId), eq(runFiles.name, name)))
          .limit(1)
      )[0];
      return row ?? null;
    },

    /**
     * Record an output file the worker uploaded. Used by the worker callback;
     * not ACL-checked here (the endpoint authenticates the worker).
     */
    async appendFile(
      input: Omit<NewRunFile, "id"> & { id?: string },
    ): Promise<RunFile> {
      const [row] = await db
        .insert(runFiles)
        .values({ id: input.id ?? v7(), ...input })
        .returning();
      return row;
    },

    async appendLog(
      input: Omit<NewRunLog, "id"> & { id?: string },
    ): Promise<RunLog> {
      const [row] = await db
        .insert(runLogs)
        .values({ id: input.id ?? v7(), ...input })
        .returning();
      return row;
    },

    /**
     * Worker/scheduler status callback. Stamps started_at / finished_at as the
     * lifecycle advances.
     */
    async updateStatus(
      id: string,
      status: Run["status"],
      opts: { error?: string | null } = {},
    ): Promise<Run | null> {
      const patch: Partial<NewRun> = {
        status,
        updatedAt: new Date(),
      };
      if (opts.error !== undefined) patch.error = opts.error;
      if (status === "running") patch.startedAt = new Date();
      if (
        status === "succeeded" ||
        status === "failed" ||
        status === "canceled"
      ) {
        patch.finishedAt = new Date();
      }
      const [row] = await db
        .update(runs)
        .set(patch)
        .where(eq(runs.id, id))
        .returning();
      return row ?? null;
    },

    async setVisibility(
      id: string,
      ownerId: string,
      visibility: Run["visibility"],
    ): Promise<Run | null> {
      const [row] = await db
        .update(runs)
        .set({ visibility, updatedAt: new Date() })
        .where(and(eq(runs.id, id), eq(runs.ownerId, ownerId)))
        .returning();
      return row ?? null;
    },

    async share(
      runId: string,
      ownerId: string,
      shareWithUserId: string,
      permission: "view" | "edit" = "view",
    ): Promise<boolean> {
      const owns = await db
        .select()
        .from(runs)
        .where(and(eq(runs.id, runId), eq(runs.ownerId, ownerId)))
        .limit(1);
      if (owns.length === 0) return false;
      await db
        .insert(runShares)
        .values({ runId, userId: shareWithUserId, permission })
        .onConflictDoUpdate({
          target: [runShares.runId, runShares.userId],
          set: { permission },
        });
      return true;
    },

    async unshare(
      runId: string,
      ownerId: string,
      userId: string,
    ): Promise<boolean> {
      const owns = await db
        .select()
        .from(runs)
        .where(and(eq(runs.id, runId), eq(runs.ownerId, ownerId)))
        .limit(1);
      if (owns.length === 0) return false;
      await db
        .delete(runShares)
        .where(and(eq(runShares.runId, runId), eq(runShares.userId, userId)));
      return true;
    },
  };
}
