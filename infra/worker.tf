# Worker fleet (spec/hosting.md §4). Droplets in the VPC so they reach RabbitMQ
# over the private network. Each runs the spade-worker image as a privileged
# container (isolate needs kernel access). Interim approach: Docker container
# from DOCR rather than the spec's baked snapshot + signed-binary release, which
# has no producer yet — swap to Packer + a release pipeline when autoscaling.
#
# Workers are outbound-only: they consume jobs from RabbitMQ and call the
# registry/KMS over HTTPS. No inbound except SSH. They do NOT touch Postgres, so
# no DB trusted-source rule is needed.

resource "digitalocean_droplet" "worker" {
  count    = var.worker_count
  name     = "spade-worker-${count.index + 1}"
  image    = "ubuntu-24-04-x64"
  size     = var.worker_size
  region   = var.region
  vpc_uuid = digitalocean_vpc.spade.id
  ssh_keys = concat(
    var.ssh_key_fingerprints,
    digitalocean_ssh_key.admin[*].fingerprint,
  )
  monitoring = true
  tags       = ["spade", "worker"]

  user_data = templatefile("${path.module}/cloud-init/worker.sh.tftpl", {
    worker_image = "${data.digitalocean_container_registry.spade.endpoint}/spade-worker:${var.image_tag}"
    docr_token   = var.docr_read_token
    # Worker is in-VPC → private broker IP. Registry/KMS are on App Platform → public URL.
    amqp_url     = "amqp://${var.rabbitmq_user}:${random_password.rabbitmq.result}@${digitalocean_droplet.rabbitmq.ipv4_address_private}:5672/"
    registry_url = "${digitalocean_app.spade.live_url}/registry"
    kms_url      = "${digitalocean_app.spade.live_url}/kms"
    worker_token = var.worker_registry_token
  })
}

# Workers reachable only via SSH from admin_ips; all egress allowed.
resource "digitalocean_firewall" "worker" {
  name        = "spade-worker"
  droplet_ids = digitalocean_droplet.worker[*].id

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
