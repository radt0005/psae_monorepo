# Hosting

This document specifies the production hosting plan for the Spade platform.  It maps each system component to a concrete DigitalOcean service and records the architectural decisions that shape the deployment.

The target deployment is a single shared instance operated by the Spade team and used by stakeholders across multiple institutions.  The plan is sized for a small-but-growing user base with no dedicated operations team, and prioritises **operational simplicity** over peak performance or absolute lowest cost.  Other deployments (e.g. a future self-hosted copy by another institution) may adapt this plan but are not its primary concern.

---

## 1. Headline Decisions

Two architectural decisions shape the rest of this document.  They are specified in the component specs; this section just records the hosting-relevant consequence:

1. **Object storage replaces the shared filesystem.**  The architecture is specified in `core.md` (Architecture) and `worker.md` (Storage Model).  The hosting consequence is no managed NFS baseline cost: workers run on standalone Droplets with local NVMe, and data between blocks moves through Spaces.

2. **No worker affinity scheduling on day one.**  The scheduler dispatches jobs via competing consumers with no locality awareness.  The message-format reservation supporting future affinity is in `worker.md` (Communication, Job dispatch); the per-invocation telemetry that enables the eventual measurement is in `worker.md` (Telemetry).  The hosting consequence is that workers are interchangeable from the broker's perspective and can be added or removed without coordination.

Decisions deferred:

- A second RabbitMQ queue for affinity-routed jobs (see `worker.md` for the current single-queue model).
- A managed CDN in front of Spaces for user-facing downloads.
- Multi-region deployment.

---

## 2. Service Topology

| Component | DigitalOcean service | Notes |
| --- | --- | --- |
| Scheduler | App Platform (containerised) | Restart-tolerant; state in Postgres + RabbitMQ |
| Web UI | App Platform (containerised) | Frontend + API for the flowchart editor and results browser |
| Registry control plane | App Platform (containerised) | `POST /publish`, state transitions, audit log, metadata mirror writes |
| KMS (secrets) | App Platform (containerised) | Envelope crypto and authorization; holds the key-encryption key; smallest tier (see `secrets.md`) |
| Homepage | App Platform static site | Free tier (3 static sites included) |
| Documentation site | App Platform static site | Free tier |
| Worker fleet | Droplets from a custom snapshot | Must run `isolate`; requires kernel-level access not available on App Platform |
| Registry build runner | Droplet from a separate bundler snapshot | Ephemeral builds triggered by registry control plane |
| Application database | Managed PostgreSQL | App data, Better Auth, registry metadata mirror |
| Object storage | Spaces (S3-compatible) | Block artifacts, intermediate pipeline I/O, user uploads, signed worker-binary releases |
| Message broker | CloudAMQP | RabbitMQ; DO has no native offering |
| Container registry | DO Container Registry | Source for App Platform deploys |

App Platform and Droplets live in the same DigitalOcean project and private VPC.  Managed PostgreSQL is reachable only over the private VPC.  CloudAMQP is the one external dependency.

---

## 3. Control Plane (App Platform)

The scheduler, web UI, and registry control plane are containerised and deployed through DO App Platform.  The homepage and documentation site are deployed as App Platform static sites.

### 3.1 Deployment Flow

1. CI builds container images for each service.
2. Images are pushed to DO Container Registry.
3. App Platform detects new images and rolls each service forward with zero-downtime deploys.

There is no separate "deploy" step in CI -- pushing the image *is* the deploy.

### 3.2 Restart Behaviour on App Platform

App Platform will restart service containers during deploys, health-check failures, and platform maintenance.  Every control-plane service must therefore tolerate restart without external coordination.

The scheduler is the service with non-trivial state to recover; its restart contract is specified in `scheduler.md` (State Management section).  The web UI and registry control plane have only request-scoped state and tolerate restart trivially.

### 3.3 Sizing Guidance

| Service | Tier | Rationale |
| --- | --- | --- |
| Scheduler | Pro-XS (dedicated 1 vCPU, 1 GB) | Long-running coordination; benefits from dedicated CPU |
| Web UI | Basic-S (1 vCPU, 1 GB) | Standard HTTP service; can scale horizontally if needed |
| Registry control plane | Basic-S (1 vCPU, 1 GB) | API service; build orchestration is delegated to the build runner |

