output "vpc_id" {
  description = "ID of the shared private VPC."
  value       = digitalocean_vpc.spade.id
}

output "vpc_ip_range" {
  description = "CIDR range DO assigned to the VPC."
  value       = digitalocean_vpc.spade.ip_range
}

output "db_cluster_id" {
  description = "Managed Postgres cluster ID."
  value       = digitalocean_database_cluster.postgres.id
}

output "db_private_host" {
  description = "VPC-private Postgres host (use this from in-VPC services)."
  value       = digitalocean_database_cluster.postgres.private_host
}

output "db_port" {
  value = digitalocean_database_cluster.postgres.port
}

output "app_database_name" {
  value = digitalocean_database_db.spade.name
}

output "db_private_uri" {
  description = "Full private connection URI (contains the password)."
  value       = digitalocean_database_cluster.postgres.private_uri
  sensitive   = true
}

output "spaces_bucket_domains" {
  description = "Per-component Spaces bucket domain names."
  value       = { for k, b in digitalocean_spaces_bucket.this : k => b.bucket_domain_name }
}

output "spaces_endpoint" {
  description = "Regional Spaces S3 endpoint."
  value       = "https://${var.spaces_region}.digitaloceanspaces.com"
}

output "container_registry_endpoint" {
  description = "Registry endpoint (registry.digitalocean.com/<name>)."
  value       = local.docr_endpoint
}

output "rabbitmq_private_ip" {
  description = "VPC-private IP of the RabbitMQ Droplet (workers connect here)."
  value       = digitalocean_droplet.rabbitmq.ipv4_address_private
}

output "rabbitmq_public_ip" {
  description = "Public IP of the RabbitMQ Droplet (SSH / management UI)."
  value       = digitalocean_droplet.rabbitmq.ipv4_address
}

output "rabbitmq_management_url" {
  description = "RabbitMQ management console (reachable from admin_ips only)."
  value       = "http://${digitalocean_droplet.rabbitmq.ipv4_address}:15672"
}

output "rabbitmq_amqp_private_uri" {
  description = "AMQP URI over the private network — for in-VPC workers."
  value       = "amqp://${var.rabbitmq_user}:${random_password.rabbitmq.result}@${digitalocean_droplet.rabbitmq.ipv4_address_private}:5672/"
  sensitive   = true
}

output "app_live_url" {
  description = "Public URL of the App Platform app (web UI at /, registry at /registry, kms at /kms)."
  value       = digitalocean_app.spade.live_url
}

output "app_id" {
  value = digitalocean_app.spade.id
}

output "app_default_ingress" {
  description = "App Platform default ingress hostname."
  value       = digitalocean_app.spade.default_ingress
}

output "buildrunner_public_ip" {
  description = "Public IP of the build-runner Droplet (SSH)."
  value       = digitalocean_droplet.buildrunner.ipv4_address
}

output "buildrunner_private_ip" {
  description = "VPC-private IP of the build-runner Droplet."
  value       = digitalocean_droplet.buildrunner.ipv4_address_private
}

output "worker_public_ips" {
  description = "Public IPs of worker Droplets (SSH)."
  value       = digitalocean_droplet.worker[*].ipv4_address
}

output "worker_private_ips" {
  description = "VPC-private IPs of worker Droplets."
  value       = digitalocean_droplet.worker[*].ipv4_address_private
}
