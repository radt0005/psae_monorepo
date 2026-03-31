# Type-to-manifest mapping
.r_type_to_manifest <- list(
  File = list(type = "file"),
  RasterFile = list(type = "file", format = "GeoTIFF"),
  VectorFile = list(type = "file", format = "GeoJSON"),
  TabularFile = list(type = "file", format = "CSV"),
  JsonFile = list(type = "json"),
  Directory = list(type = "directory"),
  FileCollection = list(type = "collection", item_type = "file"),
  RasterFileCollection = list(type = "collection", item_type = "file", format = "GeoTIFF"),
  VectorFileCollection = list(type = "collection", item_type = "file", format = "GeoJSON"),
  TabularFileCollection = list(type = "collection", item_type = "file", format = "CSV"),
  character = list(type = "string"),
  integer = list(type = "number"),
  numeric = list(type = "number"),
  logical = list(type = "boolean")
)

# Type-to-output-name mapping
.type_to_output_name <- list(
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

#' Build a block manifest from a handler function
#'
#' Generates a block manifest list from a handler function's type annotations.
#' The manifest includes input declarations, output declarations, and an
#' optional description.
#'
#' @param fn A handler function with spade type annotations.
#' @return A list representing the block manifest.
#' @export
build <- function(fn) {
  hints <- spade_types(fn)
  if (is.null(hints)) hints <- list()

  param_names <- names(formals(fn))

  # Build inputs
  inputs <- list()
  for (pname in param_names) {
    ptype <- hints[[pname]]
    if (is.null(ptype)) next
    manifest_entry <- .r_type_to_manifest[[ptype]]
    if (!is.null(manifest_entry)) {
      inputs[[pname]] <- manifest_entry
    }
  }

  # Build outputs from .return type hint
  outputs <- list()
  return_type <- hints[[".return"]]
  if (!is.null(return_type)) {
    manifest_entry <- .r_type_to_manifest[[return_type]]
    if (!is.null(manifest_entry)) {
      output_name <- .type_to_output_name[[return_type]]
      if (is.null(output_name)) output_name <- "output"
      outputs[[output_name]] <- manifest_entry
    }
  }

  # Build manifest
  manifest <- list()

  # Extract description from attribute
  doc <- attr(fn, "spade_description")
  if (!is.null(doc)) {
    manifest[["description"]] <- doc
  }

  manifest[["inputs"]] <- inputs
  manifest[["outputs"]] <- outputs

  manifest
}
