# Spade Documentation Website Implementation Plan

This plan covers the full implementation of the Spade documentation website at `https://docs.spade.app`, built with Zola 0.22.1. The site documents the Spade data processing system for an audience of researchers and developers, covering core concepts, YAML pipeline authoring, the CLI, and block development libraries for Python, R, TypeScript, Go, and Rust.

---

## Phase 1: Zola Configuration and Project Setup

- [x] Update `config.toml` with full site settings: `title = "Spade Documentation"`, `description`, `default_language = "en"`, `generate_feeds = false`, `minify_html = true`, taxonomy definitions (none needed initially), and `[markdown]` settings including `highlight_theme = "nord"` and `highlight_themes_css` for light/dark support. Set `[search]` with `index_format = "elasticlunr_json"` so the search index is generated as a JSON file consumable by a custom search UI. Add `[extra]` variables for `docs_version = "1.0"`, `product_url = "https://spade.app"`, and `github_repo = "https://github.com/..."` (placeholder until repo is public).
- [x] Create the `static/fonts/` directory and add the font files used by the main Spade website: **Playfair Display** (headings), **Inter** (body), and **JetBrains Mono** (code). These should be self-hosted WOFF2 files for performance. If licensing prevents bundling, use Google Fonts CDN links in the base template instead.
- [x] Create `static/img/` and add the Spade logo SVG (`spade-logo.svg`), favicon (`favicon.svg`), and any shared brand assets from the main website at `../website/static/img/`.
- [x] Create `static/js/search.js` with a client-side search implementation using `elasticlunr`. The script should: load `search_index.en.json` on first interaction, provide a search input handler with debouncing (300ms), render results as a dropdown list with page title, section breadcrumb, and a content snippet, and support keyboard navigation (arrow keys, Enter, Escape). Reference the Zola project's own search implementation as a starting point.
- [x] Create `static/js/main.js` with minimal JavaScript for: mobile navigation toggle, copy-to-clipboard for code blocks (insert a "Copy" button on each `<pre>` element), active sidebar link highlighting based on scroll position, and collapsible sidebar sections on mobile.

---

## Phase 2: Sass Styling

- [x] Create `sass/_variables.scss` defining the design tokens from the main Spade website: color palette (`$red-primary: #c0392b`, `$red-light: #e74c3c`, `$red-dark: #96281b`, neutrals `$black: #1a1a1a`, `$white: #ffffff`, `$gray-light: #f5f5f5`, `$gray: #888888`, `$gray-dark: #333333`), typography (`$font-heading: 'Playfair Display', serif`, `$font-body: 'Inter', sans-serif`, `$font-mono: 'JetBrains Mono', 'Fira Code', monospace`), spacing scale (`$space-xs` through `$space-xxl`), breakpoints (`$bp-mobile: 480px`, `$bp-tablet: 768px`, `$bp-desktop: 1024px`), and layout dimensions (`$sidebar-width: 280px`, `$content-max-width: 800px`, `$container-max-width: 1200px`).
- [x] Create `sass/_reset.scss` with a minimal CSS reset: box-sizing border-box on all elements, zero default margin/padding on body, and sensible defaults for images and links.
- [x] Create `sass/_typography.scss` with `@font-face` declarations (if self-hosting), base `body` font settings (`$font-body`, 16px, line-height 1.7 for readability), heading styles (`h1`-`h6` using `$font-heading`), paragraph and list styles with generous spacing, and inline code styling (light gray background, red-tinted text, slight padding and border-radius).
- [x] Create `sass/_layout.scss` defining the documentation page layout: a fixed-position left sidebar (`$sidebar-width` wide), a scrollable main content area (centered, `$content-max-width`), and an optional right-side table-of-contents rail (220px wide, visible only on wide screens above 1280px). Use CSS Grid: `grid-template-columns: $sidebar-width 1fr 220px` for desktop, collapsing to single-column on tablet and below.
- [x] Create `sass/_navigation.scss` styling the top header bar (fixed, with frosted glass `backdrop-filter: blur(10px)` effect matching the main site), the sidebar navigation (hierarchical list with indentation for nested sections, active state with left red border and background tint), and the mobile hamburger menu (slide-in drawer from left).
- [x] Create `sass/_components.scss` with styles for: admonition/callout boxes (note, warning, tip, important — each with a colored left border and icon), code blocks (dark background `#2b303b` with syntax highlighting, horizontal scrolling for long lines, line numbers), tables (striped rows, sticky header), breadcrumbs, pagination links (previous/next page), language tabs (for multi-language code examples — a tabbed interface with a red underline on the active tab), and the search modal/dropdown.
- [x] Create `sass/_responsive.scss` with media queries that collapse the sidebar into a hamburger-triggered drawer on screens below `$bp-tablet`, hide the ToC rail below `$bp-desktop`, and adjust font sizes and spacing for mobile readability.
- [x] Create `sass/style.scss` as the main entry point that imports all partials in order: `_variables`, `_reset`, `_typography`, `_layout`, `_navigation`, `_components`, `_responsive`.

