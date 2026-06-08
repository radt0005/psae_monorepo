import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import { fileURLToPath } from "node:url";

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: "happy-dom",
    globals: false,
    include: ["tests/**/*.test.ts"],
    // PGlite (Postgres-in-WASM) cold start in the DB repository tests can
    // exceed the 5s default on first spin-up; give them headroom.
    testTimeout: 30000,
    hookTimeout: 30000,
  },
  resolve: {
    alias: {
      "~": fileURLToPath(new URL("./", import.meta.url)),
      "@": fileURLToPath(new URL("./", import.meta.url)),
    },
  },
});
