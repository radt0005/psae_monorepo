/**
 * One-shot migration: copy block records from the old PocketBase `blocks`
 * collection into the new Postgres `blocks` table.
 *
 * Run once after Postgres is provisioned and before deleting the
 * PocketBase blocks collection. Idempotent — uses upsert by name.
 *
 *   POCKETBASE_URL=... POCKETBASE_USER=... POCKETBASE_PASSWORD=... \
 *   DATABASE_URL=... bun scripts/migrate-blocks-from-pocketbase.ts
 */
import "dotenv/config";
import PocketBase from "pocketbase";
import { drizzle } from "drizzle-orm/bun-sql";
import { SQL } from "bun";
import * as schema from "~/server/db/schema";
import { makeBlockRepo } from "~/server/db/repositories/blocks";
import type { BlockManifest } from "~/utils/types";

async function main() {
  const dbUrl = process.env.DATABASE_URL;
  const pbUrl = process.env.POCKETBASE_URL;
  const pbUser = process.env.POCKETBASE_USER;
  const pbPassword = process.env.POCKETBASE_PASSWORD;

  if (!dbUrl) throw new Error("DATABASE_URL required");
  if (!pbUrl || !pbUser || !pbPassword) {
    throw new Error("POCKETBASE_URL/USER/PASSWORD required");
  }

  const pb = new PocketBase(pbUrl);
  await pb.admins.authWithPassword(pbUser, pbPassword);
  const records = await pb.collection("blocks").getFullList();

  const sql = new SQL(dbUrl);
  const db = drizzle(sql, { schema });
  const repo = makeBlockRepo(db);

  let copied = 0;
  for (const record of records) {
    const manifest = (record as any).manifest as BlockManifest | undefined;
    if (!manifest) {
      console.warn(`Skipping block ${record.name} — no manifest field.`);
      continue;
    }
    await repo.upsert({
      label: (record as any).label ?? record.name,
      manifest,
    });
    copied++;
  }

  console.log(`Migrated ${copied} block(s).`);
  await sql.end();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
