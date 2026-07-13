# Registers a local SSH public key with DO so Droplets are reachable. Terraform
# creates it (the provider's token has write access — it built the backbone),
# which sidesteps needing doctl write scope. Set ssh_public_key_path = "" to skip
# creation (e.g. in CI) and pass ssh_key_fingerprints instead.
resource "digitalocean_ssh_key" "admin" {
  count      = var.ssh_public_key_path != "" ? 1 : 0
  name       = "spade-admin"
  public_key = file(pathexpand(var.ssh_public_key_path))
}
