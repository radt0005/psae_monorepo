import { defineConfig, devices } from "@playwright/test";

/**
 * Phase G end-to-end suite (see IMPLEMENTATION_PLAN.md §"Phase G").
 *
 * These tests drive the real Nuxt app against the real Postgres + MinIO +
 * RabbitMQ stack. Unlike the vitest unit tests (which use PGlite in-process),
 * they need the full backend running. See e2e/README.md for the one-time
 * setup. The unit runner ignores this directory (vitest only globs the
 * `tests/` folder for `.test.ts` files, never `e2e/`).
 *
 * The worker is simulated via the `PATCH /api/runs/:id` callback (guarded by
 * WORKER_CALLBACK_SECRET) so result-browsing flows can be exercised without a
 * live Go worker — see e2e/helpers/worker.ts.
 */

const BASE_URL = process.env.E2E_BASE_URL || "http://localhost:3000";

export default defineConfig({
  testDir: "./e2e",
  testMatch: "**/*.spec.ts",
  // Pipeline submission + polling can take a beat; keep per-test generous.
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false, // tests share one DB; keep ordering deterministic
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",

  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },

  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],

  /**
   * Start the dev server automatically unless E2E_BASE_URL points at an
   * already-running instance. The backend stack (docker compose up) and
   * migrations must already be up — the web server alone is not enough.
   */
  webServer: process.env.E2E_NO_SERVER
    ? undefined
    : {
        command: "bun run dev",
        url: BASE_URL,
        timeout: 120_000,
        reuseExistingServer: !process.env.CI,
        stdout: "pipe",
        stderr: "pipe",
      },
});
