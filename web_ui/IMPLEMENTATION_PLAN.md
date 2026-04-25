# Web UI Implementation Plan

A step-by-step checklist to bring the `web_ui/` Nuxt prototype into conformance with `../spec/web_ui.md` (and the supporting `pipeline.md`, `blocks.md`, `scheduler.md`, `worker.md`).

## Architectural decisions baked into this plan

These were settled in the planning conversation:

- **Stay on Nuxt 3 + Vue 3.** Vue Flow is the most expensive piece of code in the repo to recreate; switching frameworks gains nothing functional and loses preference. Nuxt 3's Nitro server gives us full-stack TypeScript; we don't need Next.js for that.
- **Replace PocketBase entirely with a custom Postgres + Drizzle + Better Auth + MinIO stack.** The deciding factor is `web_ui.md`'s requirement for "potentially large" geospatial uploads with sharing — MinIO multipart is purpose-built for it, and Postgres + RLS gives the spec's `private | shared | public` ACL model first-class. Drizzle uses the Bun SQL driver (fastest, built in).
- **Incremental migration, not a from-scratch rewrite.** The keepers are the logic with tests: `utils/wiring.ts`, `utils/validate.ts`, `utils/pipeline.ts`, `composables/useFlow.ts`, the Vue Flow editor components, all 39 vitest specs. Everything that touches PocketBase gets replaced when we touch it during the swap; we don't preserve old patterns when threading the new stack through.
- **Design system from `../website` and `../documentation`.** Both Zola sites already share an identical, well-documented palette and typography. Phase 10 ports those tokens into Tailwind / Nuxt UI.
- **Phase order.** (a) Phase 10 design system from the marketing/docs site (cheap, parallelizable, lives next to backend work). (b) Backend swap-out: Postgres + Drizzle + Better Auth replaces PocketBase entirely, including the existing `runs` / `blocks` / `files` tables and the `Export.vue` submission path. (c) MinIO + Phase 7 uploads. (d) Phases 4 / 5 / 8.

## Done so far (v1 of this plan)

These were landed in the first implementation pass and have vitest coverage. Updated phases below take this as the starting point.

- ✅ Vitest test infrastructure (`bun run test`, `test:watch`, `test:ui`, `typecheck`)
- ✅ **Phase 0** repo hygiene & runtimeConfig (incl. path-traversal guards on results endpoints)
- ✅ **Phase 1** pipeline & block data model rewrite — `Block.inputs` plural, `InputRef = string | {block, output}`, `is_data` removed, UUIDv7 throughout, `BlockManifest` type matching `blocks.md` §3
- ✅ **Phase 2** I/O wiring resolver — `utils/wiring.ts` (algorithm), `ConnectionResolver.vue` (modal with descriptions), edge bindings, save guards in Export.vue, `/api/blocks/[id]` endpoint
- ✅ **Phase 3** client-side validation — `utils/validate.ts` (all `pipeline.md` §7 checks + map/reduce structural), floating Problems panel
- ✅ Quick wins: signup actually creates users (Phase 9.1), block-editor surfaces `network: true` badge (Phase 5 preview), CustomBlock shows unresolved-edge warning

---

# Phase A — Design system port (was Phase 10)

The website (`../website`) and the docs site (`../documentation`) share an identical visual language. Our job is to port the SCSS tokens to Tailwind and rebuild Nuxt UI's defaults to match. Done first because (1) it's cheap, (2) it's parallelizable with backend work, (3) every later page benefits.

## A.1 Tokens

The tokens to port (verbatim from `../website/sass/_variables.scss` and `../documentation/sass/_variables.scss` — they already match):

- [ ] **Colors**:
  - `--color-black: #1a1a1a`
  - `--color-white: #ffffff`
  - `--color-red: #c0392b`
  - `--color-red-light: #e74c3c`
  - `--color-red-dark: #96281b`
  - `--color-gray-light: #f5f5f5`
  - `--color-gray: #888888`
  - `--color-gray-dark: #333333`
  - Admonition extras (from docs): `--color-blue: #2980b9`, `--color-green: #27ae60`, `--color-yellow: #f39c12`, plus `*-light` background variants
