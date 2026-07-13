# Registry build runner (spec/hosting.md §5). A single Droplet in the VPC that
# claims build jobs from the shared Postgres queue (buildrunnerd) and launches
# an ephemeral per-language builder container per job (registry.md §5.2). The
# registry control plane on App Platform runs with BUILD_DISPATCH_ENABLED=false
# and never dispatches builds itself (App Platform has no Docker daemon).
#
# Interim approach mirrors worker.tf: Docker containers pulled from DOCR rather
# than the spec's bundler snapshot — container-per-build isolation stands in
# for per-build temp dirs. Untrusted collection code runs only inside the
# builder containers, which get a per-job token and staging S3 credentials,
# never the DATABASE_URL held by the runner.
#
# The runner is outbound-only: Postgres over the VPC, registry callbacks and
# Spaces over HTTPS, git clones of published collections. No inbound except SSH.

locals {
  # Language keys must match core.CollectionLanguage (what `POST /publish`
  # records); image short-names match the registry/Dockerfile.builder-* files.
  builder_langs = { go = "go", rust = "rust", typescript = "ts", python = "python", r = "r" }
  builder_images_map = { for lang, short in local.builder_langs :
    lang => "${local.docr_endpoint}/spade-builder-${short}:${var.image_tag}"
  }
}

# Explicit tag so the Postgres trusted-source rule (database.tf) has a stable
# handle that doesn't depend on Droplet ordering.
resource "digitalocean_tag" "buildrunner" {
  name = "buildrunner"
}

resource "digitalocean_droplet" "buildrunner" {
  name     = "spade-buildrunner"
  image    = "ubuntu-24-04-x64"
  size     = var.buildrunner_size
  region   = var.region
  vpc_uuid = digitalocean_vpc.spade.id
  ssh_keys = concat(
    var.ssh_key_fingerprints,
    digitalocean_ssh_key.admin[*].fingerprint,
  )
  monitoring = true
  tags       = ["spade", digitalocean_tag.buildrunner.name]

  user_data = templatefile("${path.module}/cloud-init/buildrunner.sh.tftpl", {
    runner_image       = "${local.docr_endpoint}/spade-buildrunner:${var.image_tag}"
    builder_images     = join(",", [for lang, img in local.builder_images_map : "${lang}=${img}"])
    builder_image_list = join(" ", values(local.builder_images_map))
    docr_token         = var.docr_read_token
    # In-VPC private URI to the app database — the same build queue registryd
    # writes publish requests to.
    database_url             = "postgresql://${digitalocean_database_cluster.postgres.user}:${digitalocean_database_cluster.postgres.password}@${digitalocean_database_cluster.postgres.private_host}:${digitalocean_database_cluster.postgres.port}/${digitalocean_database_db.spade.name}?sslmode=require"
    registry_url             = "${digitalocean_app.spade.live_url}/registry"
    spaces_endpoint          = local.spaces_endpoint
    spaces_region            = var.spaces_region
    artifacts_bucket         = digitalocean_spaces_bucket.this["artifacts"].name
    spaces_access_key_id     = var.spaces_access_key_id
    spaces_secret_access_key = var.spaces_secret_access_key
  })
}

# Build runner reachable only via SSH from admin_ips; all egress allowed.
resource "digitalocean_firewall" "buildrunner" {
  name        = "spade-buildrunner"
  droplet_ids = [digitalocean_droplet.buildrunner.id]

  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = var.admin_ips
  }

  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
  outbound_rule {
    protocol              = "icmp"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}
