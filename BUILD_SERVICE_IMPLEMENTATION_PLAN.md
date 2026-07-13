# Build Service Implementation Plan

Stand up the **registry build runner** (spec/hosting.md §5) as a standalone
service, and give the registry control plane a flag to disable its embedded
build dispatcher so the two can be split in production.

## Decisions (confirmed with project owner)

1. **Job claiming: direct Postgres.** The build service reuses
   `internal/dispatch` + `internal/store` against the shared Managed Postgres
   over the VPC, exactly as hosting.md §5.2 describes ("the build runner polls
   this queue"). `ClaimNextBuildJob` already uses `FOR UPDATE SKIP LOCKED`, so
   embedded and standalone dispatchers can even coexist safely. The trust
   separation is unchanged: untrusted code runs only inside the per-language
   builder containers, which never receive DB credentials.
2. **Execution: Docker per build.** The build-runner Droplet runs Docker and
   the service launches the existing five builder images
   (`spade-builder-{go,rust,ts,python,r}`) pulled from DOCR — container-per-build
   isolation, no bundler snapshot to bake. This mirrors the interim
   `infra/worker.tf` pattern (Docker on a plain Ubuntu Droplet) and refines
   hosting.md §5.1's bundler-snapshot description the same way the worker
   refines §4.2.
3. **Registry flag: `BUILD_DISPATCH_ENABLED`, default `true`.** Gates only the
   in-process dispatcher goroutine. The `/builds/:id/*` callback endpoints stay
   on regardless — remote builders report over HTTP. Local/dev single-binary
   and docker-compose modes keep working unchanged; production App Platform
   sets it `false`.
4. **Stuck-job reaper: yes.** Once the dispatcher is remote, a crash between
   claim and terminal state strands jobs in `claimed`/`running`. A periodic
   sweep requeues them, with an attempt cap.

## Stage 1 — Registry dispatch flag

- `registry/internal/config/config.go`: add `BuildDispatchEnabled bool` to
  `RegistryConfig`, read from `BUILD_DISPATCH_ENABLED` (default `true`).
- `registry/cmd/registryd/main.go`: start the dispatcher goroutine only when
  enabled; log clearly in both cases ("build dispatch disabled; expecting an
  external build service").
- `registry/internal/config/config_test.go`: cover default + explicit false.

## Stage 2 — Stuck-job reaper

Lives in `internal/dispatch` so both the embedded and standalone dispatchers
get it (whichever dispatcher is running owns reaping).

- `internal/state/state.go`: add legal edges `screening → submitted` and
  `building → submitted` for **system actors only** (requeue path). Keep
  developer/operator authorization unchanged.
- `internal/store`: add `ListStuckBuildJobs(staleBefore time.Time)` (state in
  `claimed`,`running` and `updated_at < staleBefore`) and
  `RequeueBuildJob(id)` (state → `queued`, clear claim/token fields;
  `attempts` already increments at claim time).
- `internal/dispatch`: a reap pass inside `Run`'s loop (e.g. every
  `ReapInterval`, default 1m). Stale window = `BuildTimeout` + margin
  (e.g. 5m). For each stuck job:
  - `attempts < MaxAttempts` (default 3): requeue job, transition version back
    to `submitted` ("requeued: build runner did not complete").
  - else: job → `failed`, version → `failed` ("build abandoned after N attempts").
- Tests: state-machine edges, store queries, and a dispatcher test simulating
  a crash (job left `running`, reaper requeues, second dispatch succeeds; and
  the attempt-cap → failed path).

## Stage 3 — `cmd/buildrunnerd` (the build service)

New third binary in the registry module — the dispatcher half of registryd
without the API server:

- `registry/cmd/buildrunnerd/main.go`:
  - Config via `config.LoadRegistry()` reused, plus validation: require
    `DATABASE_URL` (a SQLite fallback would be a *different* database from
    registryd's — refuse), require `BUILDER_IMAGES`, and require
    `REGISTRY_URL` (the callback URL injected into builder containers;
    registryd's equivalent is `REGISTRY_INTERNAL_URL`).
  - Open Postgres store; construct `audit.Logger`, mirror
    (`MIRROR_ENABLED`, same default-on-Postgres rule as registryd),
    `state.Machine`, `dispatch.New` with `DockerLauncher{ExtraArgs:
    cfg.BuilderDockerArgs}`; run until SIGTERM.
  - Minimal `/healthz` HTTP listener (`LISTEN_ADDR`, default `:8091`) for
    Droplet monitoring.
- `registry/Dockerfile.buildrunnerd`: multi-stage Go build; runtime image needs
  the **docker CLI** (the launcher shells out to `docker`; the host socket is
  mounted at run time).
- Integration: extend `internal/integration` with a split-mode e2e — registryd
  with `BuildDispatchEnabled=false` + a separately constructed dispatcher
  (in-process launcher), asserting publish → build → available works with the
  control plane not dispatching.

## Stage 4 — Build scripts

- `build-buildrunner.sh`: `build_and_push "spade-buildrunner"
  "registry/Dockerfile.buildrunnerd" "."`.
- `build-builder-images.sh`: push all five builder images to DOCR
  (`spade-builder-<lang>`), since the Droplet pulls them rather than building
  locally like compose does.
- Add both to `build-all.sh`.
- docker-compose: unchanged (flag defaults `true`, embedded mode remains the
  local dev path).

## Stage 5 — Infra (Terraform)

- `infra/buildrunner.tf`:
  - `digitalocean_droplet.buildrunner`: `s-2vcpu-4gb` (hosting.md §5.3),
    Ubuntu 24.04, in the VPC, tag `buildrunner`, monitoring on.
  - `infra/cloud-init/buildrunner.sh.tftpl` (modeled on `worker.sh.tftpl`):
    install Docker CE, `docker login` to DOCR, pull `spade-buildrunner` + the
    five builder images, run the runner with `--restart always`,
    `-v /var/run/docker.sock:/var/run/docker.sock`, and env:
    `DATABASE_URL` (private-network URI for the `spade` DB, constructed from
    the cluster's private host/port/user/password), `REGISTRY_URL`
    (`${app.live_url}/registry`), `S3_*` (Spaces, path-style false),
    `BUILDER_IMAGES` (DOCR refs), `BUILD_TIMEOUT`. Same user-data secrets
    caveat as the worker: rotatable, root-only.
  - Firewall: inbound SSH from `admin_ips` only; all egress.
- `infra/database.tf`: add a `tag = "buildrunner"` rule to the authoritative
  `digitalocean_database_firewall` (the comment there already anticipates
  this). Declare the tag via `digitalocean_tag` so the rule doesn't depend on
  droplet ordering.
- `infra/app.tf`: registry service gains `BUILD_DISPATCH_ENABLED=false`;
  replace the "future stage" NOTE with a pointer to `buildrunner.tf`.
- `infra/variables.tf`: `buildrunner_size` (default `s-2vcpu-4gb`).
- `infra/outputs.tf`: build-runner IP.
- `infra/README.md`: document the new piece + Droplet-quota note
  (hosting.md §4.5: worker + broker + build runner ⇒ default 3-Droplet limit
  is fully consumed).

## Stage 6 — Docs

- `registry/README.md`: architecture table becomes three binaries
  (`registryd` / `buildrunnerd` / `builder`); document
  `BUILD_DISPATCH_ENABLED`, reaper knobs, and buildrunnerd's env.

## Verification

- `cd registry && go test ./... && go vet ./...` (includes the new split-mode
  e2e and reaper tests).
- Local: `docker compose up --build registry builder-go-image`, publish a
  fixture collection, confirm embedded mode still builds; then rerun with
  `BUILD_DISPATCH_ENABLED=false` on the registry plus a local `buildrunnerd`
  and confirm split mode builds.
- `cd infra && tofu validate && tofu plan` (plan-only; apply is an operator
  action).