- [ ] **Typography**:
  - Heading: `Playfair Display`, weights 400/700/900 — already loaded via Google Fonts on both sites
  - Body: `Inter`, weights 400/500/600/700
  - Mono: `JetBrains Mono`, fallbacks `'Fira Code', Consolas`
  - Sizes: H1 3.5rem, H2 2.5rem, H3 1.75rem, H4 1.25rem, body 16px / line-height 1.6
- [ ] **Spacing scale**: xs 0.25rem, sm 0.5rem, md 1rem, lg 2rem, xl 3rem, xxl 5rem
- [ ] **Radii**: sm 4px, md 8px, lg 16px
- [ ] **Card shadows**: rest `0 2px 8px rgba(0,0,0,0.1)`, hover `0 6px 20px rgba(0,0,0,0.15)`
- [ ] **Breakpoints**: mobile 480, tablet 768, desktop 1024, wide 1280

## A.2 Implementation

- [ ] Add `tailwind.config.ts` with the tokens above as `theme.extend`. Map the names to Tailwind utilities (`text-spade-red`, `bg-spade-gray-light`, `font-heading`, etc.) so we never hard-code hex in components.
- [ ] Add `assets/css/fonts.css` to preconnect + load the same three Google Fonts the sites use.
- [ ] Override Nuxt UI's primary color to map to `spade-red` (`@nuxt/ui` v2 supports `ui: { primary: 'red' }` plus a custom palette; replace the placeholder `colorMode: false` line that's currently a typecheck error).
- [ ] Build a small `components/brand/SpadeLogo.vue` that renders the SVG spade from `base.html` lines 23-26 (so it scales by prop and can land in the navbar/footer/login page).
- [ ] Replace `app.vue` and `layouts/default.vue` with a layout matching the marketing site:
  - Fixed header with logo + nav links (Home / Editor / Pipelines / Blocks / Data / Results) + user menu
  - Footer with brand block + quick links + black background
  - Use `nav__link--active` underline pattern for the current route
- [ ] Replace `pages/index.vue` placeholder with a branded landing in the spirit of the marketing hero (`hero__title`, `hero__subtitle`, `hero__buttons`), but tailored to authenticated users (CTAs: "New Pipeline" / "Browse Pipelines" / "Recent Runs").
- [ ] Tests:
  - Snapshot test for `SpadeLogo.vue` rendering at 28×34 and 24×29 sizes
  - DOM test that `default.vue` renders the nav with the expected links and active-route handling

## A.3 Component-level theming touch-ups (incremental)

Apply the design language to existing surfaces as we touch them in later phases. **Don't** do these in a single big PR — fold them into the relevant phase work:

- [ ] Login + Signup: card variant, brand logo above the form, red primary submit button
- [ ] Editor toolbar buttons: replace generic `<UButton>` styling with `.btn--primary` / `.btn--outline` semantics
- [ ] Result file list and viewers: card padding/radius, mono font for filenames
- [ ] Problems panel: red-themed admonition styling (`.admonition.important`)
- [ ] Connection resolver modal: card with the admonition pattern around the descriptions

---

# Phase B — Backend stack swap (PocketBase → Postgres + Drizzle + Better Auth)

This is the biggest single change. Strategy: **add the new stack alongside PocketBase, migrate one resource at a time, delete PocketBase last.** That keeps the app continuously runnable and makes each step reviewable.

## B.1 Foundations

