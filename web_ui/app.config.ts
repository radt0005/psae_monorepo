// Nuxt UI v2 reads `ui.primary` and `ui.gray` here to derive its component
// palettes. Both names must match Tailwind color names from tailwind.config.ts.
export default defineAppConfig({
  ui: {
    primary: "brand", // seeds --color-primary-* from the `brand` palette (spade red) in tailwind.config.ts
    gray: "neutral",

    button: {
      // A plain <UButton> (no `color`) defaults to color="primary" +
      // variant="solid", whose template hard-codes `text-white bg-{color}-500`.
      // When that background doesn't resolve, the label is white on the white
      // card. Default unspecified buttons to the neutral "white" color and give
      // it a guaranteed-legible black outline + black text (plain spade
      // utilities, no dependency on the primary palette). Buttons that pass an
      // explicit color (primary CTAs, red destructive actions, gray toggles)
      // are unaffected.
      default: { color: "white" },
      color: {
        white: {
          solid:
            "shadow-sm ring-1 ring-inset ring-spade-black text-spade-black bg-spade-white hover:bg-spade-gray-light disabled:opacity-60 disabled:cursor-not-allowed aria-disabled:opacity-60 focus-visible:ring-2 focus-visible:ring-spade-black",
        },
      },
    },
  },
});
