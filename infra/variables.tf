# Non-secret configuration. Secrets (API token, Spaces keys) come from the
# environment — see main.tf. Override any of these in terraform.tfvars.

variable "region" {
  description = "DigitalOcean region for VPC, Droplets, App Platform, and Managed Postgres."
  type        = string
  default     = "nyc3"
}

variable "spaces_region" {
  description = <<-EOT
    Region for Spaces buckets. Defaults to the deployment region; if Spaces is
    not offered there, pin this to the nearest Spaces-enabled region (e.g.
    "nyc3"). The app talks to an S3 endpoint, so cross-region only costs a
    little latency/egress (see spec/hosting.md §6.2).
  EOT
  type        = string
  default     = "nyc3"
}

variable "project_name" {
  description = "DigitalOcean project that groups the Spade resources."
  type        = string
  default     = "spade"
}

variable "environment" {
  description = "DO project environment tag (Development | Staging | Production)."
  type        = string
  default     = "Production"
}

variable "vpc_name" {
  description = "Name of the private VPC that all components share."
  type        = string
  default     = "spade-vpc"
}

# --- Managed Postgres -------------------------------------------------------

variable "db_name" {
  description = "Name of the Managed Postgres cluster."
  type        = string
  default     = "spade-postgres"
}

variable "db_size" {
  description = "Droplet size slug for the Postgres cluster (db-s-1vcpu-1gb ≈ $15/mo)."
  type        = string
  default     = "db-s-1vcpu-1gb"
}

variable "db_version" {
  description = "Postgres major version (nyc3 supports 15–18)."
  type        = string
  default     = "16"
}

variable "db_node_count" {
  description = "Postgres nodes (1 = single node, no standby)."
  type        = number
  default     = 1
}

variable "app_db_name" {
  description = "Application database created inside the cluster."
  type        = string
  default     = "spade"
}

variable "db_trusted_ips" {
  description = <<-EOT
    Bare IP addresses (NOT CIDRs) added to the Postgres trusted sources for
    direct access — e.g. your workstation for local psql / drizzle migrations.
    Empty means only the App Platform app can connect. Example: ["73.18.91.11"].
  EOT
  type        = list(string)
  default     = []
}

# --- Spaces -----------------------------------------------------------------

variable "bucket_suffix" {
  description = <<-EOT
    Optional suffix appended to every bucket name for global uniqueness. Spaces
    bucket names share a per-region namespace, so if "spade-artifacts" et al.
    are taken, set e.g. "-uga" to get "spade-artifacts-uga".
  EOT
  type        = string
  default     = ""
}

# --- RabbitMQ Droplet -------------------------------------------------------

variable "rabbitmq_size" {
  description = "Droplet size for the RabbitMQ broker (s-1vcpu-2gb ≈ $12/mo)."
  type        = string
  default     = "s-1vcpu-2gb"
}

variable "rabbitmq_user" {
  description = "RabbitMQ application username created at bootstrap."
  type        = string
  default     = "spade"
}

variable "ssh_public_key_path" {
  description = <<-EOT
    Path to a local SSH public key that Terraform registers with DO and installs
    on Droplets. Set to "" to skip creation and supply ssh_key_fingerprints
    instead (e.g. in CI where no local key exists).
  EOT
  type        = string
  default     = "~/.ssh/id_ed25519.pub"
}

variable "ssh_key_fingerprints" {
  description = <<-EOT
    Fingerprints (or IDs) of EXISTING DO SSH keys to also install on Droplets,
    in addition to the one created from ssh_public_key_path. Get them with
    `doctl compute ssh-key list`. Usually left empty.
  EOT
  type        = list(string)
  default     = []
}

