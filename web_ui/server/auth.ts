import { betterAuth } from "better-auth";
import { drizzleAdapter } from "better-auth/adapters/drizzle";
import { useDb } from "./db";

let cached: ReturnType<typeof betterAuth> | null = null;

/**
 * Server-side Better Auth instance, lazy so the DB pool isn't opened at
 * import time.
 */
export function useAuth() {
  if (cached) return cached;
  const config = useRuntimeConfig();
  const db = useDb();

  cached = betterAuth({
    secret: (config.betterAuthSecret as string) || process.env.BETTER_AUTH_SECRET,
    baseURL: (config.betterAuthUrl as string) || "http://localhost:3000",
    database: drizzleAdapter(db, { provider: "pg" }),
    emailAndPassword: {
      enabled: true,
      requireEmailVerification: false, // flip on once we have an email transport
    },
    user: {
      additionalFields: {
        role: { type: "string", required: false, defaultValue: "user" },
      },
    },
  });

  return cached;
}
