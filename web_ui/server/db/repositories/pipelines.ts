import { and, desc, eq, inArray, or } from "drizzle-orm";
import {
  pipelines,
  pipelineShares,
  type Pipeline,
  type NewPipeline,
} from "../schema/pipelines";
import type { Db } from "../index";

export type PipelineRepo = ReturnType<typeof makePipelineRepo>;

/**
 * Pipeline library repository — owner / shared / public ACLs per
 * web_ui.md §"Pipelines". Every read goes through one of the explicit
 * list/get methods so we can't accidentally leak rows across tenants.
 */
export function makePipelineRepo(db: Db) {
  return {
    async listOwnedBy(userId: string): Promise<Pipeline[]> {
      return db
        .select()
        .from(pipelines)
        .where(eq(pipelines.ownerId, userId))
        .orderBy(desc(pipelines.updatedAt));
    },

    async listSharedWith(userId: string): Promise<Pipeline[]> {
      const shareRows = await db
        .select({ pipelineId: pipelineShares.pipelineId })
        .from(pipelineShares)
        .where(eq(pipelineShares.userId, userId));
      const ids = shareRows.map((r) => r.pipelineId);
      if (ids.length === 0) return [];
      return db
        .select()
        .from(pipelines)
        .where(inArray(pipelines.id, ids))
        .orderBy(desc(pipelines.updatedAt));
    },

    async listPublic(): Promise<Pipeline[]> {
      return db
        .select()
        .from(pipelines)
        .where(eq(pipelines.visibility, "public"))
        .orderBy(desc(pipelines.updatedAt));
    },

    /**
     * Fetch a pipeline only if the user is allowed to see it (owner, shared
     * with, or public). Returns null otherwise so callers can 404.
     */
    async getByIdForUser(
      id: string,
      userId: string,
    ): Promise<Pipeline | null> {
      const row = (
        await db.select().from(pipelines).where(eq(pipelines.id, id)).limit(1)
      )[0];
      if (!row) return null;
      if (row.ownerId === userId || row.visibility === "public") return row;

      const share = (
        await db
          .select()
          .from(pipelineShares)
          .where(
            and(
              eq(pipelineShares.pipelineId, id),
              eq(pipelineShares.userId, userId),
            ),
          )
          .limit(1)
      )[0];
      return share ? row : null;
    },

    async create(input: NewPipeline): Promise<Pipeline> {
      const [row] = await db.insert(pipelines).values(input).returning();
      return row;
    },

    /**
     * Only the owner may update. Returns null if no row was updated (caller
     * 403s).
     */
    async update(
      id: string,
      ownerId: string,
      patch: Partial<NewPipeline>,
    ): Promise<Pipeline | null> {
      const [row] = await db
        .update(pipelines)
        .set({ ...patch, updatedAt: new Date() })
        .where(and(eq(pipelines.id, id), eq(pipelines.ownerId, ownerId)))
        .returning();
      return row ?? null;
    },

    async delete(id: string, ownerId: string): Promise<boolean> {
      const out = await db
        .delete(pipelines)
        .where(and(eq(pipelines.id, id), eq(pipelines.ownerId, ownerId)))
        .returning({ id: pipelines.id });
      return out.length > 0;
    },

    async share(
      pipelineId: string,
      ownerId: string,
      shareWithUserId: string,
      permission: "view" | "edit" = "view",
    ): Promise<boolean> {
      // Verify caller owns the pipeline first.
      const owns = await db
        .select()
        .from(pipelines)
        .where(
          and(eq(pipelines.id, pipelineId), eq(pipelines.ownerId, ownerId)),
        )
        .limit(1);
      if (owns.length === 0) return false;
      await db
        .insert(pipelineShares)
        .values({ pipelineId, userId: shareWithUserId, permission })
        .onConflictDoUpdate({
          target: [pipelineShares.pipelineId, pipelineShares.userId],
          set: { permission },
        });
      return true;
    },

    async unshare(
      pipelineId: string,
      ownerId: string,
      userId: string,
    ): Promise<boolean> {
      const owns = await db
        .select()
        .from(pipelines)
        .where(
          and(eq(pipelines.id, pipelineId), eq(pipelines.ownerId, ownerId)),
        )
        .limit(1);
      if (owns.length === 0) return false;
      await db
        .delete(pipelineShares)
        .where(
          and(
            eq(pipelineShares.pipelineId, pipelineId),
            eq(pipelineShares.userId, userId),
          ),
        );
      return true;
    },
  };
}
