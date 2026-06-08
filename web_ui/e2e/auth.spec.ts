import { test, expect } from "@playwright/test";
import { makeUser } from "./helpers/users";
import { signUpViaUi, signInViaUi, signOutViaUi } from "./helpers/auth";

/**
 * Phase G scenario 1: Sign up → (email verify) → log in → access a protected
 * page → log out (B.2).
 *
 * NOTE: email verification is currently disabled server-side
 * (`requireEmailVerification: false` in server/auth.ts), so sign-up lands the
 * user straight in. When an email transport is wired up, insert the
 * verify-email step here (consume the token from the mail stub / DB).
 */
test.describe("auth", () => {
  test("sign up, reach a protected page, log out, log back in", async ({
    page,
  }) => {
    const user = makeUser("auth");

    // Sign up → redirected to the authenticated home.
    await signUpViaUi(page, user);
    await expect(page).toHaveURL(/\/$/);
    // The header (only rendered when authed) shows the Sign out button.
    await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();

    // Protected page is reachable.
    await page.goto("/pipelines");
    await expect(page).toHaveURL(/\/pipelines/);
    await expect(page.getByRole("heading", { name: "Pipelines" })).toBeVisible();

    // Log out → bounced to /login.
    await signOutViaUi(page);

    // A protected route now redirects to /login.
    await page.goto("/pipelines");
    await expect(page).toHaveURL(/\/login/);

    // Log back in.
    await signInViaUi(page, user);
    await expect(page).toHaveURL(/\/$/);
    await page.goto("/pipelines");
    await expect(page.getByRole("heading", { name: "Pipelines" })).toBeVisible();
  });

  test("unauthenticated users are redirected to /login", async ({ page }) => {
    await page.goto("/data");
    await expect(page).toHaveURL(/\/login/);
  });
});
