# Spade Website Implementation Plan

This plan describes the full implementation of the Spade marketing website using Zola (v0.22.1). The site targets a general audience and explains what the Spade data processing system is and what it can do. The visual theme is inspired by playing cards, featuring a spade logo and a black, white, and red color scheme.

---

## Phase 1: Project Configuration and Foundation

- [DONE] **Update `config.toml`** with site metadata: `title = "Spade"`, `description = "A data processing system for massive data with first-class geospatial support"`, and `default_language = "en"`. Add `[extra]` variables for `site_tagline`, `github_url`, and `docs_url` (pointing to `https://docs.spade.app`).
- [DONE] **Create directory structure** for the project:
  - `templates/` — Zola HTML templates
  - `sass/` — SCSS stylesheets
  - `static/img/` — Images and logo assets
  - `static/fonts/` — Web fonts (if self-hosted)
  - `content/` — Markdown content pages
  - `content/features/` — Section for feature detail pages
  - `content/use-cases/` — Section for use case pages
- [DONE] **Create `static/img/spade-logo.svg`**: An SVG spade symbol (the playing card suit). The spade should be bold and geometric, suitable for use as both a favicon and a header logo. Use black fill with optional red accent.
- [DONE] **Create `static/img/favicon.ico`** (or `favicon.png`): A 32x32 favicon derived from the spade logo.
- [DONE] **Create `static/img/og-image.png`**: A 1200x630 Open Graph image for social media sharing, featuring the spade logo and tagline on a white background with red accent elements.

---

## Phase 2: Base Templates and Layout

- [DONE] **Create `templates/base.html`**: The root layout template containing:
  - `<!DOCTYPE html>` with `lang="en"`
  - `<head>` block with:
    - Meta charset, viewport, description, and Open Graph tags
    - `<title>` using `{% block title %}{{ config.title }}{% endblock %}`
    - Favicon link
    - Stylesheet link to the compiled Sass output (`/style.css`)
    - Optional: Google Fonts link for a clean serif or sans-serif font (e.g., "Playfair Display" for headings, "Inter" for body text, to evoke a card/typographic feel)
  - `<body>` containing:
    - `{% block header %}{% endblock %}`
    - `{% block content %}{% endblock %}`
    - `{% block footer %}{% endblock %}`
  - A `<header>` include with site navigation (logo, nav links: Home, Features, Use Cases, Docs, GitHub)
  - A `<footer>` include with copyright, links to docs and GitHub, and a small spade icon
- [DONE] **Create `templates/index.html`**: Extends `base.html`. This is the landing page template. Contains Zola blocks for each landing page section (hero, features overview, how it works, use cases, CTA). Detailed content structure is defined in Phase 4.
- [DONE] **Create `templates/page.html`**: A generic page template extending `base.html` with a centered content column for markdown-rendered pages (e.g., about, privacy).
- [DONE] **Create `templates/section.html`**: A section listing template extending `base.html` that renders a list of child pages (used by `/features/` and `/use-cases/` sections).
- [DONE] **Create `templates/features/page.html`** (optional): Template for individual feature detail pages if needed, extending `base.html` with a hero banner and markdown body.
- [DONE] **Create `templates/macros/cards.html`**: A Zola macro file containing reusable card components:
  - `card(title, description, icon)` — A playing-card-styled content card with rounded corners, subtle shadow, and a red/black suit icon accent
  - `feature_card(title, description, icon)` — Variant used on the features overview section

---

## Phase 3: Styling (Sass)

- [DONE] **Create `sass/style.scss`**: The main Sass entry point that imports all partials in order:
  ```scss
  @import "variables";
  @import "reset";
  @import "typography";
  @import "layout";
  @import "components";
  @import "hero";
  @import "cards";
  @import "navigation";
  @import "footer";
  @import "responsive";
  ```
- [DONE] **Create `sass/_variables.scss`**: Define the design token system:
  - Colors: `$color-black: #1a1a1a`, `$color-white: #ffffff`, `$color-red: #c0392b` (or similar rich red), `$color-red-light: #e74c3c`, `$color-gray-light: #f5f5f5`, `$color-gray: #888888`
  - Fonts: `$font-heading`, `$font-body`
  - Spacing scale: `$space-xs` through `$space-xxl`
  - Breakpoints: `$bp-mobile: 480px`, `$bp-tablet: 768px`, `$bp-desktop: 1024px`, `$bp-wide: 1280px`
  - Border radius: `$radius-sm: 4px`, `$radius-md: 8px`, `$radius-lg: 16px`
  - Card-specific: `$card-shadow`, `$card-border`
