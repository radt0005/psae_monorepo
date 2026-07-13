terraform {
  required_version = ">= 1.6.0"

  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "2.95.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # ---------------------------------------------------------------------------
  # Remote state (DigitalOcean Spaces, S3-compatible).
  #
  # Commented out for the FIRST apply so state starts local.  Once the state
  # bucket exists (see README §"Remote state bootstrap"), uncomment this block
  # and run `tofu init -migrate-state` to move state into Spaces.
  #
  # `region` must be a real AWS region string for the AWS SDK's benefit; Spaces
  # ignores it.  The `endpoints.s3` value selects the actual Spaces region.
  # ---------------------------------------------------------------------------
  # backend "s3" {
  #   bucket = "spade-tfstate"
  #   key    = "backbone/terraform.tfstate"
  #   region = "us-east-1"
  #
  #   endpoints = {
  #     s3 = "https://nyc3.digitaloceanspaces.com"
  #   }
  #
  #   skip_credentials_validation = true
  #   skip_requesting_account_id  = true
  #   skip_metadata_api_check     = true
  #   skip_region_validation      = true
  #   skip_s3_checksum            = true
  #   use_path_style              = true
  # }
}

# The provider reads credentials from the environment:
#   DIGITALOCEAN_TOKEN         — API token (Droplets, DB, VPC, registry, project)
#   SPACES_ACCESS_KEY_ID       — Spaces access key (bucket resources)
#   SPACES_SECRET_ACCESS_KEY   — Spaces secret key
# Nothing secret is committed; see terraform.tfvars.example for non-secret knobs.
provider "digitalocean" {}
