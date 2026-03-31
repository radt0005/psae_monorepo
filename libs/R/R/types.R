#' @include zzz.R

# ---- Base Classes ----

#' File type
#'
#' Represents a single file input or output with a path.
#'
#' @slot path Character string giving the file path.
#' @export
setClass("File", slots = list(path = "character"))

#' Create a File object
#'
#' @param path Character string giving the file path.
#' @return A File object.
#' @export
File <- function(path) new("File", path = path)

#' Directory type
#'
#' Represents a directory input or output with a path.
#'
#' @slot path Character string giving the directory path.
#' @export
setClass("Directory", slots = list(path = "character"))

#' Create a Directory object
#'
#' @param path Character string giving the directory path.
#' @return A Directory object.
#' @export
Directory <- function(path) new("Directory", path = path)

# ---- File Subtypes ----

#' RasterFile type
#'
#' Represents a raster data file (e.g. GeoTIFF). Inherits from File.
#'
#' @export
setClass("RasterFile", contains = "File")

#' Create a RasterFile object
#'
#' @param path Character string giving the file path.
#' @return A RasterFile object.
#' @export
RasterFile <- function(path) new("RasterFile", path = path)

#' VectorFile type
#'
#' Represents a vector data file (e.g. GeoJSON). Inherits from File.
#'
#' @export
setClass("VectorFile", contains = "File")

#' Create a VectorFile object
#'
#' @param path Character string giving the file path.
#' @return A VectorFile object.
#' @export
VectorFile <- function(path) new("VectorFile", path = path)

#' TabularFile type
#'
#' Represents a tabular data file (e.g. CSV). Inherits from File.
#'
#' @export
setClass("TabularFile", contains = "File")

#' Create a TabularFile object
#'
#' @param path Character string giving the file path.
#' @return A TabularFile object.
#' @export
TabularFile <- function(path) new("TabularFile", path = path)

#' JsonFile type
#'
#' Represents a JSON file. Inherits from File.
#'
#' @export
setClass("JsonFile", contains = "File")

#' Create a JsonFile object
#'
#' @param path Character string giving the file path.
#' @return A JsonFile object.
#' @export
JsonFile <- function(path) new("JsonFile", path = path)

# ---- Collection Types ----

#' FileCollection type
#'
#' Represents a collection of files with a character vector of paths.
#'
#' @slot paths Character vector of file paths.
#' @export
setClass("FileCollection", slots = list(paths = "character"))

#' Create a FileCollection object
#'
#' @param paths Character vector of file paths.
#' @return A FileCollection object.
#' @export
FileCollection <- function(paths) new("FileCollection", paths = paths)

#' RasterFileCollection type
#'
#' Represents a collection of raster files. Inherits from FileCollection.
#'
#' @export
setClass("RasterFileCollection", contains = "FileCollection")

#' Create a RasterFileCollection object
#'
#' @param paths Character vector of file paths.
#' @return A RasterFileCollection object.
#' @export
RasterFileCollection <- function(paths) new("RasterFileCollection", paths = paths)

#' VectorFileCollection type
#'
#' Represents a collection of vector files. Inherits from FileCollection.
#'
#' @export
setClass("VectorFileCollection", contains = "FileCollection")

#' Create a VectorFileCollection object
#'
#' @param paths Character vector of file paths.
#' @return A VectorFileCollection object.
#' @export
VectorFileCollection <- function(paths) new("VectorFileCollection", paths = paths)

#' TabularFileCollection type
#'
#' Represents a collection of tabular files. Inherits from FileCollection.
#'
#' @export
setClass("TabularFileCollection", contains = "FileCollection")

#' Create a TabularFileCollection object
#'
#' @param paths Character vector of file paths.
#' @return A TabularFileCollection object.
#' @export
TabularFileCollection <- function(paths) new("TabularFileCollection", paths = paths)

# ---- Validity Methods ----

setValidity("File", function(object) {
  if (length(object@path) != 1 || !nzchar(object@path)) {
    "path must be a non-empty single character string"
  } else {
    TRUE
  }
})

setValidity("Directory", function(object) {
  if (length(object@path) != 1 || !nzchar(object@path)) {
    "path must be a non-empty single character string"
  } else {
    TRUE
  }
})

setValidity("FileCollection", function(object) {
  if (!is.character(object@paths)) {
    "paths must be a character vector"
  } else {
    TRUE
  }
})

# ---- Show Methods ----

#' @export
setMethod("show", "File", function(object) {
  cat(sprintf("<%s> %s\n", class(object), object@path))
})

#' @export
setMethod("show", "Directory", function(object) {
  cat(sprintf("<%s> %s\n", class(object), object@path))
})

#' @export
setMethod("show", "FileCollection", function(object) {
  cat(sprintf("<%s> [%d files]\n", class(object), length(object@paths)))
})