These are starting tiers.  Each service can scale up or out as load demands; App Platform supports both.

---

## 4. Worker Fleet (Droplets)

Workers run as Droplets, not as App Platform containers.  The worker uses the Ubuntu `isolate` sandbox (see `worker.md`), which requires kernel-level access and a privileged install path.  App Platform's runtime model does not permit this.

### 4.1 Why Not Kubernetes

The fleet is small enough (single-digit Droplets at launch) that Kubernetes-level orchestration is overhead, not savings.  Direct Droplet management with snapshot-based provisioning keeps the operational surface minimal.  If the fleet grows to a size where manual rotation becomes a real pain point, migrating workers to DOKS is a contained change -- the control plane stays where it is.

### 4.2 Snapshot-Based Provisioning

A custom DO snapshot is the worker base image.  It contains:

- Ubuntu LTS base
- `isolate` built from source and installed setuid (see `install_isolate.sh` in the repo root)
- Language runtimes: `python3` + `uv`, `R` + `pak`, GDAL system library, Bun
- The worker user account and `subuid`/`subgid` configuration
- No build toolchains (`cargo`, `go`, `gcc`, language-dev headers) -- those live in the bundler snapshot (§5)

The snapshot does **not** contain the worker binary itself.  See §4.4.

### 4.3 Per-Worker Configuration via Cloud-Init

Each Droplet's first-boot cloud-init script handles the bits that vary per-worker or change frequently:

1. Fetch the latest signed worker binary from Spaces and verify its signature against a bundled public key
2. Write RabbitMQ credentials, registry service token, and Spaces credentials from secrets injected via DO's user-data mechanism
3. Generate a stable worker ID
4. Start the worker as a `systemd` service

This separation means routine worker-binary releases do **not** require re-baking the snapshot; the binary is pulled fresh on every new Droplet boot and rotated workers always pick up the current version.

### 4.4 Snapshot Update Workflow

The snapshot is re-baked when something heavy changes -- runtime version bumps, OS patches, `isolate` updates, new system libraries.  The workflow:

1. Provision a Droplet from the current snapshot
2. Apply the changes (`apt upgrade`, runtime version bump, etc.) via the provisioning script in the repo
3. Take a new snapshot via the DO API or `doctl compute droplet-action snapshot`
4. Update the snapshot reference in the worker-provisioning configuration
5. Roll the fleet by terminating one worker at a time -- in-flight jobs complete and ack before termination; queued jobs are picked up by remaining workers

No coordinated shutdown is required because the broker redelivery semantics in `worker.md` already cover worker termination.

Snapshot storage cost is $0.06/GB-month; a ~30 GB worker snapshot is about $2/month and is not a meaningful line item.

### 4.5 Sizing Guidance

Starting Droplet: 4 vCPU / 8 GB / 80 GB NVMe (~$48/month).  Local NVMe sizing is the constraint that matters most -- it holds the worker-local input cache (see §1).  80 GB is sufficient for the cache + working set of a few concurrent invocations on common geospatial workloads.  Workers processing larger reference datasets may need 160 GB or 320 GB Droplets.

The fleet starts at 1 Droplet and scales by adding more Droplets from the same snapshot.  Autoscaling is manual at first; an autoscaling implementation may be added later if demand justifies it.

---

## 5. Registry Build Runner

The registry's screen-build-sign pipeline (see `registry.md`) runs on a dedicated Droplet, separate from the worker fleet.  Build jobs are ephemeral and have a fundamentally different trust profile from the worker: the build runner executes **untrusted source code** during compilation, whereas workers only execute artifacts already screened and signed by the registry.

### 5.1 Bundler Snapshot

The build runner uses its own snapshot -- the **bundler image** -- which contains everything in the worker snapshot plus the language build toolchains: `cargo`, `go`, `bun`, `gcc`/`clang`, GDAL development headers, R development headers, and `uv`.

The bundler snapshot is the strict superset of the worker snapshot.  When the worker snapshot is re-baked, the bundler snapshot is re-baked from the new base with the toolchain layer reapplied.

### 5.2 Trigger and Lifecycle

