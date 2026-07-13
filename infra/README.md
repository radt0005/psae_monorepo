# Spade Infrastructure (OpenTofu / Terraform)

Infrastructure-as-code for the Spade platform on DigitalOcean, region **`nyc3`**
(New York) — see `../spec/hosting.md` for the full plan.

This directory currently provisions the **stateful backbone**:

| Resource | File | Notes |
| --- | --- | --- |
| Private VPC | `vpc.tf` | Shared by App Platform, Droplets, Postgres |
| Managed Postgres | `database.tf` | 1 GB tier, in-VPC, `spade` database |
| Spaces buckets ×4 | `spaces.tf` | artifacts / pipeline-io / user-data / worker-bin |
| Container registry | `registry.tf` | **Referenced** (already exists), not created |
| RabbitMQ broker | `rabbitmq.tf` | 1-Click Droplet in-VPC + firewall + generated password |
| Control plane app | `app.tf` | App Platform: web UI, scheduler, registry, KMS + migrate job + DB binding |
| Generated secrets | `secrets.tf` | Better Auth secret, worker callback secret, KMS KEK |
| Worker fleet | `worker.tf` | Droplet(s) running the spade-worker image as a privileged container |
| Build runner | `buildrunner.tf` | Droplet running spade-buildrunner (build dispatcher) + per-language builder containers |
| DO project | `project.tf` | Groups DB + buckets + broker + workers |

## Worker & build-runner notes

**Workers** run the `spade-worker` image (isolate + runtimes) as a **privileged
Docker container** pulled from DOCR — a deviation from the spec's baked-snapshot +
signed-binary model (§4.2/§4.3), which has no producer yet. Prereqs before `apply`:

```sh
./build-worker.sh                                   # push spade-worker to DOCR
export TF_VAR_docr_read_token="<DO token, registry read>"  # for the Droplet to pull
export TF_VAR_worker_registry_token="<or leave empty>"     # SPADE_WORKER_TOKEN (see below)
```

The worker will boot, connect to RabbitMQ (private IP), and consume jobs. Two things
gate it actually *running* pipeline blocks: (1) `SPADE_WORKER_TOKEN` — the registry
service token isn't provisioned yet (empty = fetch path dormant); (2) the worker
reaches the registry/KMS at the app's public `/registry` and `/kms` paths, which
depends on App Platform open item #3 (path-prefix routing) working.

**Build runner** (`buildrunner.tf`, spec §5): a Droplet running the
`spade-buildrunner` image with the host docker socket mounted. `buildrunnerd`
claims build jobs from the shared Postgres queue (trusted-source rule by the
`buildrunner` tag in `database.tf`) and launches one ephemeral per-language
builder container per build; the registry service on App Platform runs with
`BUILD_DISPATCH_ENABLED=false`. Prereqs before `apply`:

```sh
./build-buildrunner.sh      # push spade-buildrunner to DOCR
./build-builder-images.sh   # push spade-builder-{go,rust,ts,python,r} to DOCR
export TF_VAR_docr_read_token="<DO token, registry read>"  # shared with the worker
```

