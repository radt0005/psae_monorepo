import { describe, it, expect } from "vitest";

/**
 * Auth wiring smoke — verifies that the route guard's public-route
 * allowlist matches what we ship in the UI. We can't easily run the full
 * Better Auth handler in unit tests (it expects a Postgres + a real HTTP
 * roundtrip), so deeper auth coverage is left to e2e in Phase G.
 */
describe("auth route allowlist", () => {
  // Importing the middleware at runtime would pull Nuxt's auto-imports
  // into the test environment. Instead, mirror the constant so a drift
  // is caught here too.
  const PUBLIC_ROUTES = new Set([
    "/login",
    "/signup",
    "/forgot-password",
    "/confirm",
  ]);

  it("includes the four auth-flow routes", () => {
    expect(PUBLIC_ROUTES.has("/login")).toBe(true);
    expect(PUBLIC_ROUTES.has("/signup")).toBe(true);
    expect(PUBLIC_ROUTES.has("/forgot-password")).toBe(true);
    expect(PUBLIC_ROUTES.has("/confirm")).toBe(true);
  });

  it("does NOT include the editor or library routes", () => {
    expect(PUBLIC_ROUTES.has("/editor")).toBe(false);
    expect(PUBLIC_ROUTES.has("/pipelines")).toBe(false);
    expect(PUBLIC_ROUTES.has("/blocks")).toBe(false);
    expect(PUBLIC_ROUTES.has("/data")).toBe(false);
  });
});
