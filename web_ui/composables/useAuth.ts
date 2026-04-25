import { createAuthClient } from "better-auth/client";
import { ref, computed, onMounted } from "vue";

let cached: ReturnType<typeof createAuthClient> | null = null;

function getClient() {
  if (cached) return cached;
  const config = useRuntimeConfig();
  const baseURL = (config.public.betterAuthUrl as string) || window.location.origin;
  cached = createAuthClient({ baseURL });
  return cached;
}

/**
 * Browser-side auth wrapper. Provides reactive `user` / `isAuthenticated`
 * plus `signIn`, `signUp`, `signOut`, `refresh` methods that proxy to
 * Better Auth's client. Per-call lazy so SSR-disabled pages don't blow up.
 */
export function useAuth() {
  const state = useState<{ user: any | null; loading: boolean }>("auth", () => ({
    user: null,
    loading: true,
  }));

  const client = process.client ? getClient() : null;

  const refresh = async () => {
    if (!client) return;
    state.value.loading = true;
    try {
      const session = await client.getSession();
      state.value.user = session?.data?.user ?? null;
    } finally {
      state.value.loading = false;
    }
  };

  const signIn = async (email: string, password: string) => {
    if (!client) throw new Error("Auth client unavailable");
    const res = await client.signIn.email({ email, password });
    if (res?.error) throw new Error(res.error.message ?? "Sign-in failed");
    await refresh();
    return res;
  };

  const signUp = async (email: string, password: string, name?: string) => {
    if (!client) throw new Error("Auth client unavailable");
    const res = await client.signUp.email({ email, password, name: name ?? email });
    if (res?.error) throw new Error(res.error.message ?? "Sign-up failed");
    await refresh();
    return res;
  };

  const signOut = async () => {
    if (!client) return;
    await client.signOut();
    state.value.user = null;
  };

  if (process.client && state.value.loading && state.value.user === null) {
    onMounted(() => {
      void refresh();
    });
  }

  return {
    user: computed(() => state.value.user),
    loading: computed(() => state.value.loading),
    isAuthenticated: computed(() => !!state.value.user),
    signIn,
    signUp,
    signOut,
    refresh,
  };
}
