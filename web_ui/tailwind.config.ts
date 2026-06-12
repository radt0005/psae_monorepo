import type { Config } from "tailwindcss";

/**
 * Tokens ported verbatim from ../website/sass/_variables.scss and
 * ../documentation/sass/_variables.scss (the two Zola sites already share an
 * identical palette/typography). Brand semantic names — never hard-code hex
 * in components.
 */
export default <Config>{
  content: [],
  theme: {
    extend: {
      colors: {
        spade: {
          black: "#1a1a1a",
          white: "#ffffff",
          red: {
            DEFAULT: "#c0392b",
            light: "#e74c3c",
            dark: "#96281b",
          },
          gray: {
            light: "#f5f5f5",
            DEFAULT: "#888888",
            dark: "#333333",
          },
          blue: {
            DEFAULT: "#2980b9",
            light: "#eaf2f8",
          },
          green: {
            DEFAULT: "#27ae60",
            light: "#eafaf1",
          },
          yellow: {
            DEFAULT: "#f39c12",
            light: "#fef9e7",
          },
        },
        // Source palette for @nuxt/ui's `primary`. Must NOT be named
        // "primary": @nuxt/ui redefines the `primary` Tailwind color as
        // rgb(var(--color-primary-*)) and then seeds those vars from the color
        // named by app.config `ui.primary`. Pointing it at a color literally
        // named "primary" is self-referential — the seed reads the var-based
        // values (no hex), the vars stay empty, and every primary button renders
        // white text on a transparent (white) background. Naming it "brand" and
        // setting ui.primary: "brand" gives the seeder real hex to convert.
        brand: {
          50: "#fdedec",
          100: "#fadbd8",
          200: "#f5b7b1",
          300: "#f1948a",
          400: "#ec7063",
          500: "#e74c3c", // red-light
          600: "#c0392b", // red (default)
          700: "#a93226",
          800: "#96281b", // red-dark
          900: "#7b241c",
          950: "#641e16",
        },
      },
      fontFamily: {
        heading: [
          "Playfair Display",
          "Georgia",
          "ui-serif",
          "serif",
        ],
        body: [
          "Inter",
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "ui-sans-serif",
          "system-ui",
          "sans-serif",
        ],
        mono: [
          "JetBrains Mono",
          "Fira Code",
          "Consolas",
          "ui-monospace",
          "monospace",
        ],
      },
      fontSize: {
        // Marketing-site heading scale (h1 3.5rem … h6 1rem)
        "display-1": ["3.5rem", { lineHeight: "1.2", fontWeight: "700" }],
        "display-2": ["2.5rem", { lineHeight: "1.2", fontWeight: "700" }],
        "display-3": ["1.75rem", { lineHeight: "1.2", fontWeight: "700" }],
        "display-4": ["1.25rem", { lineHeight: "1.2", fontWeight: "700" }],
      },
      spacing: {
        // _variables.scss spacing scale
        "spade-xs": "0.25rem",
        "spade-sm": "0.5rem",
        "spade-md": "1rem",
        "spade-lg": "2rem",
        "spade-xl": "3rem",
        "spade-xxl": "5rem",
      },
      borderRadius: {
        "spade-sm": "4px",
        "spade-md": "8px",
        "spade-lg": "16px",
      },
      boxShadow: {
        card: "0 2px 8px rgba(0, 0, 0, 0.1)",
        "card-hover": "0 6px 20px rgba(0, 0, 0, 0.15)",
      },
      screens: {
        "spade-mobile": "480px",
        "spade-tablet": "768px",
        "spade-desktop": "1024px",
        "spade-wide": "1280px",
      },
    },
  },
};