- [DONE] **Create `sass/_reset.scss`**: A minimal CSS reset (box-sizing border-box, margin/padding zero, sensible defaults for images and links).
- [DONE] **Create `sass/_typography.scss`**: Base typography styles:
  - Body: `$font-body`, `$color-black` on `$color-white`, line-height 1.6
  - Headings (h1-h6): `$font-heading`, bold weights, tight line-height
  - Links: `$color-red` with hover underline
  - Code blocks: Monospace font with `$color-gray-light` background
- [DONE] **Create `sass/_layout.scss`**: Layout utility classes:
  - `.container` — max-width 1200px, centered, horizontal padding
  - `.section` — vertical padding for full-width page sections
  - `.grid-2`, `.grid-3`, `.grid-4` — CSS Grid layouts with responsive fallbacks
  - `.flex-center`, `.flex-between` — Flexbox utility classes
- [DONE] **Create `sass/_components.scss`**: Reusable component styles:
  - `.btn` — Base button: padding, border-radius, font-weight, transition
  - `.btn--primary` — Red background, white text
  - `.btn--secondary` — White background, black text, red border
  - `.btn--outline` — Transparent with red border
  - `.tag` — Small label/badge for language names (Python, Go, Rust, etc.)
  - `.divider` — Decorative divider using a small spade suit symbol or a red line
  - `.suit-icon` — Inline playing card suit icons (spade, heart, diamond, club) for decorative use
- [DONE] **Create `sass/_hero.scss`**: Hero section styling:
  - Full-viewport-height hero with white background
  - Large centered heading and tagline
  - Red accent elements (decorative suit symbols in corners, like a playing card)
  - CTA buttons centered below tagline
  - Optional: Subtle animated card-flip or card-fan motif using CSS
- [DONE] **Create `sass/_cards.scss`**: Playing-card-themed content card styling:
  - White background, subtle shadow, rounded corners
  - Top-left and bottom-right corner accents (like card indices) in red or black
  - Hover effect: slight lift/shadow increase
  - Card header with suit icon and title
  - Card body with description text
- [DONE] **Create `sass/_navigation.scss`**: Header navigation styles:
  - Fixed/sticky header, white background, subtle bottom border
  - Spade logo on the left, nav links on the right
  - Mobile: Hamburger menu icon that toggles a slide-down nav
  - Active link indicated with red underline
- [DONE] **Create `sass/_footer.scss`**: Footer styles:
  - Black background, white text
  - Grid layout: logo + tagline | quick links | community links
  - Small spade suit decorative element
  - Bottom bar with copyright
- [DONE] **Create `sass/_responsive.scss`**: Responsive breakpoint overrides:
  - Stack grid columns on mobile
  - Reduce heading sizes on small screens
  - Hamburger menu activation below tablet breakpoint
  - Adjust hero section sizing for mobile

---

## Phase 4: Landing Page Content

- [DONE] **Create `content/_index.md`**: The landing page content file with `template = "index.html"` in frontmatter and the following extra data fields:
  - `tagline`: "Data processing at scale. Built for geospatial."
  - `subtitle`: A one-line description like "Build powerful data pipelines with reusable blocks in any language."
  - Sections defined as frontmatter data arrays (or via template hardcoding — see below)

- [DONE] **Implement Hero Section** in `templates/index.html`:
  - Large spade logo (SVG) centered or left-aligned
  - Main heading: The site tagline from frontmatter
  - Subtitle paragraph explaining Spade in one sentence
  - Two CTA buttons: "Get Started" (links to docs) and "View on GitHub" (links to repo)
  - Decorative playing card corner elements (small suit symbols at corners of the hero area, like the corner indices on a playing card)

- [DONE] **Implement "What is Spade?" Section** in `templates/index.html`:
  - Brief 2-3 paragraph explanation of Spade:
    - A data processing system for massive data with first-class geospatial support
    - Plugin-based architecture: processing steps are independent "blocks" that execute in isolation
    - Build workflows as declarative pipelines (YAML DAGs), executed in parallel by the scheduler
  - Accompanied by a simple visual diagram or illustration (can be CSS/SVG) showing: Data -> Pipeline -> Blocks -> Results

- [DONE] **Implement Features Section** in `templates/index.html`:
  - Section heading: "Deal Yourself a Winning Hand" (or similar card-themed pun)
  - 6 feature cards in a 3x2 grid, each with a playing card suit icon, title, and short description:
    1. **Multi-Language Blocks** — "Write blocks in Python, R, Go, Rust, or TypeScript. Spade handles the rest." (icon: spade)
    2. **Declarative Pipelines** — "Define workflows as YAML. The scheduler resolves dependencies and parallelizes execution." (icon: diamond)
    3. **Geospatial First** — "Native support for raster, vector, and tabular geospatial data through GDAL integration." (icon: heart)
    4. **Map/Reduce at Scale** — "Fan out across collections and reduce results back together, automatically." (icon: club)
    5. **Secure by Default** — "Each block runs in an isolated sandbox with restricted filesystem and network access." (icon: spade)
    6. **CLI + Web UI** — "Develop locally with the CLI. Deploy and collaborate through the web interface." (icon: diamond)