---

## Phase 3: Zola Templates

- [x] Create `templates/base.html` as the root layout. It should include: `<!DOCTYPE html>`, `<html lang="{{ lang }}">`, `<head>` with charset, viewport meta, `<title>{% block title %}{{ config.title }}{% endblock %}</title>`, `<meta name="description">` block, the compiled `style.css` link, font preloads, favicon link, and Open Graph meta tags. The `<body>` should contain: a `{% block header %}` with the site header/nav bar (logo linking to `https://spade.app`, "Docs" as active, search input field, GitHub link), `{% block content %}{% endblock %}`, and `{% block footer %}` with a minimal footer (copyright, link to main site, link to GitHub). Include `search.js` and `main.js` script tags before closing `</body>`.
- [x] Create `templates/index.html` extending `base.html` for the documentation home page. This template renders `section.content` (from `content/_index.md`) as the hero/welcome area, followed by a grid of documentation section cards. Each card links to a top-level section (Getting Started, Core Concepts, Pipelines, CLI Reference, Libraries, Tutorials). Use `get_section(path="...")` to pull titles and descriptions for each card. The layout should not include the sidebar (landing page only).
- [x] Create `templates/section.html` extending `base.html` for documentation section listing pages. It renders the three-column layout: left sidebar navigation (all sections), center content area showing `section.content` followed by a list of child pages (title, description, link) and child subsections, and right ToC rail (if section content has headings). The sidebar must show the full navigation tree with the current section expanded and highlighted.
- [x] Create `templates/page.html` extending `base.html` for individual documentation pages. Same three-column layout as `section.html`. Center content renders `page.content | safe`. Below the content, include previous/next navigation links using `page.lower` and `page.higher` (from weight-based sorting). The right rail shows the table of contents generated from `page.toc`. Include "Edit this page on GitHub" link using the page's path.
- [x] Create `templates/shortcodes/note.html` rendering an admonition box with a blue-left-border "Note" callout: `<div class="admonition note"><span class="admonition-title">Note</span>{{ body }}</div>`.
- [x] Create `templates/shortcodes/warning.html` rendering a yellow/orange "Warning" admonition with the same structure.
- [x] Create `templates/shortcodes/tip.html` rendering a green "Tip" admonition.
- [x] Create `templates/shortcodes/important.html` rendering a red "Important" admonition.
- [x] Create `templates/shortcodes/lang_tabs.html` rendering a tabbed code block interface. Accepts a `body` containing fenced code blocks separated by a delimiter (e.g., `<!-- tab:Python -->`, `<!-- tab:R -->`, etc.). The shortcode parses the body and generates a tabbed UI where each tab shows the code block for one language, with the first tab active by default. This is critical for showing equivalent block implementations across all five languages.
- [x] Create `templates/macros/nav.html` with a Tera macro `sidebar_nav` that recursively renders the site's section tree as a nested `<ul>`. Each item shows the section title, is a link to the section, and has a toggle arrow for expanding/collapsing children. The current page's section path should be expanded and highlighted with an active class. Accept the top-level sections as input via `get_section()`.

---

## Phase 4: Content — Information Architecture

All content goes in `content/`. Every directory with pages gets an `_index.md` with `sort_by = "weight"` so page ordering is explicit. Each page's front matter includes `title`, `description`, and `weight` (for ordering). Descriptions should be jargon-free since many readers are researchers.

- [x] Create `content/_index.md` with front matter (`title = "Spade Documentation"`, `template = "index.html"`) and body content: a welcoming introduction explaining that Spade is a system for building reproducible data processing workflows, especially for geospatial data. Include a brief "What you'll find here" list pointing to the main sections with one-sentence descriptions. Mention that Spade supports Python, R, TypeScript, Go, and Rust. Set the tone: accessible, thorough, no assumed programming expertise beyond basic familiarity.

