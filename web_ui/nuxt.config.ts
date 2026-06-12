// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: "2024-04-03",
  devtools: { enabled: true },
  modules: ["@nuxt/ui", "@nuxtjs/leaflet", "@pinia/nuxt"],
  ssr: false,
  css: ["~/assets/css/fonts.css", "~/assets/css/app.css"],
  colorMode: {
    preference: "light",
    fallback: "light",
  },
  runtimeConfig: {
    // Database
    databaseUrl: process.env.DATABASE_URL || "",
    // Better Auth
    betterAuthSecret: process.env.BETTER_AUTH_SECRET || "",
    betterAuthUrl: process.env.BETTER_AUTH_URL || "http://localhost:3000",
    // S3 / MinIO
    s3Endpoint: process.env.S3_ENDPOINT || "http://localhost:9000",
    s3Region: process.env.S3_REGION || "us-east-1",
    s3AccessKeyId: process.env.S3_ACCESS_KEY_ID || "",
    s3SecretAccessKey: process.env.S3_SECRET_ACCESS_KEY || "",
    s3Bucket: process.env.S3_BUCKET || "spade",
    // Shared secret the Go scheduler presents on run-status callbacks
    // (PATCH /api/runs/:id).  Must match SPADE_UI_CALLBACK_SECRET.
    workerCallbackSecret: process.env.WORKER_CALLBACK_SECRET || "",
    public: {
      betterAuthUrl:
        process.env.NUXT_PUBLIC_BETTER_AUTH_URL ||
        process.env.BETTER_AUTH_URL ||
        "http://localhost:3000",
    },
  },
});
