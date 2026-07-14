# Control plane on App Platform (spec/hosting.md §3): web UI, scheduler, registry
# control plane, KMS — all pulled from DOCR — plus a pre-deploy DB migration job.
# The managed Postgres cluster is bound as an app database, which auto-injects
# connection env vars AND adds this app to the cluster's trusted sources.
#
# ── OPEN ITEMS (see README §"App Platform open items") ──────────────────────
#   1. Images must be built + pushed to DOCR before the first deploy succeeds.
#   2. Scheduler → RabbitMQ crosses out of the VPC: uses the broker PUBLIC IP,
#      which requires broker_public_ingress_ips (App Platform egress) + ideally
#      TLS. Plaintext today.
#   3. registry & kms are routed by PATH PREFIX. Workers (Droplets) reach them at
#      the app's public URL — but path-prefix routing only works if those
#      services are base-path aware, else they need their own subdomains/apps.
#   4. Bucket mapping: web UI/worker use a single S3_BUCKET (var.app_data_bucket).

locals {
  spaces_endpoint = "https://${var.spaces_region}.digitaloceanspaces.com"

  # Scheduler lives on App Platform (outside the VPC) so it can't use the broker's
  # private IP — it dials the public IP. Workers use rabbitmq_amqp_private_uri.
  amqp_public_uri = "amqp://${var.rabbitmq_user}:${random_password.rabbitmq.result}@${digitalocean_droplet.rabbitmq.ipv4_address}:5672/"
}

