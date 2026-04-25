import { useAuth } from "~/server/auth";

/**
 * Better Auth ships its handlers as a single `fetch` function. Mount it
 * here so /api/auth/* (sign-in, sign-up, sessions, OAuth callbacks) all
 * route through the library.
 */
export default defineEventHandler(async (event) => {
  const auth = useAuth();
  return auth.handler(toWebRequest(event));
});