Builder containers report back to the registry at the app's public `/registry`
path (same App Platform open item #3 as the worker) and upload staged artifacts
to the `spade-artifacts` bucket.

**Droplet quota:** worker (1) + rabbitmq (1) + build runner (1) = 3, which is the
default account cap — request an increase before adding a second worker.

## Runtime secrets for App Platform

Four secrets are auto-generated (`secrets.tf`) and injected as encrypted env
vars. Four more you must supply — pass them as `TF_VAR_*` so they never touch a
file:

```sh
export TF_VAR_spaces_access_key_id="$SPACES_ACCESS_KEY_ID"
export TF_VAR_spaces_secret_access_key="$SPACES_SECRET_ACCESS_KEY"
export TF_VAR_scheduler_token_privkey="<base64 ed25519 private key>"
export TF_VAR_kms_token_pubkeys="<base64 ed25519 public key>"
```

### Capability-token keypair

The scheduler signs per-invocation capability tokens with an ed25519 private key;
the KMS verifies them with the matching public key (spec/secrets.md §6). Generate
a fresh pair with the repo's keygen command — it prints the values in the exact
base64 encoding the services parse (`captoken.ParsePrivateKey`/`ParsePublicKeys`):

```sh
go -C captoken run ./cmd/captoken-keygen
# SCHEDULER_TOKEN_PRIVKEY=<64-byte key, base64>
# KMS_TOKEN_PUBKEYS=<32-byte key, base64>
```

Export the two values into the matching (lowercase) Terraform vars:

```sh
export TF_VAR_scheduler_token_privkey="<SCHEDULER_TOKEN_PRIVKEY value>"
export TF_VAR_kms_token_pubkeys="<KMS_TOKEN_PUBKEYS value>"
```

**Do not reuse the docker-compose dev keys in production**, and keep the private
key out of shell history / files (generate it in the shell you run `tofu` from).

## App Platform open items

The app config is complete and valid, but these must be resolved before it runs:

1. **Push images to DOCR first.** App Platform pulls `spade-web-ui`, `spade-scheduler`,
   `spade-registry`, `spade-kms`, and `spade-migrate` (tag = `var.image_tag`) from
   `registry.digitalocean.com/spade-default-1`. The `apply` waits for a healthy
   deploy, so build + push all five before (or the deploy errors on missing images).
2. **Scheduler → RabbitMQ connectivity.** The scheduler runs on App Platform and
   can't reach the broker's private IP, so `SPADE_AMQP_URL` targets the broker's
   *public* IP. That needs a firewall grant (`broker_public_ingress_ips`, ideally
   an App Platform dedicated egress IP) and, before any real traffic, **TLS** (the
   1-Click broker is plaintext). CloudAMQP's public TLS endpoint sidesteps this
   entirely if it gets fiddly.
3. **registry & kms are path-routed** (`/registry`, `/kms`). Workers (Droplets)
   reach them at the app's public URL. Path-prefix routing only works if those
   services are base-path aware — otherwise give them their own subdomains or
   separate apps. Verify against the service code.
4. **Registry build dispatch.** No Docker daemon on App Platform, so the registry
   runs control-plane-only; the build-runner Droplet (next stage) claims build
   jobs off the Postgres queue. Confirm the registry starts without `BUILDER_*`.
5. **Bucket mapping.** `S3_BUCKET` for the web UI/worker is a single bucket
   (`var.app_data_bucket`); the spec splits pipeline-io vs user-data. Reconcile
   when the code supports separate bucket configs.

## SSH keys

Droplets need an SSH key or they're unreachable. Terraform registers one for you
from `ssh_public_key_path` (default `~/.ssh/id_ed25519.pub`) via `ssh.tf` — no
`doctl` write access needed. Override the path in `terraform.tfvars`, or set it
to `""` and pass `ssh_key_fingerprints` if the key already exists in DO.

Also set `admin_ips` to your own CIDR — it defaults to `0.0.0.0/0`, which leaves
SSH and the RabbitMQ management UI reachable from anywhere (key auth still
applies, but narrow it):

```hcl
# terraform.tfvars
admin_ips = ["203.0.113.4/32"]   # your office/VPN IP
```

Note: `doctl` itself reads `DIGITALOCEAN_ACCESS_TOKEN` (not the `DIGITALOCEAN_TOKEN`
the Terraform provider uses). If `doctl` write commands fail with "permission
denied," its stored token is read-only — re-auth with `doctl auth init` using a
write-scoped token.

## Prerequisites

- [OpenTofu](https://opentofu.org) ≥ 1.6 (`tofu`) — or Terraform ≥ 1.6.
- A DO API token and a Spaces access key pair.

Export credentials into your shell (nothing secret is committed):

```sh
export DIGITALOCEAN_TOKEN="dop_v1_..."
export SPACES_ACCESS_KEY_ID="..."
export SPACES_SECRET_ACCESS_KEY="..."
```

## First apply (local state)

```sh
cd infra
cp terraform.tfvars.example terraform.tfvars   # edit if needed
tofu init
tofu plan
tofu apply
```

## Remote state bootstrap (do once, then switch)

State holds secrets (DB password), so move it into Spaces after the first apply:

1. Create the state bucket (one-time, outside Terraform to avoid a chicken-and-egg
   with the backend). Using the AWS CLI against the Spaces endpoint:

   ```sh
   AWS_ACCESS_KEY_ID="$SPACES_ACCESS_KEY_ID" \
   AWS_SECRET_ACCESS_KEY="$SPACES_SECRET_ACCESS_KEY" \
   aws s3 mb s3://spade-tfstate \
     --region us-east-1 \
     --endpoint-url https://nyc3.digitaloceanspaces.com
   ```

2. Uncomment the `backend "s3"` block in `main.tf`.

3. The S3 backend reads `AWS_*` env vars, so mirror the Spaces keys:

   ```sh
   export AWS_ACCESS_KEY_ID="$SPACES_ACCESS_KEY_ID"
   export AWS_SECRET_ACCESS_KEY="$SPACES_SECRET_ACCESS_KEY"
   tofu init -migrate-state   # copies local state into the bucket
   ```

After migration, `terraform.tfstate` is remote; the local copy can be deleted.

## Container registry

`registry.tf` **references** the existing `spade-default-1` registry as a data
source. `tofu plan` will error with *"registry does not exist"* if the token in
`DIGITALOCEAN_TOKEN` belongs to a team that doesn't own it. (`doctl registry get`
currently 404s under the default context, so double-check which team owns the
registry.) Fix by exporting a token for the owning team, or adjust
`container_registry_name`.

## Region

Everything is provisioned in **`nyc3`** (New York). Atlanta (`atl1`) was the
initial preference for user proximity (UGA / Virginia Tech), but DO does not
offer Managed Postgres there, so the stack is consolidated in `nyc3` — the
nearest region hosting Postgres, Spaces, Droplets, and App Platform together
(and the only NY region with Spaces). See `../spec/hosting.md` §2.
