# Access to secrets injected into a block by the Spade runtime.
#
# Secrets are delivered as the SPADE_SECRETS environment variable — a JSON
# object mapping the block's logical secret names to their values — by the
# worker (cloud) or CLI (local). This file parses that blob, serves values
# through get_secret(), and scrubs the variable from the environment so it is
# not inherited by any subprocess the block spawns. See spec/secrets.md §4.

# Package-local cache for the parsed secrets.
.spade_secrets <- new.env(parent = emptyenv())

# Parse and cache SPADE_SECRETS, removing it from the environment on first read.
# Idempotent. JSON is valid YAML, so it is parsed with the yaml dependency
# rather than adding jsonlite.
.load_secrets <- function() {
  if (is.null(.spade_secrets$cache)) {
    raw <- Sys.getenv("SPADE_SECRETS", unset = "")
    Sys.unsetenv("SPADE_SECRETS")
    if (nzchar(raw)) {
      .spade_secrets$cache <- yaml.load(raw)
    } else {
      .spade_secrets$cache <- list()
    }
  }
  .spade_secrets$cache
}

#' Get a secret provided to this block
#'
#' Returns the secret bound to a logical name. The mapping from logical name to
#' a stored secret is declared in the pipeline (see spec/secrets.md); the value
#' is injected by the worker (cloud) or CLI (local). Errors if the name was not
#' provided — a declared-but-unresolved secret is a real error, not empty.
#'
#' @param name Logical secret name (the same name used in the block).
#' @return The secret value as a character string.
#' @export
get_secret <- function(name) {
  secrets <- .load_secrets()
  if (!name %in% names(secrets) || is.null(secrets[[name]])) {
    stop(sprintf(
      "secret '%s' was not provided to this block; declare it in the pipeline's secrets mapping",
      name
    ))
  }
  as.character(secrets[[name]])
}
