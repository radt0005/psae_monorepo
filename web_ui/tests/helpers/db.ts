import { PGlite } from "@electric-sql/pglite";
import { drizzle } from "drizzle-orm/pglite";
import * as schema from "~/server/db/schema";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

/**
 * Spin up an embedded Postgres (PGlite, real Postgres in WASM) with the
 * project schema applied. Used by repository tests so they don't need a
 * running container. Production code uses Bun's native `Bun.sql` driver —
 * both speak the same Drizzle API, so the repository functions work
 * against either.
 */
export async function makeTestDb() {
  const client = new PGlite();
  await client.waitReady;

  const migrationSql = fs.readFileSync(
    path.resolve(__dirname, "../../server/db/migrations/0000_hot_odin.sql"),
    "utf8",
  );
  for (const stmt of migrationSql.split("--> statement-breakpoint")) {
    const trimmed = stmt.trim();
    if (trimmed.length === 0) continue;
    await client.exec(trimmed);
  }

  const db = drizzle(client, { schema });
  return { db, client };
}

export type TestDb = Awaited<ReturnType<typeof makeTestDb>>["db"];
