import { betterAuth } from "better-auth";
import { drizzleAdapter } from "better-auth/adapters/drizzle";
import { deviceAuthorization } from "better-auth/plugins";
import { useDb } from "./db";
import { users, sessions, accounts, verifications } from "./db/schema";

/**
 * Build the Better Auth instance. Factored out so `cached` infers the exact
 * `Auth<…concrete options…>` type — `Auth` is invariant in its options param,
 * so the defaulted `ReturnType<typeof betterAuth>` (`Auth<BetterAuthOptions>`)
 * wouldn't accept it.
 */
function createAuth() {
  const config = useRuntimeConfig();
  const db = useDb();

  return betterAuth({
    secret: (config.betterAuthSecret as string) || process.env.BETTER_AUTH_SECRET,
    baseURL: (config.betterAuthUrl as string) || "http://localhost:3000",
    trustedOrigins: [ (config.betterAuthUrl as string) || "http://localhost:3000"],
    // Map Better Auth's model names (singular) to our drizzle table objects.
    // Our tables are exported under plural keys (`users`, `sessions`, …), which
    // the adapter can't resolve on its own — so pass the schema explicitly.
    database: drizzleAdapter(db, {
      provider: "pg",
      schema: {
        user: users,
        session: sessions,
        account: accounts,
        verification: verifications,
      },
    }),
    plugins: [
      deviceAuthorization({
        verificationUri: "/device",
        validateClient: async (clientId) => {
          console.log(`[device-auth] authorize request client_id=${clientId}`);
          return clientId === "spade-cli";
        },
      }),
    ],
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
}

let cached: ReturnType<typeof createAuth> | null = null;

/**
 * Server-side Better Auth instance, lazy so the DB pool isn't opened at
 * import time.
 */
export function useAuth() {
  if (!cached) cached = createAuth();
  return cached;
}
