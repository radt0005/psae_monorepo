# Private VPC shared by App Platform (outbound), Droplets (worker, build runner,
# RabbitMQ), and Managed Postgres. Inter-service traffic stays off the public
# internet (spec/hosting.md §8.1). ip_range is auto-assigned by DO.
resource "digitalocean_vpc" "spade" {
  name   = var.vpc_name
  region = var.region
}
