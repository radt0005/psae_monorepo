# Block: helloWorld
#
# A minimal, dependency-free R block used to verify that R blocks execute
# correctly inside the spade sandbox. It deliberately uses only base R (no
# `library(spade)`, no `yaml`) so it runs against any R install — system- or
# user-managed — without first installing extra packages. Its job is to prove
# the sandbox can load R's shared libraries and that the block can write an
# output the executor will collect.

# Minimal base-R parser for the `key: value` params.yaml spade writes. We avoid
# the yaml package so the block has zero external dependencies.
read_params <- function(path = "params.yaml") {
  if (!file.exists(path)) {
    return(list())
  }
  params <- list()
  for (line in readLines(path, warn = FALSE)) {
    line <- trimws(line)
    if (line == "" || startsWith(line, "#")) {
      next
    }
    kv <- regmatches(line, regexec("^([^:]+):[[:space:]]*(.*)$", line))[[1]]
    if (length(kv) == 3) {
      val <- trimws(kv[3])
      val <- sub('^"(.*)"$', "\\1", val) # strip surrounding quotes, if any
      params[[trimws(kv[2])]] <- val
    }
  }
  params
}

params <- read_params()
name <- if (!is.null(params$name) && nzchar(params$name)) params$name else "World"

greeting <- sprintf(
  "Hello, %s! R %s ran successfully inside the spade sandbox.\n",
  name, getRversion()
)
cat(greeting)

# Write the greeting as the block's `greeting` output. The executor collects
# every entry under outputs/, so each output gets its own subdirectory.
out_dir <- file.path("outputs", "greeting")
dir.create(out_dir, recursive = TRUE, showWarnings = FALSE)
writeLines(greeting, file.path(out_dir, "greeting.txt"))