- [ ] Add deps: `drizzle-orm`, `drizzle-kit`, `bun-types`, `better-auth`, `@better-auth/drizzle`, `dotenv`. Drizzle uses Bun's built-in `Bun.sql` driver (`drizzle-orm/bun-sql`).
- [ ] Add a docker-compose.yml for local dev: Postgres 16, MinIO. Keep RabbitMQ entry alongside (already in use by the worker).
- [ ] Update `.env.example` and `runtimeConfig`:
  - `DATABASE_URL` (postgres)
  - `BETTER_AUTH_SECRET`, `BETTER_AUTH_URL`
  - `S3_ENDPOINT`, `S3_REGION`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_BUCKET`
  - Keep RabbitMQ vars; remove PocketBase vars when B.7 lands (not before)
- [ ] Create `server/db/index.ts` exporting a Drizzle instance from `Bun.sql(DATABASE_URL)`.
- [ ] Create `server/db/schema/` and split tables into one file per resource: `users.ts`, `sessions.ts`, `accounts.ts`, `verifications.ts` (Better Auth tables), `pipelines.ts`, `runs.ts`, `blocks.ts`, `files.ts`, `data_files.ts`, `shares.ts`.
- [ ] Wire `drizzle.config.ts` and `bun drizzle-kit generate` / `bun drizzle-kit migrate` scripts.
- [ ] Tests:
  - Per-table integration tests against an ephemeral Postgres (testcontainers or a dedicated test DB) — insert/select/update/delete one row each
  - Migration round-trip test (apply, then a known-good seed)

## B.2 Better Auth integration

- [ ] Add `server/auth.ts` configuring Better Auth (email/password to start; social providers later if needed). Use the Drizzle adapter pointed at the Better Auth tables.
- [ ] Mount the catch-all auth route handler at `server/api/auth/[...all].ts`.
- [ ] Add a `useAuth` composable wrapping `better-auth/client` for the browser side; replace the `usePB().value.collection('users').authWithPassword(...)` calls in `Login.vue` and `SignUp.vue`.
- [ ] Replace `middleware/auth.ts` with a Better Auth session check; redirect unauthenticated users to `/login`.
- [ ] Implement `pages/confirm.vue` (currently empty) and a `pages/forgot-password.vue` flow using Better Auth's email-verification + reset endpoints.
- [ ] Tests:
  - E2E (with `@nuxt/test-utils`) covering: register → verify-email-stub → login → access protected page → logout
  - Unit test for the auth middleware redirect behaviour

## B.3 Block registry

The PocketBase `blocks` collection is already partially proxied through `/api/blocks/list` and `/api/blocks/[id]`. We just swap the storage layer.

- [ ] Schema: `blocks` table with `name` (unique), `manifest` (jsonb matching `BlockManifest`), `version`, `created_at`, `updated_at`.
- [ ] Repository: `server/db/repositories/blocks.ts` with `listAll`, `getByName`, `upsert(manifest)`.
- [ ] Rewrite `server/api/blocks/list.ts` and `server/api/blocks/[id].get.ts` against the new repo. The contracts already match `BlockListItem` and `BlockManifest` — no client changes needed.
- [ ] Add a write endpoint `POST /api/blocks` (admin-only, server-side trust check via Better Auth session role) that accepts a YAML manifest and upserts.
- [ ] One-off migration script `scripts/migrate-blocks-from-pocketbase.ts` that reads the existing PocketBase rows and inserts them into the new table. Document running it once before we cut over.
- [ ] Tests:
  - Repo tests: upsert round-trip, getByName, listAll ordering
  - API tests: `/api/blocks` returns spec-conformant manifest list

## B.4 Pipelines (the missing pipeline library — also handles old Phase 4)

- [ ] Schema: `pipelines` (`id`, `owner_id`, `name`, `description`, `version`, `yaml`, `visibility` enum, `created_at`, `updated_at`). Plus `pipeline_shares` (`pipeline_id`, `user_id`, `permission`).
- [ ] Repository methods: `listOwnedBy`, `listSharedWith`, `listPublic`, `getById` (with ACL check), `create`, `update`, `delete`, `share`, `unshare`.
- [ ] Endpoints (auth-gated, ACL-checked):
  - `GET /api/pipelines` (mine | shared | public tabs)
  - `POST /api/pipelines`
  - `GET /api/pipelines/:id`
  - `PUT /api/pipelines/:id`
  - `DELETE /api/pipelines/:id`
  - `POST /api/pipelines/:id/share`
  - `DELETE /api/pipelines/:id/share/:userId`
- [ ] Pages:
  - `pages/pipelines/index.vue` — table with mine / shared / public tabs, Open / Run / Share / Delete actions
  - Editor: add a "Save" button distinct from "Run" (opens name/description/visibility dialog). Update `pages/index.vue` so the "Edit Existing Pipeline" link goes to `/pipelines`.
  - Share dialog component — visibility radio + email input for `shared_with`
- [ ] Loading a pipeline from `/pipelines` calls the existing `flow.yamlToNodes()` (already spec-conformant from Phase 1).
- [ ] Tests:
  - Repo: ACL filters return correct rows for each role
  - API: 401 / 403 paths covered
  - Component: pipeline-list loading + tab switching with mocked endpoints

## B.5 Runs

The existing flow uses RabbitMQ (kept) plus PocketBase `runs` for state and result files. We replace the PocketBase half.

- [ ] Schema: `runs` (`id`, `pipeline_id` nullable so ad-hoc YAML still works, `owner_id`, `yaml`, `status` enum, `started_at`, `finished_at`, `error`). Plus `run_files` (`run_id`, `name`, `mime_type`, `size`, `s3_key`, `block_id`) and `run_logs` (`run_id`, `block_id`, `stdout`, `stderr`).
- [ ] Repository: `create`, `getById` (ACL-checked), `listOwnedBy`, `appendFile`, `updateStatus`, `appendLog`.
- [ ] Endpoints:
  - `POST /api/runs` — submits a pipeline (writes to DB, then enqueues to RabbitMQ via existing `publishJob`)
  - `GET /api/runs` — list mine
  - `GET /api/runs/:id` — single run with files + logs
  - `GET /api/runs/:id/files/:name` — streaming download from MinIO with HTTP Range support (Phase B.6 dep)
- [ ] **Worker contract** — coordinate with the Go scheduler team:
  - Worker reads job from RabbitMQ (unchanged), executes, writes outputs to S3 under `runs/<run_id>/<block_id>/<output_name>/<filename>`, then PATCHes `/api/runs/:id` with status + the file metadata. Document the wire format in `../spec/worker.md` once agreed.
- [ ] Update `Export.vue` to POST to `/api/runs` instead of `pb.collection('runs').create(...)`.
- [ ] Update `pages/results/[id].vue` and `pages/results/index.vue` to use the new endpoints.
- [ ] Tests:
  - API: submit-run happy path (mocked queue + DB)
  - Status-polling component test (the `autoRefresh` behavior)

## B.6 Custom data uploads + result files (S3/MinIO; subsumes old Phase 7 + Phase 8.2)

- [ ] Add `server/storage.ts` exporting a thin S3 client (Bun has S3 built in; use `Bun.s3`).
- [ ] Schema: `data_files` (`id`, `owner_id`, `name`, `size`, `mime_type`, `s3_key`, `visibility`, `created_at`). Plus `data_file_shares`.
- [ ] Endpoints:
  - `POST /api/uploads/data/init` — returns a presigned multipart URL for resumable uploads
  - `POST /api/uploads/data/complete` — finalizes + writes the row
  - `GET /api/data` (mine | shared | public)
  - `GET /api/data/:id` (presigned download URL or proxy)
  - `DELETE /api/data/:id`
  - `POST /api/data/:id/share`
- [ ] Result-file streaming: `GET /api/runs/:id/files/:name` returns a presigned URL or a Range-supporting proxy. Replace the per-file looped download in `pages/results/[id].vue` with a proper "Download all" zip stream endpoint (`GET /api/runs/:id/zip`).
- [ ] Pages:
  - `pages/data/index.vue` — list / upload / share / delete with chunked-upload progress UI
  - `pages/data/upload.vue` (or a slide-over from the index) — drag-drop with tus-style resumable behavior
- [ ] Editor integration: when a block has a `type: file` input, allow binding to a `data_files` entry alongside wiring from another block. Wire format proposal: a synthetic source block with `name: data.user_file`, `args: { id }`. Confirm with worker team before shipping.
- [ ] Tests:
  - Repo: upload round-trip with a fake S3 (use MinIO testcontainer)
  - API: presign generation, ACL-blocked download
  - Component: upload-progress UI with mocked stream

## B.7 PocketBase removal

- [ ] Audit for remaining `pb.value.collection(...)` calls. None should remain after B.2 / B.3 / B.4 / B.5 / B.6.
- [ ] Delete `composables/usePB.ts`, `server/utils/useServerPocketbase.ts`, `migrations/` (PocketBase migration dir), the `pocketbase` npm dependency.
- [ ] Remove `POCKETBASE_*` env vars from `nuxt.config.ts` and `.env.example`.
- [ ] Delete the docker-compose pocketbase service.

---

# Phase C — Block browser (was Phase 5)

Most of this is now unblocked because B.3 stores full manifests.

- [ ] `pages/blocks/index.vue` — searchable, filterable list. Filters: collection, kind (`standard | map | reduce`), `network` requirement.
- [ ] `pages/blocks/[id].vue` — block detail: description, version, all inputs/outputs (name + type + format + description), example usages.
- [ ] Replace the `<USelectMenu>` in `BlockEditor.vue` with a richer picker linking to the detail view.
- [ ] `network: true` badge already shipped in the editor; reuse the same component.
- [ ] Tests:
  - Component test for the filter behaviour (mocked block list)
  - Detail-page smoke test

---

# Phase D — Map / reduce in the flowchart (was Phase 6)

- [ ] Custom Vue Flow node variants for `kind: map` and `kind: reduce` (distinct accent color from the brand red — perhaps the marketing palette's gray-dark with a red border).
- [ ] When dragging an edge from a `kind: map` block, mark downstream nodes as "in map context" until a `kind: reduce` is reached. Subtle background grouping.
- [ ] Validation already enforces structure (Phase 3 done); add visual hints for "no nested maps".
- [ ] Document broadcast inputs (non-mapped deps into a map context) in tooltip text — no other UI change needed.

---

# Phase E — Result browsing extras (subset of old Phase 8)

The streaming download + zip endpoint were folded into Phase B.5/B.6. What remains is viewers + sharing.

- [ ] Result sharing: `runs.visibility` + `run_shares` table (model copies pipelines/data). `POST /api/runs/:id/share`. UI on the run detail page.
- [ ] Raster preview component — server-side `gdal_translate` to PNG thumbnail; downloadable original. Decide on a service container vs a worker job; doc accordingly.
- [ ] Parquet/Arrow tabular viewer (server-side conversion to a sampled CSV, or `parquet-wasm` in the browser).
- [ ] Replace the `cleanupFileName` heuristic in `pages/results/[id].vue` with the real filename from `run_files.name`.

---

# Phase F — Auth polish (was Phase 9)

Most of Phase 9 lands inside Phase B.2. Remaining items:

- [ ] Profile page (`pages/account.vue`) — change password, change email, sign out.
- [ ] Audit Better Auth session expiry, CSRF, secure cookies in production.
- [ ] Admin role for the block-upload endpoint (B.3) and for cross-tenant moderation if we add it.

---

# Phase G — End-to-end verification

- [ ] Playwright suite covering:
  - [ ] Sign up → verify email → log in (B.2)
  - [ ] Build a pipeline with an ambiguous connection → resolve via modal → save (Phases 1 / 2 / B.4)
  - [ ] Save pipeline → open in another browser session → run (B.4 / B.5)
  - [ ] Share pipeline → second user sees it (B.4)
  - [ ] Upload a >100 MB data file → use it in a pipeline → run completes (B.6)
  - [ ] Download a single result file; download all as ZIP (B.5 / B.6)
  - [ ] Share a result run → second user views it (E)
- [ ] Round-trip test: hand-written spec-conformant YAML → import → export → byte-identical (already covered by `tests/useFlow.test.ts` at unit level; promote to e2e)
- [ ] Manual regression pass against `web_ui.md`'s opening 8-item checklist; every box reachable from the homepage in ≤2 clicks.

---

## Cross-cutting reminders

- **Don't add features beyond the spec.** When a step feels speculative, drop it and reopen `web_ui.md` to confirm.
- **Coordinate the wire formats with the worker team** before B.5 ships:
  - Run-status PATCH from worker → web UI
  - Custom-data input format in the pipeline YAML (B.6)
- **Phase order is enforced by data dependencies.** Phase A is genuinely parallelizable. Inside Phase B, the order B.1 → B.2 → B.3 → B.4 → B.5 → B.6 → B.7 is right; you can sometimes overlap B.5 and B.6 since they share an S3 dependency that lands once. Phase C onward depends on B being done.
- **The migration's safety net** is that Postgres + MinIO live alongside PocketBase until B.7. If a migration step is wrong, we can flip a feature flag and fall back. Delete PocketBase only when every read/write path is on the new stack.
