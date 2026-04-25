import { and, desc, eq, inArray } from "drizzle-orm";
import {
  dataFiles,
  dataFileShares,
  type DataFile,
  type NewDataFile,
} from "../schema/dataFiles";
import type { Db } from "../index";

export type DataFileRepo = ReturnType<typeof makeDataFileRepo>;

export function makeDataFileRepo(db: Db) {
  return {
    async listOwnedBy(userId: string): Promise<DataFile[]> {
      return db
        .select()
        .from(dataFiles)
        .where(eq(dataFiles.ownerId, userId))
        .orderBy(desc(dataFiles.createdAt));
    },

    async listSharedWith(userId: string): Promise<DataFile[]> {
      const shares = await db
        .select({ dataFileId: dataFileShares.dataFileId })
        .from(dataFileShares)
        .where(eq(dataFileShares.userId, userId));
      const ids = shares.map((s) => s.dataFileId);
      if (ids.length === 0) return [];
      return db
        .select()
        .from(dataFiles)
        .where(inArray(dataFiles.id, ids))
        .orderBy(desc(dataFiles.createdAt));
    },

    async listPublic(): Promise<DataFile[]> {
      return db
        .select()
        .from(dataFiles)
        .where(eq(dataFiles.visibility, "public"))
        .orderBy(desc(dataFiles.createdAt));
    },

    async getByIdForUser(
      id: string,
      userId: string,
    ): Promise<DataFile | null> {
      const row = (
        await db.select().from(dataFiles).where(eq(dataFiles.id, id)).limit(1)
      )[0];
      if (!row) return null;
      if (row.ownerId === userId || row.visibility === "public") return row;
      const share = (
        await db
          .select()
          .from(dataFileShares)
          .where(
            and(
              eq(dataFileShares.dataFileId, id),
              eq(dataFileShares.userId, userId),
            ),
          )
          .limit(1)
      )[0];
      return share ? row : null;
    },

    async create(input: NewDataFile): Promise<DataFile> {
      const [row] = await db.insert(dataFiles).values(input).returning();
      return row;
    },

    async delete(id: string, ownerId: string): Promise<DataFile | null> {
      const out = await db
        .delete(dataFiles)
        .where(and(eq(dataFiles.id, id), eq(dataFiles.ownerId, ownerId)))
        .returning();
      return out[0] ?? null;
    },

    async share(
      dataFileId: string,
      ownerId: string,
      shareWithUserId: string,
      permission: "view" | "edit" = "view",
    ): Promise<boolean> {
      const owns = await db
        .select()
        .from(dataFiles)
        .where(
          and(eq(dataFiles.id, dataFileId), eq(dataFiles.ownerId, ownerId)),
        )
        .limit(1);
      if (owns.length === 0) return false;
      await db
        .insert(dataFileShares)
        .values({ dataFileId, userId: shareWithUserId, permission })
        .onConflictDoUpdate({
          target: [dataFileShares.dataFileId, dataFileShares.userId],
          set: { permission },
        });
      return true;
    },
  };
}
