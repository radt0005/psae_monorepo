import PocketBase from "pocketbase";

let cached: PocketBase | null = null;

export const useServerPocketbase = () => {
  if (cached) return cached;
  const config = useRuntimeConfig();
  const url = (config.pocketbaseUrl as string) || process.env.POCKETBASE_URL;
  if (!url) {
    throw new Error(
      "POCKETBASE_URL is not configured (set NUXT_POCKETBASE_URL / POCKETBASE_URL).",
    );
  }
  const pb = new PocketBase(url);
  const adminUser =
    (config.pocketbaseAdminUser as string) || process.env.POCKETBASE_USER;
  const adminPassword =
    (config.pocketbaseAdminPassword as string) ||
    process.env.POCKETBASE_PASSWORD;
  if (adminUser && adminPassword) {
    pb.admins.authWithPassword(adminUser, adminPassword).catch((err) => {
      console.error("[useServerPocketbase] admin auth failed", err);
    });
  }
  cached = pb;
  return pb;
};
