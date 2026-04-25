import type { H3Event } from "h3";
import { useAuth } from "~/server/auth";

/**
 * Look up the authenticated user from the request. Returns the user record
 * or throws a 401 if no valid session is present.
 *
 * Use in any protected endpoint:
 *   const user = await requireUser(event);
 */
export async function requireUser(event: H3Event) {
  const auth = useAuth();
  const session = await auth.api.getSession({
    headers: event.headers,
  });
  if (!session?.user) {
    throw createError({ statusCode: 401, statusMessage: "Not authenticated" });
  }
  return session.user;
}

export async function getUserOrNull(event: H3Event) {
  const auth = useAuth();
  const session = await auth.api.getSession({
    headers: event.headers,
  });
  return session?.user ?? null;
}
