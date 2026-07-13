# The DO Container Registry already exists (registry.digitalocean.com/spade-default-1),
# so it is REFERENCED, not managed here. App Platform pulls service images from it.
#
# NOTE: `tofu plan` fails here if the doctl/API token in your environment points
# at a team that does not own this registry. If that happens, either switch to
# the owning team's token or confirm the registry name. See README §"Container
# registry".
data "digitalocean_container_registry" "spade" {
  name = var.container_registry_name
}
