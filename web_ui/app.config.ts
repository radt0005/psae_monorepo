// Nuxt UI v2 reads `ui.primary` and `ui.gray` here to derive its component
// palettes. Both names must match Tailwind color names from tailwind.config.ts.
export default defineAppConfig({
  ui: {
    primary: "primary", // mapped to spade-red shades in tailwind.config.ts
    gray: "neutral",
  },
});
