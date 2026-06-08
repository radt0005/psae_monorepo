# End-to-end tests (Phase G)

Playwright suite that drives the real Nuxt app against the real backend stack
(Postgres + MinIO + RabbitMQ). This is separate from the vitest unit tests,
which run in-process against PGlite and never touch the network.

## What's covered

| Spec | Phase G scenario |
| --- | --- |
| `auth.spec.ts` | 1 — sign up → log in → reach protected page → log out |
| `editor.spec.ts` | round-trip YAML import → export; (fixme) ambiguous-connection resolve |
| `pipelines.spec.ts` | 3 — save → reopen → run; 4 — share pipeline → invitee sees it |
| `data.spec.ts` | 5 — upload custom data + share; (fixme) >100 MB + use-in-pipeline |
| `results.spec.ts` | 6 — browse/download (single + ZIP); 7 — share a run |

Two scenarios are `test.fixme` with in-file TODOs explaining the blocker
(Vue Flow edge-drag mechanics; >100 MB multipart + editor↔data binding).

## The worker seam

There is no Go worker in e2e. Result-browsing tests simulate it through the
`PATCH /api/runs/:id` callback (`helpers/worker.ts`), which is guarded by
`WORKER_CALLBACK_SECRET`. To make downloads real, tests first PUT actual bytes
to MinIO via the presigned-upload flow (`helpers/storage.ts`) and point the
run's output file at that object key.

## One-time setup

```bash
cp .env.example .env             # creds already match docker-compose
bun run stack:up                 # Postgres + MinIO + RabbitMQ, waits for health,
                                 #   and auto-creates the MinIO "spade" bucket
bun run db:migrate               # apply migrations
bunx playwright install chromium # browser binaries
```

(`bun run stack:up` is `docker compose up -d --wait`; the bucket is created by
the one-shot `createbuckets` service in docker-compose.yml.)

## Environment

The web server and the tests must agree on `WORKER_CALLBACK_SECRET`. Put the
backend config in `.env` (see `.env.example`) and export the secret for the
test process too:

```bash
export WORKER_CALLBACK_SECRET=replace-me-with-a-long-random-string   # must match the server
export E2E_BASE_URL=http://localhost:3000                            # optional, this is the default
```

Relevant server env (from `.env.example`): `DATABASE_URL`, `BETTER_AUTH_SECRET`,
`BETTER_AUTH_URL`, `S3_*`, `RABBITMQ_URL`, `WORKER_CALLBACK_SECRET`.

## Running

```bash
bun run test:e2e          # headless
bun run test:e2e:ui       # Playwright UI mode
bun run test:e2e:report   # open the last HTML report
```

By default Playwright starts the dev server itself (`bun run dev`) and waits
for `E2E_BASE_URL`. If you already have the app running, set `E2E_NO_SERVER=1`
to skip that.

## Seeding (for the fixme scenarios)

The block browser and the ambiguous-connection flow need installed blocks, and
block upload (`POST /api/blocks`) is admin-only. To seed:

1. Register a user, then promote it in Postgres:
   `UPDATE "user" SET role = 'admin' WHERE email = '<you>';`
2. `POST /api/blocks` with a YAML manifest (see `spec/blocks.md` §3) for each
   block. For the ambiguity test you need two blocks whose type-matching is
   genuinely ambiguous (≥2 same-typed outputs feeding ≥2 same-typed inputs).

A `helpers/seed.ts` that automates this is a good follow-up once those
scenarios are unskipped.

## Manual regression checklist (web_ui.md opening 8 items)

Every box should be reachable from the homepage in ≤2 clicks:

- [ ] 1. Create & run a pipeline via the flowchart (Editor → Add Block → Export → Submit run)
- [ ] 2. View results (Results → a run)
- [ ] 3. Download result files — single file and "Download all (ZIP)"
- [ ] 4. Re-use a pipeline (Pipelines → Open)
- [ ] 5. Share a pipeline (share endpoint; invitee sees it under "Shared with me")
- [ ] 6. Browse blocks (Blocks → detail)
- [ ] 7. Share results (run detail → Sharing)
- [ ] 8. Upload custom data (Data → Upload file)
- [ ] Auth & authorization: signup / login / logout / protected-route redirect
- [ ] Input/Output wiring resolution (ambiguous connection → modal)
- [ ] Look & feel matches the marketing/docs site (palette, fonts, logo)
