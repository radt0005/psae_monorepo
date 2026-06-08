import { expect, type Page } from "@playwright/test";
import type { TestUser } from "./users";

/**
 * Auth helpers. Two flavours:
 *  - UI flows (`signUpViaUi` / `signInViaUi`) drive the actual forms — used by
 *    the auth spec that's testing those screens.
 *  - `registerViaApi` hits Better Auth's HTTP endpoint to set up a logged-in
 *    session fast, for specs whose subject is something *other* than the login
 *    screen. It runs against the page's own cookie jar, so the page is
 *    authenticated afterwards.
 */

// UFormGroup doesn't reliably associate <label for> with the UInput, so we
// target inputs by stable attributes (type/autocomplete) instead of getByLabel.
export async function signUpViaUi(page: Page, user: TestUser) {
  await page.goto("/signup");
  await page.locator('input[autocomplete="name"]').fill(user.name);
  await page.locator('input[type="email"]').fill(user.email);
  const pw = page.locator('input[type="password"]');
  await pw.nth(0).fill(user.password); // Password
  await pw.nth(1).fill(user.password); // Confirm password
  await page.getByRole("button", { name: "Create account" }).click();
}

export async function signInViaUi(page: Page, user: TestUser) {
  await page.goto("/login");
  await page.locator('input[type="email"]').fill(user.email);
  await page.locator('input[type="password"]').fill(user.password);
  await page.getByRole("button", { name: "Sign in" }).click();
}

export async function signOutViaUi(page: Page) {
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/login/);
}

/**
 * Create a user + authenticated session via the Better Auth HTTP API using the
 * page's cookie jar. After this resolves, `page` is logged in as `user`.
 */
export async function registerViaApi(page: Page, user: TestUser) {
  const res = await page.request.post("/api/auth/sign-up/email", {
    data: { email: user.email, password: user.password, name: user.name },
  });
  if (!res.ok()) {
    throw new Error(
      `sign-up failed (${res.status()}): ${await res.text()}`,
    );
  }
  return res;
}

/** Sign in an existing user via the API on the page's cookie jar. */
export async function signInViaApi(page: Page, user: TestUser) {
  const res = await page.request.post("/api/auth/sign-in/email", {
    data: { email: user.email, password: user.password },
  });
  if (!res.ok()) {
    throw new Error(`sign-in failed (${res.status()}): ${await res.text()}`);
  }
  return res;
}

/** The id of the currently-authenticated user (via Better Auth session). */
export async function currentUserId(page: Page): Promise<string> {
  const res = await page.request.get("/api/auth/get-session");
  const body = await res.json();
  const id = body?.user?.id ?? body?.session?.userId;
  if (!id) throw new Error("No authenticated session");
  return id;
}
