import { test, expect } from "@playwright/test";
import { makeUser } from "./helpers/users";
import { registerViaApi } from "./helpers/auth";
import { submitRunViaApi } from "./helpers/pipeline";
import { uploadObject } from "./helpers/storage";
import { workerCallback } from "./helpers/worker";

/**
 * Phase G scenarios 6 & 7: browse/download results, share a run.
 *
 * The Go worker isn't running in e2e, so we play its part via the
 * `PATCH /api/runs/:id` callback (helpers/worker.ts). To make downloads real,
 * we first PUT actual bytes to MinIO (helpers/storage.ts) and point the run's
 * output file at that same object key.
 */
test.describe("results", () => {
  test("a completed run shows files + logs, single download and ZIP work", async ({
    page,
  }) => {
    await registerViaApi(page, makeUser("res-dl"));

    // Submit a run (queued), then seed a real output object in MinIO.
    const run = await submitRunViaApi(page, { name: "downloadable-run" });
    const csv = "id,value\n1,42\n2,99\n";
    const { s3Key } = await uploadObject(page, {
      name: "result.csv",
      mimeType: "text/csv",
      body: csv,
    });

    // Worker reports success with that file + a log line.
    await workerCallback(page, run.id, {
      status: "succeeded",
      files: [
        {
          name: "result.csv",
          s3Key,
          blockId: "blk-1",
          mimeType: "text/csv",
          sizeBytes: csv.length,
        },
      ],
      logs: [{ blockId: "blk-1", stdout: "computed 2 rows" }],
    });

    // Detail page reflects the completed run.
    await page.goto(`/results/${run.id}`);
    await expect(page.getByText("Succeeded")).toBeVisible();
    await expect(page.getByText("result.csv")).toBeVisible();
    await expect(page.getByText("computed 2 rows")).toBeVisible();

    // Single-file download: presign endpoint returns a URL that serves bytes.
    const presign = await page.request.get(
      `/api/runs/${run.id}/files/result.csv`,
    );
    expect(presign.ok()).toBeTruthy();
    const { url } = await presign.json();
    const fileRes = await page.request.get(url);
    expect(fileRes.ok()).toBeTruthy();
    expect(await fileRes.text()).toBe(csv);

    // Download-all ZIP streams an application/zip body.
    const zip = await page.request.get(`/api/runs/${run.id}/zip`);
    expect(zip.ok()).toBeTruthy();
    expect(zip.headers()["content-type"]).toContain("application/zip");
    const zipBody = await zip.body();
    expect(zipBody.length).toBeGreaterThan(0);
    // ZIP local-file-header magic number "PK\x03\x04".
    expect(zipBody.subarray(0, 4)).toEqual(Buffer.from([0x50, 0x4b, 0x03, 0x04]));
  });

  test("a failed run surfaces the error", async ({ page }) => {
    await registerViaApi(page, makeUser("res-fail"));
    const run = await submitRunViaApi(page, { name: "failing-run" });
    await workerCallback(page, run.id, {
      status: "failed",
      error: "block exited with code 1",
      logs: [{ blockId: "blk-1", stderr: "boom" }],
    });

    await page.goto(`/results/${run.id}`);
    await expect(page.getByText("Failed")).toBeVisible();
    await expect(page.getByText("block exited with code 1")).toBeVisible();
  });

  test("share a run; the invitee can view it", async ({ browser }) => {
    const ownerCtx = await browser.newContext();
    const owner = await ownerCtx.newPage();
    await registerViaApi(owner, makeUser("run-owner"));

    const inviteeCtx = await browser.newContext();
    const invitee = await inviteeCtx.newPage();
    const inviteeUser = makeUser("run-invitee");
    await registerViaApi(invitee, inviteeUser);

    // Owner runs something and the worker completes it.
    const run = await submitRunViaApi(owner, { name: "shared-run" });
    await workerCallback(owner, run.id, { status: "succeeded" });

    // Owner shares it via the run detail page UI.
    await owner.goto(`/results/${run.id}`);
    await owner.getByPlaceholder("Share with email…").fill(inviteeUser.email);
    await owner.getByRole("button", { name: "Share", exact: true }).click();
    await expect(owner.getByText(/Shared with/)).toBeVisible();

    // Invitee sees it under "Shared with me" and can open the detail page.
    await invitee.goto("/results");
    await invitee.getByRole("tab", { name: /Shared with me/i }).click();
    await expect(invitee.getByText("shared-run")).toBeVisible();
    await invitee.getByText("shared-run").click();
    await expect(invitee).toHaveURL(new RegExp(`/results/${run.id}`));
    await expect(invitee.getByText("Succeeded")).toBeVisible();

    await ownerCtx.close();
    await inviteeCtx.close();
  });
});
