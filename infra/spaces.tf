# Object storage, organised by component (spec/hosting.md §6.2). No bucket is
# public. Names share a per-region namespace; var.bucket_suffix disambiguates
# if a base name is already taken.
locals {
  buckets = {
    artifacts   = "spade-artifacts"   # registry-built collection artifacts + .sig
    pipeline_io = "spade-pipeline-io" # intermediate pipeline I/O, keyed by invocation
    user_data   = "spade-user-data"   # uploads from authenticated users
    worker_bin  = "spade-worker-bin"  # signed worker binaries by version
  }
}

resource "digitalocean_spaces_bucket" "this" {
  for_each = local.buckets

  name   = "${each.value}${var.bucket_suffix}"
  region = var.spaces_region
  acl    = "private"
}