---

## Phase 5: Content — Getting Started

- [x] Create `content/getting-started/_index.md` (`title = "Getting Started"`, `weight = 1`, `sort_by = "weight"`). Body: brief section overview explaining this section walks through installation, first pipeline, and first block.
- [x] Create `content/getting-started/installation.md` (`weight = 1`). Cover: system requirements (OS support), installing the Spade CLI (Go binary, or `go install`), running `spade setup` to initialize `~/.spade/`, verifying the installation with `spade --help`. Include expected terminal output. Mention that block development additionally requires language-specific toolchains (Rust/Cargo, Go, Python/uv, Bun, R) — link to each language's getting-started page for setup details.
- [x] Create `content/getting-started/first-pipeline.md` (`weight = 2`). Walk through: what a pipeline is (a series of processing steps connected together), creating a simple two-block pipeline YAML file from scratch, explaining each field (`id`, `name`, `version`, `blocks`, `inputs`, `args`), validating with `spade check pipeline.yaml`, running with `spade run pipeline.yaml`, and inspecting the outputs. Use a concrete geospatial example (download satellite image, reproject it). Show the full YAML and expected output at each step.
- [x] Create `content/getting-started/first-block.md` (`weight = 3`). Walk through creating a custom block: `spade init --language python` to scaffold a collection, `spade add my-block` to add a block, editing the generated manifest (`blocks/my-block.yaml`) to declare inputs/outputs, implementing the handler in the generated Python file (read an input file, process it, write output), validating with `spade check`, installing locally with `spade install file://.`, and using the new block in a pipeline. Keep the example simple (e.g., a block that reads a CSV and computes summary statistics).

---

## Phase 6: Content — Core Concepts

- [x] Create `content/concepts/_index.md` (`title = "Core Concepts"`, `weight = 2`, `sort_by = "weight"`). Body: overview explaining the key ideas behind Spade's design — isolation, reproducibility, and language-agnostic processing.
- [x] Create `content/concepts/blocks.md` (`weight = 1`). Explain what blocks are (self-contained units of computation), how they are isolated (run as separate subprocesses, sandboxed), the block manifest format (`blocks/<name>.yaml`) with full field reference and examples, the execution environment (working directory layout with `params.yaml`, `inputs/`, `outputs/`, `logs/`), input/output types (file, directory, collection, string, number, boolean, json, expansion) with descriptions and when to use each, caching behavior (how Spade determines cache keys from block ID + version + input hashes + params), network access (disabled by default, enabled via `network: true`), and error handling (non-zero exit = failure, pipeline halts).
- [x] Create `content/concepts/collections.md` (`weight = 2`). Explain block collections: what they are (repositories of related blocks sharing a language and build system), directory structure (`blocks/*.yaml` manifests, language-specific source layout), language detection from project root files (`Cargo.toml`, `go.mod`, `pyproject.toml`, `package.json`, `renv.lock`), versioning, and the relationship between collection name and block IDs (`<collection>.<block>`).
- [x] Create `content/concepts/pipelines.md` (`weight = 3`). Explain pipelines conceptually: directed acyclic graphs (DAGs) of block invocations, how data flows from one block's outputs to the next block's inputs, dependency resolution (blocks execute only when all upstream blocks complete), parallel execution of independent blocks, and validation rules (unique IDs, valid references, no cycles, type compatibility).
- [x] Create `content/concepts/map-reduce.md` (`weight = 4`). Explain map/reduce for parallel processing: when you need it (processing many similar items, like tiles of a large satellite image), the three-step pattern (map block enumerates items, downstream blocks run once per item in parallel, reduce block collects results), map blocks and expansion manifests, how the scheduler fans out invocations (creating `<block_id>.0`, `<block_id>.1`, etc.), context propagation (downstream blocks inherit map context), broadcasting non-mapped inputs (e.g., a trained model shared across all tiles), reduce blocks collecting outputs into a collection, and the constraint that nested maps are not yet supported. Include a clear diagram or step-by-step walkthrough of the satellite tile processing example from the spec.
- [x] Create `content/concepts/execution-model.md` (`weight = 5`). Explain how Spade executes pipelines end-to-end: the scheduler determines execution order, workers receive block assignments, the worker sets up the working directory (creates dirs, writes `params.yaml`, symlinks inputs from dependency outputs), the block subprocess runs in a sandbox (`isolate`), stdout/stderr are captured to `logs/`, the worker reports success/failure to the scheduler, and caching stores outputs for future reuse. Cover both local execution (single-instance scheduler via `spade run`) and distributed execution (multiple workers with shared filesystem).
- [x] Create `content/concepts/input-resolution.md` (`weight = 6`). Explain how block inputs are wired: simple (bare) references where Spade matches outputs to inputs by type, explicit references where you name both the source block and the specific output, the resolution algorithm (explicit first, then type-matching for remaining, reject if ambiguous), and when to use each form. Include clear before/after YAML examples showing both reference styles.

