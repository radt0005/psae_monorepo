# Self-hosted RabbitMQ broker (spec/hosting.md §6.3). Runs on a Droplet inside
# the VPC so workers reach it over the private network. Provisioned from the DO
# Marketplace "rabbitmq" 1-Click image; a cloud-init script creates the Spade
# application user.
#
# Scheduler connectivity note: App Platform services do not join the VPC to
# reach a Droplet's PRIVATE IP, so the scheduler will connect over the public IP
# (+ TLS, once configured) with a firewall grant added at the App Platform
# stage. Workers (in-VPC Droplets) use the private URI below.

resource "random_password" "rabbitmq" {
  length  = 32
  special = false # keep it URL-safe for amqp://user:pass@host
}

resource "digitalocean_droplet" "rabbitmq" {
  name     = "spade-rabbitmq"
  image    = "rabbitmq" # DO Marketplace 1-Click slug
  size     = var.rabbitmq_size
  region   = var.region
  vpc_uuid = digitalocean_vpc.spade.id
  ssh_keys = concat(
    var.ssh_key_fingerprints,
    digitalocean_ssh_key.admin[*].fingerprint,
  )
  monitoring = true

  user_data = templatefile("${path.module}/cloud-init/rabbitmq.sh.tftpl", {
    rabbitmq_user     = var.rabbitmq_user
    rabbitmq_password = random_password.rabbitmq.result
  })

  tags = ["spade", "rabbitmq"]
}

# Locks the broker down: AMQP (5672) only from inside the VPC; SSH and the
# management UI (15672) only from admin_ips. Everything else is denied inbound.
resource "digitalocean_firewall" "rabbitmq" {
  name        = "spade-rabbitmq"
  droplet_ids = [digitalocean_droplet.rabbitmq.id]

  # SSH — restrict to admin_ips (default is the whole internet; narrow it).
  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = var.admin_ips
  }

  # AMQP — VPC (workers) plus any App Platform egress IPs (scheduler). Keep
  # broker_public_ingress_ips empty until you have a stable App Platform egress
  # IP; opening 5672 to the world would expose a plaintext broker.
  inbound_rule {
    protocol         = "tcp"
    port_range       = "5672"
    source_addresses = concat([digitalocean_vpc.spade.ip_range], var.broker_public_ingress_ips)
  }

  # Management UI — admin_ips only.
  inbound_rule {
    protocol         = "tcp"
    port_range       = "15672"
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
