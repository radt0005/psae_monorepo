# Generated secrets for the control plane. Stored in Terraform state — move state
# to Spaces (README §"Remote state bootstrap") so they're encrypted at rest.
#
# The KEK is special: if it is ever lost, every envelope-encrypted secret in the
# KMS becomes undecryptable. Losing state = losing the KEK. Back up state.

resource "random_password" "better_auth_secret" {
  length  = 48
  special = false
}

resource "random_password" "worker_callback_secret" {
  length  = 48
  special = false
}

# 32-byte AES key-encryption key, base64-encoded, in the "id:material" form the
# KMS expects (spec/secrets.md §5.1). Active id is "v1"; rotation adds "v2:..".
resource "random_id" "kek" {
  byte_length = 32
}