---

## Phase 7: Content — Pipeline Reference

- [x] Create `content/pipelines/_index.md` (`title = "Pipeline Reference"`, `weight = 3`, `sort_by = "weight"`). Body: overview stating this section is the complete reference for writing pipeline YAML files.
- [x] Create `content/pipelines/format.md` (`weight = 1`). Full YAML pipeline format reference: top-level fields (`id`, `name`, `version`, `description`), the `blocks` list, each block's fields (`id`, `name`, `inputs`, `args`), ID format (UUIDv7), how `args` map to `params.yaml` at runtime. Include a complete annotated example pipeline with inline comments explaining every field.
- [x] Create `content/pipelines/input-references.md` (`weight = 2`). Detailed reference for input wiring syntax: bare references (just the invocation ID string), explicit references (object with `block` and `output` keys), mixed references in the same inputs list, and the type-matching resolution algorithm with step-by-step examples showing both unambiguous and ambiguous cases.
- [x] Create `content/pipelines/validation.md` (`weight = 3`). Document what `spade check` validates: unique block invocation IDs, all referenced IDs exist in the pipeline, all block names refer to installed blocks, the dependency graph is acyclic, input/output type compatibility between connected blocks, named output references match declared outputs, all required `args` are present. Include examples of invalid pipelines with the expected error messages.
- [x] Create `content/pipelines/map-reduce-pipelines.md` (`weight = 4`). Pipeline-level reference for map/reduce: declaring a map block invocation that outputs an expansion, connecting downstream blocks that inherit the map context, wiring the reduce block to collect mapped outputs, and a complete end-to-end example pipeline YAML for the satellite tile processing use case (download → map → process each → classify each with broadcast model → reduce/mosaic).
- [x] Create `content/pipelines/examples.md` (`weight = 5`). A collection of complete, copy-pasteable pipeline YAML examples: (1) a simple two-block linear pipeline, (2) a pipeline with parallel independent branches that merge, (3) a map/reduce pipeline for tile-based processing, and (4) a pipeline demonstrating explicit input references for blocks with multiple outputs. Each example should have a description, the full YAML, and a brief explanation of the data flow.

---

## Phase 8: Content — CLI Reference

