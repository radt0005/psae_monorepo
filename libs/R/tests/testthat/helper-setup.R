# Create a mock Spade working directory with inputs/, outputs/, logs/
setup_work_dir <- function() {
  work_dir <- tempfile("spade_test_")
  dir.create(work_dir)
  dir.create(file.path(work_dir, "inputs"))
  dir.create(file.path(work_dir, "outputs"))
  dir.create(file.path(work_dir, "logs"))
  old_wd <- setwd(work_dir)
  list(path = work_dir, old_wd = old_wd)
}

# Restore original working directory
teardown_work_dir <- function(env) {
  setwd(env$old_wd)
  unlink(env$path, recursive = TRUE)
}

# Write params.yaml content
write_params <- function(params) {
  yaml::write_yaml(params, "params.yaml")
}

# Create a single file in an input subdirectory
create_input_file <- function(name, filename = "data.tif", content = charToRaw("test data")) {
  input_dir <- file.path("inputs", name)
  dir.create(input_dir, showWarnings = FALSE)
  file_path <- file.path(input_dir, filename)
  writeBin(content, file_path)
  file_path
}

# Create multiple files in an input subdirectory
create_input_collection <- function(name, filenames, content = charToRaw("test data")) {
  input_dir <- file.path("inputs", name)
  dir.create(input_dir, showWarnings = FALSE)
  paths <- character(length(filenames))
  for (i in seq_along(filenames)) {
    file_path <- file.path(input_dir, filenames[i])
    writeBin(content, file_path)
    paths[i] <- file_path
  }
  paths
}
