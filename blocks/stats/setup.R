# setup.R — build step for the `stats` R collection.
#
# `spade install` runs this with `Rscript setup.R` from the collection root.
# Its job is to make `library(spade)` (and the packages the handlers use)
# resolvable at block runtime. The worker's isolate sandbox binds the user
# library (R_LIBS_USER) and puts it on R's search path, so installing into
# that library here is sufficient — see core/executor.go, languageSandboxBinds.
#
# Without this step `library(spade)` fails: nothing in the install flow ships
# the spade R package, so it is never on any library path inside the sandbox.

# Install into the per-user library so the sandbox (which binds R_LIBS_USER)
# can find the packages. Fall back to the first writable .libPaths() entry.
user_lib <- Sys.getenv("R_LIBS_USER")
if (!nzchar(user_lib) || user_lib == "NULL") {
  user_lib <- .libPaths()[1]
}
dir.create(user_lib, recursive = TRUE, showWarnings = FALSE)
.libPaths(c(user_lib, .libPaths()))

# Runtime packages the handlers depend on: jsonlite for JSON output, and yaml
# (a hard dependency of the spade package). Install only what is missing so
# repeat installs are fast.
ensure <- function(pkgs) {
  missing <- pkgs[!vapply(pkgs, requireNamespace, logical(1), quietly = TRUE)]
  if (length(missing) > 0) {
    install.packages(missing, lib = user_lib, repos = "https://cloud.r-project.org")
  }
}
ensure(c("jsonlite", "yaml"))

# Install the local spade authoring library. Default to the monorepo layout
# (libs/R is two levels up from blocks/stats), overridable via SPADE_R_LIB_SRC
# for installs from a different checkout location.
spade_src <- Sys.getenv("SPADE_R_LIB_SRC")
if (!nzchar(spade_src)) {
  spade_src <- normalizePath(file.path("..", "..", "libs", "R"), mustWork = FALSE)
}
if (!dir.exists(spade_src)) {
  stop(sprintf(
    "spade R library source not found at '%s'; set SPADE_R_LIB_SRC to its path",
    spade_src
  ))
}
install.packages(spade_src, lib = user_lib, repos = NULL, type = "source")

# Fail loudly if the package still cannot be loaded, so a broken install
# surfaces here rather than at block-execution time inside the sandbox.
if (!requireNamespace("spade", quietly = TRUE)) {
  stop("spade package failed to install or load after setup")
}
cat("setup.R: spade library installed into", user_lib, "\n")
