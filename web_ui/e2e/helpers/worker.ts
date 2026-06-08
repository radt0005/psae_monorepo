import { expect, type Page } from "@playwright/test";

/**
 * Stand-in for the Go worker/scheduler. Drives the `PATCH /api/runs/:id`
 * callback (worker.md §"Result reporting"), authenticated by the shared
 * WORKER_CALLBACK_SECRET, so result-browsing/sharing flows can be exercised
 * end-to-end without a live worker.
 *
 * The same secret must be set in the web server's env
 * (WORKER_CALLBACK_SECRET) — see e2e/README.md.
 */
const SECRET = process.env.WORKER_CALLBACK_SECRET || "";

export interface WorkerFile {
  name: string;
  s3Key: string;
  blockId?: string;
  mimeType?: string;
  sizeBytes?: number;
}

export interface WorkerCallback {
  status?: "running" | "succeeded" | "failed" | "canceled";
  error?: string;
  files?: WorkerFile[];
  logs?: Array<{ blockId?: string; stdout?: string; stderr?: string }>;
}

export async function workerCallback(
  page: Page,
  runId: string,
  payload: WorkerCallback,
) {
  if (!SECRET) {
    throw new Error(
      "WORKER_CALLBACK_SECRET is not set in the e2e environment — see e2e/README.md",
    );
  }
  const res = await page.request.patch(`/api/runs/${runId}`, {
    headers: { Authorization: `Bearer ${SECRET}` },
    data: payload,
  });
  expect(
    res.ok(),
    `worker callback: ${res.status()} ${await res.text()}`,
  ).toBeTruthy();
  return res.json();
}
