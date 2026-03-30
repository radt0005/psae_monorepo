# Website Implementation Notes

## Syntax Highlighting for Template Code Blocks

### Problem
The homepage code examples live in the `index.html` template, not in markdown
content. Zola 0.22's syntax highlighting only processes code fences in markdown
files — there is no `highlight()` template function available.

### Approaches tried and rejected

1. **Zola `highlight()` Tera function** — Does not exist in Zola 0.22.
   Build fails with "Function 'highlight' not found".

2. **Tera backtick/escaped strings with highlight()** — Moot since the function
   doesn't exist, but also: Tera does not support `\"` escape sequences inside
   double-quoted strings (parse error), though backtick strings do work for
   multi-line literals with embedded quotes.

3. **Shortcodes with markdown body processing** — `{% codepanel() %}` shortcodes
   with code fences in the body do NOT get their body rendered as markdown in
   Zola 0.22 section content (`_index.md`). The code fences pass through as
   literal backtick text. Tested both with and without surrounding HTML blocks;
   neither works.

### Solution chosen
Code fences go directly in `_index.md` as plain markdown content. Zola's
markdown processor highlights them with the `github-dark` theme (configured in
`config.toml` under `[markdown.highlighting]`). The template renders the
highlighted HTML via `{{ section.content | safe }}` inside the code-tabs
container. JavaScript (`main.js`) then dynamically:

- Finds all `<pre>` elements with `code[data-lang]` children
- Wraps each in a `.code-panel` div
- Generates tab buttons from a language-name lookup table
- Wires up click handlers for tab switching

This keeps syntax highlighting entirely server-side (no JS highlighting library)
while still supporting the tabbed UI.

## Zola 0.22 Highlight Theme Names

The available built-in themes are `github-light` and `github-dark`. The older
syntect theme names (`base16-ocean-dark`, `OneHalfDark`, etc.) do not exist in
Zola 0.22 and cause build failures.
