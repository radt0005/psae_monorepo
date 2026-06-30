# setup.R — build step for the `sae` R collection.
#
# `spade install` runs this with `Rscript setup.R` from the collection root to
# make `library(spade)` resolvable at block runtime. The worker's isolate
# sandbox binds the user library (R_LIBS_USER) and puts it on R's search path,
# so installing into that library here is sufficient — see core/executor.go,
# languageSandboxBinds.

# Install into the per-user library so the sandbox (which binds R_LIBS_USER)
# can find the packages. Fall back to the first writable .libPaths() entry.
user_lib <- Sys.getenv("R_LIBS_USER")
if (!nzchar(user_lib) || user_lib == "NULL") {
  user_lib <- .libPaths()[1]
}
dir.create(user_lib, recursive = TRUE, showWarnings = FALSE)
.libPaths(c(user_lib, .libPaths()))

# yaml is a hard dependency of the spade package.
if (!requireNamespace("yaml", quietly = TRUE)) {
  install.packages("yaml", lib = user_lib, repos = "https://cloud.r-project.org")
}

# Install the local spade authoring library. Default to the monorepo layout
# (libs/R is two levels up from blocks/sae), overridable via SPADE_R_LIB_SRC.
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

if (!requireNamespace("spade", quietly = TRUE)) {
  stop("spade package failed to install or load after setup")
}
cat("setup.R: spade library installed into", user_lib, "\n")
