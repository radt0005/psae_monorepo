import PocketBase from "pocketbase";

export default function usePB() {
  return useState("pb", () => {
    const config = useRuntimeConfig();
    const url = config.public.pocketbaseUrl as string;
    if (!url) {
      console.warn(
        "[usePB] NUXT_PUBLIC_POCKETBASE_URL (or POCKETBASE_URL) is not set; PocketBase calls will fail.",
      );
    }
    return new PocketBase(url || "http://localhost:8090");
  });
}