- [DONE] **Implement "How It Works" Section** in `templates/index.html`:
  - Section heading: "How It Works"
  - 3-step visual flow (numbered, left-to-right or top-to-bottom):
    1. **Define** — "Write your processing block as a simple function with typed inputs and outputs."
    2. **Compose** — "Chain blocks into a pipeline using YAML or the visual flowchart editor."
    3. **Execute** — "The scheduler handles dependencies, parallelism, caching, and sandboxed execution."
  - Each step accompanied by a code snippet or visual:
    - Step 1: A small Python code example showing a block handler function
    - Step 2: A small YAML pipeline snippet
    - Step 3: A visual showing parallel block execution

- [DONE] **Implement Code Example Section** in `templates/index.html`:
  - Section heading: "Simple by Design"
  - Side-by-side or tabbed code examples showing block creation in multiple languages:
    - **Python tab**:
      ```python
      from spade import run, RasterFile

      def handler(source: RasterFile) -> RasterFile:
          # Your processing logic here
          return RasterFile(path=result)

      if __name__ == "__main__":
          run(handler)
      ```
    - **TypeScript tab**:
      ```typescript
      import { run, RasterFile } from "spade";

      function handler(source: RasterFile): RasterFile {
        // Your processing logic here
        return new RasterFile(resultPath);
      }

      run(handler);
      ```
  - Brief paragraph below emphasizing: "Type-safe inputs and outputs. Automatic parameter loading. Zero boilerplate."

- [DONE] **Implement Use Cases Section** in `templates/index.html`:
  - Section heading: "Built for Real-World Data"
  - 3 use case cards:
    1. **Remote Sensing** — Processing satellite imagery at scale, including reprojection, tiling, and analysis
    2. **Geospatial ETL** — Extracting, transforming, and loading spatial datasets across formats and coordinate systems
    3. **Scientific Pipelines** — Reproducible research workflows combining data from multiple providers and processing steps
  - Each card has a brief description and a "Learn More" link (can link to `/use-cases/<slug>/`)

- [DONE] **Implement CTA / Getting Started Section** in `templates/index.html`:
  - Section heading: "Ready to Play?"
  - Brief text: "Get started in minutes with the Spade CLI."
  - Terminal-style code block showing:
    ```
    $ spade init python
    $ spade add my-block
    $ spade run pipeline.yaml
    ```
  - Two CTA buttons: "Read the Docs" and "View on GitHub"
  - Decorative card-fan motif or spade watermark behind the section

---

## Phase 5: Content Pages

- [DONE] **Create `content/features/_index.md`**: Section index page for features. Frontmatter: `title = "Features"`, `template = "section.html"`, `sort_by = "weight"`. Brief introductory text.
- [DONE] **Create `content/features/multi-language.md`**: Detailed page on multi-language block support. Weight: 1. Explain supported languages (Python, R, Go, Rust, TypeScript/Bun), how the library handles I/O, and the handler function pattern.
- [DONE] **Create `content/features/pipelines.md`**: Detailed page on declarative pipelines. Weight: 2. Explain YAML pipeline format, block dependencies, input/output wiring (simple and explicit references), and validation (`spade check`).
- [DONE] **Create `content/features/geospatial.md`**: Detailed page on geospatial support. Weight: 3. Explain GDAL block library, supported data types (RasterFile, VectorFile, TabularFile, collections), and data provider integrations (OpenDAL).
- [DONE] **Create `content/features/map-reduce.md`**: Detailed page on map/reduce. Weight: 4. Explain map blocks, expansion manifests, parallel invocation, reduce blocks, and map context propagation.
- [DONE] **Create `content/features/security.md`**: Detailed page on security model. Weight: 5. Explain sandbox execution (isolate), filesystem restrictions, network access declaration, content hash verification, and trust model.
- [DONE] **Create `content/features/cli.md`**: Detailed page on the CLI. Weight: 6. Explain `spade init`, `spade add`, `spade check`, `spade run`, `spade install`, `spade upload`, and `spade setup`.
- [DONE] **Create `content/use-cases/_index.md`**: Section index page for use cases. Frontmatter: `title = "Use Cases"`, `template = "section.html"`, `sort_by = "weight"`.
- [DONE] **Create `content/use-cases/remote-sensing.md`**: Use case page for remote sensing workflows. Weight: 1.
- [DONE] **Create `content/use-cases/geospatial-etl.md`**: Use case page for geospatial ETL. Weight: 2.
- [DONE] **Create `content/use-cases/scientific-pipelines.md`**: Use case page for scientific research pipelines. Weight: 3.