variable "admin_ips" {
  description = <<-EOT
    CIDRs allowed to reach SSH (22) and the RabbitMQ management UI (15672).
    Defaults to the whole internet — NARROW THIS to your office/VPN CIDR. Key
    auth still protects SSH, but tightening the source is strongly advised.
  EOT
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "broker_public_ingress_ips" {
  description = <<-EOT
    CIDRs allowed to reach AMQP (5672) over the broker's PUBLIC IP — intended for
    the App Platform scheduler's egress IP. Leave empty until you have a stable
    egress IP (App Platform dedicated egress); do NOT set 0.0.0.0/0 (plaintext
    broker). See app.tf open item #2.
  EOT
  type        = list(string)
  default     = []
}

# --- App Platform -----------------------------------------------------------

variable "app_region" {
  description = "App Platform region slug (data-center group). For New York use \"nyc\"."
  type        = string
  default     = "nyc"
}

variable "image_tag" {
  description = "Tag to deploy for every service image in DOCR (push this tag, and the app rolls forward)."
  type        = string
  default     = "latest"
}

# Instance sizes per spec/hosting.md §3.3. Slugs may need adjustment if DO has
# renamed tiers (apply-time validation).
variable "scheduler_instance_size" {
  type    = string
  default = "professional-xs"
}
variable "web_ui_instance_size" {
  type    = string
  default = "basic-s"
}
variable "registry_instance_size" {
  type    = string
  default = "basic-s"
}
variable "kms_instance_size" {
  type    = string
  default = "basic-xs"
}

variable "app_data_bucket" {
  description = <<-EOT
    Bucket the web UI / worker use for pipeline I/O + user data via S3_BUCKET.
    NOTE: the app currently uses a SINGLE bucket here (docker-compose used
    "spade"), while the spec splits pipeline-io and user-data. Reconcile when the
    code grows separate bucket configs. Defaults to the pipeline-io bucket.
  EOT
  type        = string
  default     = "spade-pipeline-io"
}

variable "admin_user_ids" {
  description = "Comma-separated Better Auth user IDs allowed registry operator actions (recall / token admin)."
  type        = string
  default     = ""
}

# --- Secrets supplied by the operator (not generated) -----------------------

variable "spaces_access_key_id" {
  description = "Spaces access key for services at RUNTIME (S3 client). Pass via TF_VAR_spaces_access_key_id."
  type        = string
  sensitive   = true
}

variable "spaces_secret_access_key" {
  description = "Spaces secret key for services at RUNTIME. Pass via TF_VAR_spaces_secret_access_key."
  type        = string
  sensitive   = true
}

variable "scheduler_token_privkey" {
  description = <<-EOT
    Base64 ed25519 PRIVATE key the scheduler uses to sign capability tokens
    (spec/secrets.md §6). Must match kms_token_pubkeys. Generate a keypair in the
    app's expected raw-base64 encoding — see README §"Capability-token keypair".
  EOT
  type        = string
  sensitive   = true
}

variable "kms_token_pubkeys" {
  description = "Comma-separated base64 ed25519 PUBLIC key(s) the KMS trusts. Must match scheduler_token_privkey."
  type        = string
  sensitive   = true
}

# --- Worker fleet (Droplets) ------------------------------------------------

variable "worker_size" {
  description = "Droplet size for workers (spec §4.5: 4 vCPU / 8 GB / 80 GB ≈ $48/mo)."
  type        = string
  default     = "s-4vcpu-8gb"
}

variable "worker_count" {
  description = "Number of worker Droplets. Mind the account Droplet quota (worker + rabbitmq + any build runner)."
  type        = number
  default     = 1
}

variable "docr_read_token" {
  description = <<-EOT
    DO API token used ON the worker Droplet to `docker login` and pull the
    spade-worker image from DOCR. Prefer a read-scoped token. Pass via
    TF_VAR_docr_read_token. Empty = boot the Droplet but don't start the worker
    (skips login/pull).
  EOT
  type        = string
  sensitive   = true
  default     = ""
}

variable "worker_registry_token" {
  description = <<-EOT
    SPADE_WORKER_TOKEN — the registry service token the worker uses to fetch
    signed collection artifacts (registry.md §7.2). Not yet provisioned; empty
    leaves the registry-fetch path dormant (worker still consumes jobs).
  EOT
  type        = string
  sensitive   = true
  default     = ""
}

# --- Build runner (Droplet) --------------------------------------------------

variable "buildrunner_size" {
  description = "Droplet size for the registry build runner (spec §5.3: 2 vCPU / 4 GB ≈ $24/mo)."
  type        = string
  default     = "s-2vcpu-4gb"
}

# --- Container registry (pre-existing) --------------------------------------

variable "container_registry_name" {
  description = "Name of the EXISTING DO Container Registry (referenced, not created)."
  type        = string
  default     = "spade-default-1"
}
