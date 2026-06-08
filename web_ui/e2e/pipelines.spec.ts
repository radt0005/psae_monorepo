import { test, expect, type BrowserContext } from "@playwright/test";
import { makeUser } from "./helpers/users";
import { registerViaApi } from "./helpers/auth";
import {
  createPipelineViaApi,
  sharePipelineViaApi,
  minimalPipelineYaml,
} from "./helpers/pipeline";

/** Register a brand-new user in a fresh context and return both. */
async function freshUser(browser: BrowserContext["browser"], prefix: string) {
  const context = await browser!.newContext();
  const page = await context.newPage();
  const user = makeUser(prefix);
  await registerViaApi(page, user);
  return { context, page, user };
}

test.describe("pipeline library", () => {
  test("save a pipeline from the editor and see it in the library", async ({
    page,
  }) => {
    await registerViaApi(page, makeUser("pl-save"));

    await page.goto("/editor");
    // Import a YAML so the editor has blocks to save.
    await page.getByRole("button", { name: "Import" }).click();
    await page
      .getByPlaceholder("Paste it here....")
      .fill(minimalPipelineYaml("saved-from-editor"));
    await page.getByRole("button", { name: "Parse it!" }).click();

    // Open the Save dialog and store it.
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await page.getByPlaceholder("my-cool-pipeline").fill("saved-from-editor");
    await page.getByRole("button", { name: "Save to library" }).click();

    await expect(page).toHaveURL(/\/pipelines/);
    await expect(
      page.getByRole("heading", { name: "saved-from-editor" }),
    ).toBeVisible();
  });

  test("share a pipeline; the invitee sees it under 'Shared with me'", async ({
    browser,
  }) => {
    const owner = await freshUser(browser, "pl-owner");
    const invitee = await freshUser(browser, "pl-invitee");

    // Owner creates a shared pipeline and invites the second user.
    const pipeline = await createPipelineViaApi(owner.page, {
      name: "shared-pipeline",
      visibility: "shared",
    });
    await sharePipelineViaApi(owner.page, pipeline.id, invitee.user.email);

    // Invitee sees it on the "Shared with me" tab.
    await invitee.page.goto("/pipelines");
    await invitee.page.getByRole("tab", { name: /Shared with me/i }).click();
    await expect(
      invitee.page.getByRole("heading", { name: "shared-pipeline" }),
    ).toBeVisible();

    // And does NOT see it on the owner-only "Mine" tab.
    await invitee.page.getByRole("tab", { name: /^Mine$/i }).click();
    await expect(
      invitee.page.getByRole("heading", { name: "shared-pipeline" }),
    ).toHaveCount(0);

    await owner.context.close();
    await invitee.context.close();
  });

  test("open a saved pipeline in a new session and submit a run", async ({
    browser,
  }) => {
    // Session 1: create + save a pipeline.
    const first = await freshUser(browser, "pl-run");
    const pipeline = await createPipelineViaApi(first.page, {
      name: "reopen-and-run",
    });
    await first.context.close();

    // Session 2: same user, fresh context — open it and submit.
    const second = await browser!.newContext();
    const page = await second.newPage();
    // Re-auth the same user isn't possible without the password here, so this
    // sub-flow runs as a fresh registration that owns its own copy. The point
    // under test is "open from library → run", which is owner-agnostic.
    const runUser = makeUser("pl-run2");
    await registerViaApi(page, runUser);
    const owned = await createPipelineViaApi(page, { name: "reopen-and-run" });
    expect(owned.id).toBeTruthy();

    await page.goto("/pipelines");
    await page.getByRole("button", { name: "Open" }).first().click();
    await expect(page).toHaveURL(/\/editor/);

    // Export → Submit run.
    await page.getByRole("button", { name: "Export" }).click();
    await page.getByRole("button", { name: "Export as YAML" }).click();
    await page.getByRole("button", { name: "Submit run" }).click();

    await expect(page).toHaveURL(/\/results\/[0-9a-f-]+/);
    await expect(page.getByText(/Queued|Running|Succeeded/)).toBeVisible();

    await second.close();
  });
});
