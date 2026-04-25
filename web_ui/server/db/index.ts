import { drizzle } from "drizzle-orm/bun-sql";
import { SQL } from "bun";
import * as schema from "./schema";

let cached: ReturnType<typeof drizzle<typeof schema>> | null = null;

/**
 * Lazy-initialised Drizzle handle. Uses Bun's native `Bun.sql` driver per
 * IMPLEMENTATION_PLAN B.1. Cached so we don't open a fresh pool per request.
 */
export function useDb() {
  if (cached) return cached;
  const config = useRuntimeConfig();
  const url = (config.databaseUrl as string) || process.env.DATABASE_URL;
  if (!url) {
    throw new Error(
      "DATABASE_URL is not configured (set it in .env or Nuxt runtimeConfig).",
    );
  }
  const sql = new SQL(url);
  cached = drizzle(sql, { schema });
  return cached;
}

export type Db = ReturnType<typeof useDb>;
export { schema };
