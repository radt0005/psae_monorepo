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

# The data source's `.endpoint` is null unless the token carries registry-read
# scope, so derive it from the (deterministic) DOCR format instead. The endpoint
# path is global — `registry.digitalocean.com/<name>` — independent of the
# registry's storage region. The data source is kept only to assert the registry
# exists at plan time.
locals {
  docr_endpoint = "registry.digitalocean.com/${var.container_registry_name}"
}
