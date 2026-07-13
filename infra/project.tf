# Groups the backbone resources under one DO project for billing/visibility.
# (VPCs and container registries are not project-assignable resources.)
resource "digitalocean_project" "spade" {
  name        = var.project_name
  description = "Spade platform — data processing for massive geospatial data."
  purpose     = "Web Application"
  environment = var.environment

  resources = concat(
    [for b in digitalocean_spaces_bucket.this : b.urn],
    digitalocean_droplet.worker[*].urn,
    [
      digitalocean_database_cluster.postgres.urn,
      digitalocean_droplet.rabbitmq.urn,
    ],
  )
}
