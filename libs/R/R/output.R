# Type-to-default-name mapping
.type_to_default_name <- list(
  File = "file",
  RasterFile = "raster",
  VectorFile = "vector",
  TabularFile = "tabular",
  JsonFile = "json",
  Directory = "directory",
  FileCollection = "files",
  RasterFileCollection = "rasters",
  VectorFileCollection = "vectors",
  TabularFileCollection = "tables"
)

#' Read block manifest for output declarations
#'
#' Checks SPADE_BLOCK_MANIFEST env var then block.yaml in CWD.
#' Returns only the outputs section, or NULL.
#'
#' @return A list of output declarations, or NULL.
#' @keywords internal
read_block_manifest <- function() {
  # Check environment variable first
  manifest_path <- Sys.getenv("SPADE_BLOCK_MANIFEST", "")
  if (nzchar(manifest_path) && file.exists(manifest_path)) {
    manifest <- yaml::read_yaml(manifest_path)
    return(manifest[["outputs"]])
  }

  # Check block.yaml in current working directory
  block_yaml <- "block.yaml"
  if (file.exists(block_yaml)) {
    manifest <- yaml::read_yaml(block_yaml)
    return(manifest[["outputs"]])
  }

  NULL
}

#' Infer output name from a return value's class
#'
#' @param value An S4 object (File, Directory, or FileCollection subclass).
#' @return A character string naming the output directory.
#' @keywords internal
infer_output_name <- function(value) {
  cls <- class(value)
  name <- .type_to_default_name[[cls]]
  if (!is.null(name)) return(name)
  tolower(cls)
}

#' Write a single output value to outputs/<name>/
#'
#' @param name The output directory name.
#' @param value An S4 object (File, Directory, or FileCollection subclass).
#' @keywords internal
write_single_output <- function(name, value) {
  output_dir <- file.path("outputs", name)
  dir.create(output_dir, recursive = TRUE, showWarnings = FALSE)

  if (is(value, "FileCollection")) {
    for (file_path in value@paths) {
      file.copy(file_path, file.path(output_dir, basename(file_path)))
    }
  } else if (is(value, "Directory")) {
    items <- list.files(value@path, full.names = TRUE, no.. = TRUE)
    for (item in items) {
      if (file.info(item)$isdir) {
        dir_dest <- file.path(output_dir, basename(item))
        dir.create(dir_dest, recursive = TRUE, showWarnings = FALSE)
        file.copy(item, output_dir, recursive = TRUE)
      } else {
        file.copy(item, file.path(output_dir, basename(item)))
      }
    }
  } else if (is(value, "File")) {
    file.copy(value@path, file.path(output_dir, basename(value@path)))
  }
}

#' Write handler outputs to the outputs/ directory
#'
#' Handles all return value patterns: NULL, single S4 object, or named list.
#'
#' @param result The handler's return value.
#' @param manifest_outputs The outputs section from block manifest, or NULL.
#' @keywords internal
write_outputs <- function(result, manifest_outputs = NULL) {
  if (is.null(result)) return(invisible(NULL))

  dir.create("outputs", showWarnings = FALSE)

  if (is.list(result) && !isS4(result) && !is.null(names(result))) {
    # Dict-like return: each key-value pair is a named output
    for (name in names(result)) {
      write_single_output(name, result[[name]])
    }
  } else if (is(result, "File") || is(result, "Directory") || is(result, "FileCollection")) {
    # Single output
    if (!is.null(manifest_outputs) && length(manifest_outputs) == 1) {
      name <- names(manifest_outputs)[1]
    } else {
      name <- infer_output_name(result)
    }
    write_single_output(name, result)
  }
}
