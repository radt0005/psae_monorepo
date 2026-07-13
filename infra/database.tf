# Single Managed Postgres cluster: app data, Better Auth, registry metadata
# mirror, scheduler DAG/invocation state, registry build queue, KMS ciphertext,
# and the upload catalog (spec/hosting.md §6.1). Attached to the private VPC.
resource "digitalocean_database_cluster" "postgres" {
  name                 = var.db_name
  engine               = "pg"
  version              = var.db_version
  size                 = var.db_size
  region               = var.region
  node_count           = var.db_node_count
  private_network_uuid = digitalocean_vpc.spade.id

  tags = ["spade", "backbone"]
}

# Application database inside the cluster (alongside the default `defaultdb`).
resource "digitalocean_database_db" "spade" {
  cluster_id = digitalocean_database_cluster.postgres.id
  name       = var.app_db_name
}

# Trusted sources: restrict the cluster to the App Platform app, the build
# runner, plus any admin IPs. Everything else is denied — DO Managed Postgres has
# no "allow all" default once any rule exists. This resource is AUTHORITATIVE (it
# sets the full list), so the app rule must stay here; dropping it would cut the
# control plane off from the DB. Trusted sources are by TYPE (app / droplet /
# tag / k8s / ip_addr) — VPC CIDRs are not a valid type. Workers do not touch
# Postgres and are intentionally absent.
resource "digitalocean_database_firewall" "postgres" {
  cluster_id = digitalocean_database_cluster.postgres.id

  # The whole App Platform app (all services + the migrate job).
  rule {
    type  = "app"
    value = digitalocean_app.spade.id
  }

  # The build runner Droplet (buildrunner.tf): buildrunnerd claims build jobs
  # from the shared Postgres queue.
  rule {
    type  = "tag"
    value = digitalocean_tag.buildrunner.name
  }

  # Optional admin IPs (bare addresses, no CIDR) for local psql / migrations.
  dynamic "rule" {
    for_each = toset(var.db_trusted_ips)
    content {
      type  = "ip_addr"
      value = rule.value
    }
  }
}
