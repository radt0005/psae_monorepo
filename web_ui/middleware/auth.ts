/**
 * Route guard. Redirects unauthenticated users to /login (except for
 * the auth-route allowlist).
 */
const PUBLIC_ROUTES = new Set([
  "/login",
  "/signup",
  "/forgot-password",
  "/confirm",
]);

export default defineNuxtRouteMiddleware(async (to) => {
  if (PUBLIC_ROUTES.has(to.path)) return;

  const { isAuthenticated, loading, refresh } = useAuth();
  if (loading.value && !isAuthenticated.value) {
    await refresh();
  }
  if (!isAuthenticated.value) {
    return navigateTo("/login", { redirectCode: 401 });
  }
});