---

## Phase 6: Interactive Elements and Polish

- [DONE] **Implement mobile navigation toggle**: Add a small inline `<script>` in `base.html` (or a `static/js/main.js` file) that toggles the mobile hamburger menu open/closed. Keep it minimal — no framework dependencies.
- [DONE] **Implement language tab switcher**: Add a small script for the code example section that switches between Python/TypeScript/Go/Rust tabs. Pure JavaScript, no dependencies.
- [DONE] **Add playing card decorative elements**:
  - CSS-only corner indices on the hero section (small "A" + spade in top-left, rotated in bottom-right, like a playing card)
  - Subtle suit symbol watermarks in section backgrounds (very low opacity)
  - Red horizontal rule dividers between sections styled as a line with a centered spade icon
  - Card-fan or card-spread motif as a decorative element (CSS transforms on stacked cards)
- [DONE] **Add scroll animations** (optional, CSS-only): Use `@keyframes` and `IntersectionObserver` (minimal JS) to fade in sections as user scrolls down.
- [DONE] **Optimize SVG assets**: Ensure the spade logo SVG is optimized (minified, no unnecessary metadata). Inline critical SVGs in templates for performance.
- [DONE] **Add `static/robots.txt`**: Allow all crawlers, point to sitemap.
- [DONE] **Add `templates/404.html`**: Custom 404 page extending `base.html` with a playing-card-themed "card not found" message (e.g., "Looks like this card isn't in the deck.").

---

## Phase 7: Build Verification and Testing

- [DONE] **Run `zola build`** from the website directory and verify it completes without errors. Fix any template syntax errors, missing variables, or Sass compilation issues.
- [DONE] **Run `zola serve`** and manually verify in a browser:
  - Landing page loads with correct layout, colors, and content
  - Navigation links work (Home, Features, Use Cases, external Docs/GitHub links)
  - Feature cards render in a grid with playing card styling
  - "How it Works" section displays the 3-step flow
  - Code examples render with syntax highlighting
  - Use case cards render correctly
  - CTA buttons link to correct destinations
  - Footer renders with correct layout and links
  - Mobile responsive: hamburger menu works, grids stack, hero resizes
- [DONE] **Create `tests/` directory** at the website root for automated validation scripts.
- [DONE] **Create `tests/test_build.sh`**: A shell script that:
  - Runs `zola build` and asserts exit code 0
  - Asserts the `public/` output directory exists
  - Asserts `public/index.html` exists and is non-empty
  - Asserts `public/style.css` exists (Sass compiled)
  - Asserts `public/features/index.html` exists
  - Asserts `public/use-cases/index.html` exists
  - Asserts `public/404.html` exists
  - Asserts `public/robots.txt` exists
  - Asserts `public/sitemap.xml` exists (Zola auto-generates this)
- [DONE] **Create `tests/test_content.sh`**: A shell script that:
  - Checks `public/index.html` contains expected text strings (e.g., "Spade", the tagline, "Features", "How It Works", "Get Started")
  - Checks that all internal links in `public/index.html` resolve to existing files in `public/`
  - Checks that feature pages exist under `public/features/`
  - Checks that use case pages exist under `public/use-cases/`
- [DONE] **Create `tests/test_accessibility.sh`** (optional): A shell script that:
  - Checks all `<img>` tags in `public/index.html` have `alt` attributes
  - Checks heading hierarchy (h1 -> h2 -> h3, no skipped levels)
  - Checks that color contrast meets WCAG AA (manual check or use a tool if available)
- [DONE] **Create `tests/run_all.sh`**: A master script that runs all test scripts and reports pass/fail status. Exits with code 0 only if all tests pass.
- [DONE] **Run `tests/run_all.sh`** and verify all tests pass. Fix any failures.

---

## Phase 8: Final Review and Cleanup

- [DONE] **Review all content** for accuracy against the specifications in `../spec/`. Ensure no features are misrepresented or omitted.
- [DONE] **Review the color scheme**: Verify that the main page is white with black text and red accents throughout. No other accent colors should appear.
- [DONE] **Review the playing card theme**: Ensure the card motif is present but tasteful — it should enhance, not overwhelm, the content. Key elements: spade logo, suit icons on feature cards, card-styled content boxes, corner indices on hero.
- [DONE] **Review responsiveness**: Test at mobile (375px), tablet (768px), and desktop (1280px) widths.
- [DONE] **Clean up**: Remove any placeholder or TODO content. Ensure all links point to valid destinations (external links to docs.spade.app and GitHub are acceptable even if those sites don't exist yet).
- [DONE] **Final `zola build` and `tests/run_all.sh`**: Confirm clean build and all tests passing.
