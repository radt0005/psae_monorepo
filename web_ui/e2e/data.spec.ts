import { test, expect } from "@playwright/test";
import { makeUser } from "./helpers/users";
import { registerViaApi } from "./helpers/auth";
import { uploadObject } from "./helpers/storage";

/**
 * Phase G scenario 5 (partial): upload custom data + share it (B.6).
 * The ">100 MB → use in a pipeline → run completes" leg is split out as a
 * fixme below — see the note there.
 */
test.describe("custom data", () => {
  test("upload a file through the UI and see it listed", async ({ page }) => {
    await registerViaApi(page, makeUser("data-up"));
    await page.goto("/data");

    // The visible button proxies a hidden <input type=file>; drive the input.
    await page.locator('input[type="file"]').setInputFiles({
      name: "sample.csv",
      mimeType: "text/csv",
      buffer: Buffer.from("a,b\n1,2\n3,4\n"),
    });

    // Row appears once /complete resolves and the table refreshes.
    await expect(page.getByText("sample.csv")).toBeVisible();
  });

  test("share a data file; the invitee sees it under 'Shared with me'", async ({
    browser,
  }) => {
    const ownerCtx = await browser.newContext();
    const owner = await ownerCtx.newPage();
    const ownerUser = makeUser("data-owner");
    await registerViaApi(owner, ownerUser);

    const inviteeCtx = await browser.newContext();
    const invitee = await inviteeCtx.newPage();
    const inviteeUser = makeUser("data-invitee");
    await registerViaApi(invitee, inviteeUser);

    // Owner uploads + registers a shared file, then shares it by email.
    const { dataFileId } = await uploadObject(owner, {
      name: "shared-data.csv",
      mimeType: "text/csv",
      body: "x,y\n1,2\n",
      register: true,
      visibility: "shared",
    });
    const shareRes = await owner.request.post(`/api/data/${dataFileId}/share`, {
      data: { email: inviteeUser.email },
    });
    expect(shareRes.ok(), `share data: ${shareRes.status()}`).toBeTruthy();

    // Invitee sees it on the "Shared with me" tab.
    await invitee.goto("/data");
    await invitee.getByRole("tab", { name: /Shared with me/i }).click();
    await expect(invitee.getByText("shared-data.csv")).toBeVisible();

    await ownerCtx.close();
    await inviteeCtx.close();
  });

  /**
   * Phase G scenario 5 (full): upload a >100 MB file, reference it from a
   * pipeline, and confirm the run completes.
   *
   * Held as fixme for two reasons:
   *  1. The init endpoint notes multipart for >100 MB is a follow-up
   *     (server/api/uploads/data/init.post.ts) — a single presigned PUT of
   *     that size is slow/flaky in CI.
   *  2. "Use the custom data in a pipeline" depends on the editor↔data-file
   *     binding (plan B.6, still pending worker wire-format sign-off).
   * Unskip once both land; generate the payload with a streamed buffer rather
   * than allocating 100 MB in memory.
   */
  test.fixme(
    "upload a >100 MB file, use it in a pipeline, run completes",
    async () => {},
  );
});