The registry control plane (running on App Platform) receives `POST /publish` requests and writes them to a Postgres queue with state `submitted` (see `registry.md`).  The build runner polls or subscribes to this queue, claims one job at a time, and executes the trust chain: clone → screen → build → sign → upload artifact to Spaces → update Postgres state.

The build process should isolate each build in a fresh temporary directory and reset the working state between builds.  Because the build runner is single-tenant and ephemeral per build, a full Droplet rotation between builds is unnecessary in this deployment; per-build directory isolation is sufficient.  If the trust model later requires stronger isolation, the build runner can be replaced with one-shot Droplets created and destroyed per build at modest cost increase.

### 5.3 Sizing Guidance

Starting Droplet: 2 vCPU / 4 GB (~$24/month).  Builds are not concurrent at launch -- the build runner processes one publish request at a time.  Higher tiers may be warranted once publish volume justifies parallel builds.

---

## 6. Managed Stateful Services

### 6.1 Managed PostgreSQL

A single Managed PostgreSQL cluster holds:

- Application data (pipelines, results, user accounts)
- Better Auth's user identity tables
- The registry metadata mirror (see `registry.md` §10)
- The scheduler's pipeline DAG and invocation state (see §3.2)
- The registry's submitted-build queue (see §5.2)
- The KMS's envelope-encrypted secret ciphertext (see `secrets.md` §5.1) -- ciphertext only; the key-encryption key never touches the database
- The user-data catalog: metadata for uploaded assets (see `uploads.md` §3) -- the bytes live in Spaces (§6.2)

Starting tier: 1 GB RAM / 10 GB storage ($15/month).  This is small but sufficient for early stage; upgrading tiers is a non-disruptive operation on DO Managed Postgres.

The database is reachable only over the private VPC.  No public-internet exposure.

### 6.2 Spaces

Spaces is the single object-storage backend for everything that isn't relational data:

- Signed collection artifact tarballs (and their `.sig` files) -- the registry's output
- Intermediate pipeline I/O between blocks (the "object storage instead of shared FS" decision)
- Persistent user uploads (custom data referenced from pipelines; catalogued in Postgres and served to blocks via pre-signed URLs -- see `uploads.md`)
- Signed worker-binary releases consumed by cloud-init (§4.3)
- The registry's public-key set (`/pubkeys` endpoint contents, served via App Platform but stored here)

Spaces is S3-compatible, which keeps the worker and registry code portable to AWS or other clouds if the deployment is ever moved.  Spaces includes 250 GB and 1 TB of egress per Space at base pricing ($5/month); usage above those tiers is metered.

Buckets are organised by component, not by tenant:

```
spade-artifacts/      # registry-built collection artifacts
spade-pipeline-io/    # intermediate pipeline I/O, keyed by invocation ID
spade-user-data/      # uploads from authenticated users
spade-worker-bin/     # signed worker binaries by version
```

Access policy is enforced via per-bucket access keys delivered to services through DO's secrets mechanism.  No bucket is publicly readable.

### 6.3 CloudAMQP

CloudAMQP provides hosted RabbitMQ.  The Spade broker holds two durable queues (`spade.jobs` and `spade.results`) as specified in `worker.md`.

Starting plan: "Tough Tiger" tier (~$19/month).  This is sized for small fleets and modest message volume; upgrading is non-disruptive.

CloudAMQP is the one external (non-DO) service in the stack.  The connection is over TLS to the CloudAMQP-managed endpoint.  If hosting it on a DO Droplet eventually becomes preferred (for VPC locality or cost), the migration is a queue drain and reconnect -- workers and the scheduler do not care which RabbitMQ they speak to.

### 6.4 DO Container Registry

The container registry holds images for the App Platform services.  Starter tier ($5/month).  Images for the worker and build runner are **not** in the container registry -- the worker binary is distributed as a signed binary in Spaces (§6.2), and the build runner does not run as a container.

---

## 7. Authentication and Identity

Identity is provided by **Better Auth**, replacing the earlier PocketBase plan.  Better Auth runs as part of the web UI service on App Platform and stores user data in the same Managed PostgreSQL cluster (§6.1).

Both the web UI and the CLI authenticate through Better Auth:

- Web users: standard Better Auth web sign-in flow
- CLI users: `spade login` performs an OAuth-style flow against the Better Auth endpoint and stores credentials locally for `spade publish` and other authenticated commands (see `cli.md`)

Workers do **not** authenticate via Better Auth.  They authenticate to the registry's fetch endpoints with a rotated service token provisioned at worker setup (see `registry.md` §7.2).

The detailed Better Auth deployment plan -- session lifetimes, provider configuration, recovery flows -- is the topic of a separate hosting subsection to be added when authentication is built out.

---

## 8. Networking

### 8.1 Private VPC

A single DO VPC contains:

- App Platform services (App Platform participates in DO VPC for outbound)
- Worker Droplets
- Build runner Droplet
- Managed PostgreSQL

Inter-service traffic stays on the private network.  Managed Postgres is reachable only from within the VPC.  Spaces is reachable from VPC endpoints without traversing the public internet.

### 8.2 Public Exposure

Only two surfaces are exposed to the public internet:

- The web UI / API origin (HTTPS via App Platform's managed load balancer and TLS)
- The registry's public read endpoints (`/collections`, `/artifacts/...`, `/pubkeys`) served by the registry control plane on App Platform

Workers, the build runner, the database, and the message broker connection are not directly reachable from the public internet.

### 8.3 External Dependencies

The deployment has two external network dependencies:

- CloudAMQP (the message broker)
- Git remotes (cloned by the build runner during the registry's screen-build step)

Both are outbound-only from the perspective of the private VPC.

---

## 9. Cost Model

Approximate monthly cost for the launch deployment.  Numbers are illustrative based on early-2026 DO pricing and will drift; the structure is the point, not the exact figures.

| Item | Monthly cost |
| --- | --- |
| App Platform: scheduler (Pro-XS) | ~$29 |
| App Platform: web UI (Basic-S) | ~$12 |
| App Platform: registry control plane (Basic-S) | ~$12 |
| App Platform: KMS / secrets (Basic-XS) | ~$12 |
| App Platform: static sites (homepage + docs) | $0 (within free tier) |
| DO Container Registry (starter) | $5 |
| Worker Droplet × 1 (4 vCPU / 8 GB / 80 GB NVMe) | $48 |
| Build runner Droplet (2 vCPU / 4 GB) | $24 |
| Managed PostgreSQL (1 GB tier) | $15 |
| Spaces | $5 + usage |
| CloudAMQP ("Tough Tiger") | $19 |
| Worker snapshot storage | ~$2 |
| **Baseline total** | **~$183/month** |
| With a second worker | ~$231/month |

At the project's $10,000 budget, the baseline supports ~4.5 years of runway and the two-worker configuration supports ~3.6 years.  Adding workers scales linearly with Droplet cost; everything else holds steady until usage drives the storage or database tiers up.

---

## 10. Scaling Path

The expected order of scaling pressure:

1. **Worker fleet.**  Add Droplets from the same snapshot.  Manual at first; an autoscaler can be added later by reading queue depth from CloudAMQP and provisioning Droplets via the DO API.
2. **PostgreSQL tier.**  Upgrade vertically when the registry mirror or invocation history outgrows the starter tier.  Non-disruptive on DO Managed.
3. **Spaces usage.**  Grows naturally with stored data and egress.  No structural change required.
4. **App Platform service tiers.**  Scale individual services horizontally or vertically as load demands.  Scheduler is likely the first to need attention as concurrent pipeline count grows.
5. **CloudAMQP plan.**  Upgrade if message rate or queue depth pushes the current tier.

Earlier deferred items become relevant once measurement justifies them: per-worker affinity routing, multi-region replication, a managed CDN in front of Spaces for public download performance.

---

## 11. Out of Scope

The following are intentionally not specified here and are deferred to follow-up documents or implementation choice:

- Backup and restore procedures for PostgreSQL and Spaces
- Monitoring, alerting, and dashboard stack
- Disaster recovery and RPO/RTO targets
- Multi-region replication
- CDN in front of Spaces for public downloads
- The detailed Better Auth deployment plan (§7)
- Autoscaling implementation for the worker fleet
- Cost optimisation for storage lifecycle (Spaces lifecycle rules for old artifacts and pipeline I/O)
- A self-hosted deployment guide for other institutions adopting the same architecture