- [x] Create `content/cli/_index.md` (`title = "CLI Reference"`, `weight = 4`, `sort_by = "weight"`). Body: overview of the `spade` command-line tool, its role in the development workflow, and a quick reference table listing all commands with one-line descriptions.
- [x] Create `content/cli/setup.md` (`weight = 1`). Document `spade setup`: what it does (creates `~/.spade/` directory structure with `blocks/`, `cache/`, `pipelines/`, and `registry.db`), the `--rebuild-index` flag (rescans installed blocks and rebuilds the SQLite registry), when to use it (first-time setup, or after manually modifying installed blocks), and the `SPADE_DIR` environment variable for custom install locations.
- [x] Create `content/cli/init.md` (`weight = 2`). Document `spade init`: purpose (scaffold a new block collection), required `--language` / `-l` flag with allowed values (`rust`, `go`, `python`, `typescript`, `r`), what files and directories are created for each language (detail the scaffolded structure for all five), and the expected workflow after running it.
- [x] Create `content/cli/add.md` (`weight = 3`). Document `spade add <name>`: purpose (add a new block to the current collection), what it creates (a `blocks/<name>.yaml` manifest and a language-appropriate source file), the generated manifest template (show the default YAML), the generated source file for each language (show the boilerplate code for all five languages), and next steps (edit the manifest to declare inputs/outputs, implement the handler).
- [x] Create `content/cli/check.md` (`weight = 4`). Document `spade check`: two modes (collection validation without arguments, pipeline validation with a YAML file argument), what collection validation checks (manifest structure, field types, ID conventions, entrypoint file existence, map/reduce constraints), what pipeline validation checks (all seven validation rules), and example output for both valid and invalid inputs.
- [x] Create `content/cli/install.md` (`weight = 5`). Document `spade install <git-url>`: purpose (install a block collection from a Git repository), the installation process step-by-step (clone → detect language → discover blocks → build → copy to `~/.spade/blocks/<collection>/<version>/` → register), language-specific build commands (`cargo build --release`, `go build`, `uv sync`, `bun build`, `Rscript setup.R`), support for local paths via `file://` URLs, and how to verify installation succeeded.
- [x] Create `content/cli/run.md` (`weight = 6`). Document `spade run <pipeline.yaml>`: purpose (execute a pipeline locally), the execution flow (load → validate → resolve → schedule → execute blocks in order → report), flags (`--no-ui` for simple line output, `--keep-work-dir` to preserve the working directory), where working directories are created (`~/.spade/pipelines/<pipeline_id>/`), caching behavior (skips blocks whose inputs haven't changed), output location, and what happens on failure (pipeline halts, logs preserved).
- [x] Create `content/cli/upload.md` (`weight = 7`). Document `spade upload`: purpose (package a collection for cloud deployment), what it does (validates via `spade check`, creates a `.tar.gz` archive), and that the cloud upload endpoint is forthcoming.

---

## Phase 9: Content — Library Reference (All Languages)

Each language gets a subsection with identical structure so researchers can follow the same learning path regardless of their preferred language.

- [x] Create `content/libraries/_index.md` (`title = "Block Development Libraries"`, `weight = 5`, `sort_by = "weight"`). Body: explain that Spade provides official libraries for five languages that handle the runtime boilerplate (reading inputs, loading parameters, writing outputs) so block authors can focus on their domain logic. Include a comparison table: language, package/crate name, runtime requirements, and a one-line install command.

### Python Library

- [x] Create `content/libraries/python/_index.md` (`title = "Python"`, `weight = 1`, `sort_by = "weight"`). Body: overview of the Python library, prerequisites (Python 3.12+, `uv` package manager), and installation (`pip install spade` or as a dependency in `pyproject.toml`).
- [x] Create `content/libraries/python/quickstart.md` (`weight = 1`). Step-by-step guide: create a collection with `spade init -l python`, add a block with `spade add`, implement a handler function (complete working example), run and test locally. Emphasize that the handler is a plain Python function — no special framework knowledge needed.
- [x] Create `content/libraries/python/types.md` (`weight = 2`). Document all Spade types available in the Python library: `File`, `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`, `Directory`, and collection variants (`FileCollection`, `RasterFileCollection`, `VectorFileCollection`, `TabularFileCollection`). For each type: what it represents, its attributes (`path` for single types, `paths` for collections), and which manifest `type`/`format` it maps to. Also cover scalar types (`str`, `int`, `float`, `bool`) and how they come from `params.yaml`.
- [x] Create `content/libraries/python/handlers.md` (`weight = 3`). Explain the handler function pattern: function signature with type hints (Spade types for file inputs, Python primitives for scalar params), how the library inspects the signature to build arguments, how inputs and params are merged into kwargs, returning a single output (typed object), returning multiple named outputs (dict), returning no output (`None`), and error handling (raise exceptions, which cause non-zero exit). Include multiple annotated examples.
- [x] Create `content/libraries/python/manifest-generation.md` (`weight = 4`). Explain the `build()` function: how it auto-inspects the handler function's type hints and docstring to generate a block manifest YAML, and how to use it to create or update `blocks/<name>.yaml`. Show the generated YAML for an example handler.
- [x] Create `content/libraries/python/examples.md` (`weight = 5`). Complete worked examples: (1) a block that reads a GeoTIFF raster file, reprojects it, and writes the result, (2) a block that reads a CSV, computes statistics, and writes a JSON output, (3) a map block that reads a collection and produces an expansion manifest. Each example includes the manifest YAML, the handler code, and explanation of the data flow.

### R Library

- [x] Create `content/libraries/r/_index.md` (`title = "R"`, `weight = 2`, `sort_by = "weight"`). Body: overview, prerequisites (R 4.0+, `renv` for dependency management), and installation.
- [x] Create `content/libraries/r/quickstart.md` (`weight = 1`). Step-by-step guide mirroring the Python quickstart but in R: `spade init -l r`, `spade add`, implement a handler using `run()`, test locally. Emphasize that R blocks use familiar R idioms (reading YAML, using base R or tidyverse for processing).
- [x] Create `content/libraries/r/types.md` (`weight = 2`). Document all S4 Spade types: `File`, `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`, `Directory`, and collection variants. Explain the S4 class system briefly for readers unfamiliar with it (slots like `@path`, `@paths`). Cover how scalar parameters (`numeric`, `character`, `logical`) are loaded from `params.yaml`.
- [x] Create `content/libraries/r/handlers.md` (`weight = 3`). Explain handler functions in R: defining the function, annotating types via `spade_types()` (attribute-based metadata), how `run()` calls the handler with merged params and inputs via `do.call()`, returning a single output or a named list for multiple outputs, and setting description via `attr(handler, "spade_description")`. Include annotated examples.
- [x] Create `content/libraries/r/manifest-generation.md` (`weight = 4`). Explain the `build()` function: reads `spade_types` and `spade_description` attributes to generate YAML. Show input/output.
- [x] Create `content/libraries/r/examples.md` (`weight = 5`). Complete worked examples in R paralleling the Python examples: raster processing with the `raster` package, tabular analysis, and a map block.

### TypeScript Library

- [x] Create `content/libraries/typescript/_index.md` (`title = "TypeScript"`, `weight = 3`, `sort_by = "weight"`). Body: overview, prerequisites (Bun runtime, TypeScript 5.0+), and installation.
- [x] Create `content/libraries/typescript/quickstart.md` (`weight = 1`). Step-by-step guide: `spade init -l typescript`, `spade add`, implement with `run()`, test. Note that Spade uses Bun as the TypeScript runtime for performance.
- [x] Create `content/libraries/typescript/types.md` (`weight = 2`). Document the type class hierarchy: `File`, `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`, `Directory`, and collection variants. Explain how metadata is stored via WeakMap and the `SpadeMetadata` system.
- [x] Create `content/libraries/typescript/handlers.md` (`weight = 3`). Explain handler patterns: the `@spadeBlock` decorator with `inputs` and `output` metadata, the alternative `setMetadata()` approach for non-decorator setups, how `buildFunctionArgs()` merges inputs and params, returning single or multiple outputs, and error handling. Include annotated examples.
- [x] Create `content/libraries/typescript/manifest-generation.md` (`weight = 4`). Explain `build()`: reads decorator/WeakMap metadata. Show generated YAML.
- [x] Create `content/libraries/typescript/examples.md` (`weight = 5`). Complete worked examples in TypeScript.

### Go Library

- [x] Create `content/libraries/go/_index.md` (`title = "Go"`, `weight = 4`, `sort_by = "weight"`). Body: overview, prerequisites (Go 1.25+), and installation (`go get`).
- [x] Create `content/libraries/go/quickstart.md` (`weight = 1`). Step-by-step guide: `spade init -l go`, `spade add`, implement with `Run[O]()` generic function, test.
- [x] Create `content/libraries/go/types.md` (`weight = 2`). Document types: `File`, `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`, `Directory`, and collection variants. Explain the Go interfaces (`SpadeType`, `FromInput`, `IntoOutput`) and how generics are used for type safety.
- [x] Create `content/libraries/go/handlers.md` (`weight = 3`). Explain handler patterns: `Run[O IntoOutput](handler func(*Args) (O, error))`, the `*Args` struct with `args.Input[T](name)` and `args.Param[T](name)` methods, `args.HasInput()` / `args.HasParam()` for optional inputs, `RunNoOutput()` for side-effect-only blocks, and the `RunAt()` variant for testing. Include annotated examples.
- [x] Create `content/libraries/go/manifest-generation.md` (`weight = 4`). Explain the fluent `NewManifestBuilder()` API: `.Description()`, `.ManifestInput[T](b, name)`, `.ManifestOutput[T](b, name)`, `.Build()`. Show generated YAML.
- [x] Create `content/libraries/go/examples.md` (`weight = 5`). Complete worked examples in Go.

### Rust Library

- [x] Create `content/libraries/rust/_index.md` (`title = "Rust"`, `weight = 5`, `sort_by = "weight"`). Body: overview, prerequisites (Rust stable, Cargo), and installation (`cargo add spade`).
- [x] Create `content/libraries/rust/quickstart.md` (`weight = 1`). Step-by-step guide: `spade init -l rust`, `spade add`, implement with `run()` generic function, test.
- [x] Create `content/libraries/rust/types.md` (`weight = 2`). Document types: `File`, `RasterFile`, `VectorFile`, `TabularFile`, `JsonFile`, `Directory`, and collection variants. Explain the Rust traits (`SpadeType`, `FromInput`, `IntoOutput`) and the `define_file_type!` / `define_collection_type!` macros. Cover `Result` return types and `SpadeError` variants.
- [x] Create `content/libraries/rust/handlers.md` (`weight = 3`). Explain handler patterns: `run(handler)` with closures/functions returning `Result<O, Box<dyn Error>>`, the `Args` struct with `args.input::<T>(name)` and `args.param::<T>(name)` methods, `Outputs::new()` for multiple outputs vs `Outputs::single()`, and testing with the private `run_at()`. Include annotated examples.
- [x] Create `content/libraries/rust/manifest-generation.md` (`weight = 4`). Explain the builder API: `build().description(...).input::<T>(name).output::<T>(name).build()`. Show generated YAML.
- [x] Create `content/libraries/rust/examples.md` (`weight = 5`). Complete worked examples in Rust.

---

## Phase 10: Content — Tutorials

- [x] Create `content/tutorials/_index.md` (`title = "Tutorials"`, `weight = 6`, `sort_by = "weight"`). Body: overview explaining these are end-to-end walkthroughs for common tasks.
- [x] Create `content/tutorials/building-a-block.md` (`weight = 1`). A comprehensive, language-agnostic tutorial on the full block development lifecycle: planning inputs/outputs, writing the manifest, implementing the handler (show equivalent code in all five languages side-by-side using the `lang_tabs` shortcode), testing locally with a hand-crafted working directory, validating with `spade check`, installing with `spade install`, and using the block in a pipeline. Use a concrete geospatial example accessible to researchers (e.g., computing NDVI from a satellite image).
- [x] Create `content/tutorials/writing-pipelines.md` (`weight = 2`). A tutorial on designing and writing pipelines from scratch: identifying the processing steps, choosing or creating blocks for each step, writing the YAML with correct input wiring, handling ambiguous connections with explicit references, validating and running, and iterating on the design. Walk through building a realistic multi-step pipeline.
- [x] Create `content/tutorials/map-reduce-tutorial.md` (`weight = 3`). A tutorial specifically on parallel processing with map/reduce: the problem (a large dataset that needs per-item processing), creating a map block that produces an expansion, connecting downstream processing blocks, adding a broadcast input (e.g., a model or configuration shared across all items), collecting results with a reduce block, and running the pipeline. Use the satellite tile mosaicking example from the spec.
- [x] Create `content/tutorials/testing-blocks.md` (`weight = 4`). A tutorial on testing blocks during development: creating a test working directory manually (with `params.yaml`, `inputs/`, `outputs/`), running the block handler directly, using the language library's test utilities (e.g., Go's `RunAt`, Rust's `run_at`), writing unit tests, and integration testing with `spade run` on a single-block pipeline.

---

## Phase 11: Search Page

- [x] Create `content/search/_index.md` with front matter `title = "Search"`, `template = "search.html"`, `render = true`, and empty body. This provides the URL `/search/` for the search results page.
- [x] Create `templates/search.html` extending `base.html` with a full-page search interface: a prominent search input field, a results container populated by `search.js`, and a "no results" message. The search index file (`search_index.en.json`) is loaded by `search.js` on this page.

---

## Phase 12: Build Verification and Polish

- [x] Run `zola check` to verify all internal links resolve, no broken references exist, and the configuration is valid.
- [x] Run `zola build` to compile the full site into `public/`. Verify zero errors and zero warnings. Inspect the output directory to confirm all pages, styles, scripts, and assets are present.
- [x] Run `zola serve` and manually verify: the home page renders with section cards, navigation works across all sections and pages, the sidebar correctly highlights the current page, code blocks have syntax highlighting, the search feature returns relevant results, the site is responsive on mobile viewport sizes, and all shortcodes (note, warning, tip, lang_tabs) render correctly.
- [x] Verify that every page has a meaningful `<title>` and `<meta name="description">` for search engine discoverability.
- [x] Verify that the `lang_tabs` shortcode correctly renders tabbed code blocks for multi-language examples and that tab switching works without page reload.
- [x] Review all content for jargon and readability. Replace technical jargon with plain explanations or define terms on first use. Ensure that a researcher with basic programming knowledge (but no systems engineering background) can follow every page.