resource "digitalocean_app" "spade" {
  spec {
    name   = "spade"
    region = var.app_region

    # Custom primary domain (DNS on Cloudflare). Making it PRIMARY sets the
    # ${APP_URL} bindable — and therefore BETTER_AUTH_URL — to https://app.psae.us.
    # Point a CNAME app -> <app>.ondigitalocean.app in Cloudflare. Use DNS-only
    # (grey cloud) so App Platform can validate + issue its cert; you can enable
    # the proxy afterward with SSL mode "Full".
    domain {
      name = "app.psae.us"
      type = "PRIMARY"
    }

    # ---- Managed Postgres binding (injects ${spade-db.DATABASE_URL}, etc.) ----
    database {
      name         = "spade-db"
      engine       = "PG"
      production   = true
      cluster_name = digitalocean_database_cluster.postgres.name
      db_name      = digitalocean_database_db.spade.name
      db_user      = "doadmin"
    }

    # ---- Pre-deploy: run Drizzle migrations before services start ----
    job {
      name               = "migrate"
      kind               = "PRE_DEPLOY"
      instance_size_slug = "basic-xxs"
      instance_count     = 1

      image {
        registry_type = "DOCR"
        repository    = "spade-migrate"
        tag           = var.image_tag
      }

      # drizzle-kit uses node-postgres (pg ^8.20), which now treats sslmode=require
      # as verify-full and rejects DO's CA. Build the DSN with sslmode=no-verify
      # (still TLS, skips CA check) instead of the injected ${spade-db.DATABASE_URL}
      # which carries sslmode=require. The DB is trusted-sources-only, not public.
      env {
        key   = "DATABASE_URL"
        value = "postgresql://${"$"}{spade-db.USERNAME}:${"$"}{spade-db.PASSWORD}@${"$"}{spade-db.HOSTNAME}:${"$"}{spade-db.PORT}/${"$"}{spade-db.DATABASE}?sslmode=no-verify"
        scope = "RUN_TIME"
      }
    }

    # ---- Web UI (Nuxt/Bun) — the one public "/" surface ----
    service {
      name               = "web-ui"
      instance_size_slug = var.web_ui_instance_size
      instance_count     = 1
      http_port          = 3000

      image {
        registry_type = "DOCR"
        repository    = "spade-web-ui"
        tag           = var.image_tag

        deploy_on_push {
          enabled = true
        }
      }

      env {
        key   = "DATABASE_URL"
        value = "${"$"}{spade-db.DATABASE_URL}"
        scope = "RUN_TIME"
      }
      env {
        key   = "BETTER_AUTH_SECRET"
        value = random_password.better_auth_secret.result
        type  = "SECRET"
        scope = "RUN_TIME"
      }
      env {
        key   = "BETTER_AUTH_URL"
        value = "${"$"}{APP_URL}"
        scope = "RUN_TIME"
      }
      env {
        key   = "NUXT_PUBLIC_BETTER_AUTH_URL"
        value = "${"$"}{APP_URL}"
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_ENDPOINT"
        value = local.spaces_endpoint
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_REGION"
        value = var.spaces_region
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_ACCESS_KEY_ID"
        value = var.spaces_access_key_id
        type  = "SECRET"
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_SECRET_ACCESS_KEY"
        value = var.spaces_secret_access_key
        type  = "SECRET"
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_BUCKET"
        value = var.app_data_bucket
        scope = "RUN_TIME"
      }
      env {
        key   = "WORKER_CALLBACK_SECRET"
        value = random_password.worker_callback_secret.result
        type  = "SECRET"
        scope = "RUN_TIME"
      }
    }

    # ---- Scheduler (Go) — internal only (no ingress rule) ----
    service {
      name               = "scheduler"
      instance_size_slug = var.scheduler_instance_size
      instance_count     = 1
      http_port          = 1323

      image {
        registry_type = "DOCR"
        repository    = "spade-scheduler"
        tag           = var.image_tag

        deploy_on_push {
          enabled = true
        }
      }

      env {
        key   = "SPADE_DATABASE_URL"
        value = "${"$"}{spade-db.DATABASE_URL}"
        scope = "RUN_TIME"
      }
      env {
        key   = "SPADE_UI_DB_URL"
        value = "${"$"}{spade-db.DATABASE_URL}"
        scope = "RUN_TIME"
      }
      env {
        key   = "SPADE_AMQP_URL"
        value = local.amqp_public_uri
        type  = "SECRET"
        scope = "RUN_TIME"
      }
      env {
        key   = "SPADE_UI_BASE_URL"
        value = "${"$"}{web-ui.PRIVATE_URL}"
        scope = "RUN_TIME"
      }
      env {
        key   = "SPADE_UI_CALLBACK_SECRET"
        value = random_password.worker_callback_secret.result
        type  = "SECRET"
        scope = "RUN_TIME"
      }
      env {
        key   = "SPADE_HTTP_ADDR"
        value = ":1323"
        scope = "RUN_TIME"
      }
      env {
        key   = "SPADE_LOG_LEVEL"
        value = "info"
        scope = "RUN_TIME"
      }
      env {
        key   = "SCHEDULER_TOKEN_PRIVKEY"
        value = var.scheduler_token_privkey
        type  = "SECRET"
        scope = "RUN_TIME"
      }
      env {
        key   = "SCHEDULER_TOKEN_TTL"
        value = "10m"
        scope = "RUN_TIME"
      }
    }

    # ---- Registry control plane (Go) — public via /registry prefix ----
    service {
      name               = "registry"
      instance_size_slug = var.registry_instance_size
      instance_count     = 1
      http_port          = 8080

      image {
        registry_type = "DOCR"
        repository    = "spade-registry"
        tag           = var.image_tag

        deploy_on_push {
          enabled = true
        }
      }

      env {
        key   = "LISTEN_ADDR"
        value = ":8080"
        scope = "RUN_TIME"
      }
      env {
        key   = "DATABASE_URL"
        value = "${"$"}{spade-db.DATABASE_URL}"
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_ENDPOINT"
        value = local.spaces_endpoint
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_REGION"
        value = var.spaces_region
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_ACCESS_KEY_ID"
        value = var.spaces_access_key_id
        type  = "SECRET"
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_SECRET_ACCESS_KEY"
        value = var.spaces_secret_access_key
        type  = "SECRET"
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_BUCKET"
        value = "spade-artifacts"
        scope = "RUN_TIME"
      }
      env {
        key   = "S3_USE_PATH_STYLE"
        value = "false"
        scope = "RUN_TIME"
      }
      env {
        key   = "REGISTRY_INTERNAL_URL"
        value = "${"$"}{registry.PRIVATE_URL}"
        scope = "RUN_TIME"
      }
      env {
        key   = "REGISTRY_PUBLIC_URL"
        value = "${"$"}{APP_URL}/registry"
        scope = "RUN_TIME"
      }
      env {
        key   = "MIRROR_ENABLED"
        value = "true"
        scope = "RUN_TIME"
      }
      env {
        key   = "ADMIN_USER_IDS"
        value = var.admin_user_ids
        scope = "RUN_TIME"
      }
      # No BUILDER_* / docker.sock here. App Platform has no Docker daemon; the
      # embedded dispatcher is disabled and the build-runner Droplet
      # (buildrunner.tf) claims build jobs off the shared Postgres queue.
      env {
        key   = "BUILD_DISPATCH_ENABLED"
        value = "false"
        scope = "RUN_TIME"
      }
    }

    # ---- KMS (Go) — public via /kms prefix (workers resolve secrets here) ----
    service {
      name               = "kms"
      instance_size_slug = var.kms_instance_size
      instance_count     = 1
      http_port          = 8081

      image {
        registry_type = "DOCR"
        repository    = "spade-kms"
        tag           = var.image_tag

        deploy_on_push {
          enabled = true
        }
      }

      env {
        key   = "KMS_ADDR"
        value = ":8081"
        scope = "RUN_TIME"
      }
      env {
        key   = "KMS_DATABASE_DSN"
        value = "${"$"}{spade-db.DATABASE_URL}"
        scope = "RUN_TIME"
      }
      env {
        key   = "KMS_KEKS"
        value = "v1:${random_id.kek.b64_std}"
        type  = "SECRET"
        scope = "RUN_TIME"
      }
      env {
        key   = "KMS_ACTIVE_KEK"
        value = "v1"
        scope = "RUN_TIME"
      }
      env {
        key   = "KMS_TOKEN_PUBKEYS"
        value = var.kms_token_pubkeys
        type  = "SECRET"
        scope = "RUN_TIME"
      }
    }

    # ---- Public routing. Components not listed here are internal-only. ----
    ingress {
      rule {
        component { name = "web-ui" }
        match {
          path { prefix = "/" }
        }
      }
      rule {
        component { name = "registry" }
        match {
          path { prefix = "/registry" }
        }
      }
      rule {
        component { name = "kms" }
        match {
          path { prefix = "/kms" }
        }
      }
    }
  }
}
