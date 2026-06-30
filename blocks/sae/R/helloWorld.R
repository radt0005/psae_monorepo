# Block: helloWorld
#
# Minimal R block that verifies the spade R runtime library works end-to-end
# inside the sandbox: `library(spade)` loads, scalar params are passed from
# params.yaml, and a returned File is collected into outputs/. The collection's
# setup.R installs the spade package so `library(spade)` resolves at runtime.
library(spade)

handler <- function(name = "World") {
  greeting <- sprintf(
    "Hello, %s! R %s ran successfully inside the spade sandbox.\n",
    name, getRversion()
  )
  cat(greeting)

  out_path <- "greeting.txt"
  writeLines(greeting, out_path)
  File(out_path)
}
spade_types(handler) <- list(name = "character", .return = "File")
attr(handler, "spade_description") <- "Minimal block that writes a greeting; verifies R execution."

run(handler)
