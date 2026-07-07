#' Run a Spade block handler
#'
#' Main entry point for executing a Spade block. Loads parameters from
#' params.yaml, scans the inputs/ directory for file inputs, calls the
#' handler function with the appropriate arguments, and writes outputs.
#'
#' @param fn A handler function defining the block's processing logic.
#' @return Invisible NULL.
#' @export
run <- function(fn) {
  # 0. Scrub SPADE_SECRETS from the environment early (before the handler runs),
  #    even if the block never calls get_secret. Idempotent.
  .load_secrets()

  # 1. Build complete arguments from params.yaml + inputs/ directory
  args <- build_function_args(fn)

  # 2. Get handler's declared parameter names
  param_names <- names(formals(fn))

  # 3. Check for ... (equivalent to Python's **kwargs)
  has_dots <- "..." %in% param_names

  # 4. Filter arguments to only those the handler declares
  if (has_dots) {
    filtered_args <- args
  } else {
    filtered_args <- args[names(args) %in% param_names]
  }

  # 5. Call the handler
  result <- do.call(fn, filtered_args)

  # 6. Read manifest and write outputs
  manifest_outputs <- read_block_manifest()
  write_outputs(result, manifest_outputs)

  invisible(NULL)
}
